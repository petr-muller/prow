/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestClientFromDirSigningKey(t *testing.T) {
	t.Parallel()

	existingConfig := map[string]string{
		"gpg.format":      "openpgp",
		"user.signingkey": "existing-key",
		"commit.gpgsign":  "false",
	}
	for _, tc := range []struct {
		name           string
		signingKeyPath string
		initialConfig  map[string]string
		wantConfig     map[string]string
	}{
		{
			name:           "configured signing key",
			signingKeyPath: "/path/to/signing key",
			initialConfig:  existingConfig,
			wantConfig: map[string]string{
				"gpg.format":      "ssh",
				"user.signingkey": "/path/to/signing key",
				"commit.gpgsign":  "true",
			},
		},
		{
			name: "unconfigured signing key",
		},
		{
			name:          "unconfigured signing key preserves existing config",
			initialConfig: existingConfig,
			wantConfig:    existingConfig,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := t.TempDir()
			runGit(t, repoDir, "init")
			for key, value := range tc.initialConfig {
				runGit(t, repoDir, "config", key, value)
			}
			factory, err := NewClientFactory(WithCacheDirBase(t.TempDir()), WithSigningKeyPath(tc.signingKeyPath))
			if err != nil {
				t.Fatalf("creating factory: %v", err)
			}
			defer factory.Clean()

			client, err := factory.ClientFromDir("org", "repo", repoDir)
			if err != nil {
				t.Fatalf("getting client: %v", err)
			}
			for _, key := range []string{"gpg.format", "user.signingkey", "commit.gpgsign"} {
				out, err := exec.Command("git", "-C", client.Directory(), "config", "--local", "--get", key).CombinedOutput()
				want, configured := tc.wantConfig[key]
				if !configured {
					var exitErr *exec.ExitError
					if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 || len(out) != 0 {
						t.Errorf("expected %s to remain unset, got %q, error: %v", key, out, err)
					}
					continue
				}
				if err != nil || strings.TrimSpace(string(out)) != want {
					t.Errorf("expected %s=%q, got %q, error: %v", key, want, out, err)
				}
			}
		})
	}
}

func TestClientFromDirSigningConfigError(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	if err := os.WriteFile(filepath.Join(repoDir, ".git", "config.lock"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	factory, err := NewClientFactory(WithCacheDirBase(t.TempDir()), WithSigningKeyPath("/path/to/key"))
	if err != nil {
		t.Fatalf("creating factory: %v", err)
	}
	defer factory.Clean()

	client, err := factory.ClientFromDir("org", "repo", repoDir)
	if client != nil || err == nil {
		t.Fatalf("expected no client and a config error, got client %v, error: %v", client, err)
	}
	for _, context := range []string{"failed to configure commit signing", "gpg.format", ".git/config"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("expected error to contain %q, got: %v", context, err)
		}
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("expected wrapped Git exit error, got: %v", err)
	}
}

type signingConfigErrorClient struct {
	RepoClient
	failingKey string
	err        error
}

func (c signingConfigErrorClient) Config(args ...string) error {
	if args[0] == c.failingKey {
		return c.err
	}
	return nil
}

func TestConfigureCommitSigningError(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"gpg.format", "user.signingkey", "commit.gpgsign"} {
		t.Run(key, func(t *testing.T) {
			configErr := errors.New("config failed")
			factory := &clientFactory{signingKeyPath: "/path/to/key"}
			err := factory.configureCommitSigning(signingConfigErrorClient{failingKey: key, err: configErr})
			want := "failed to configure commit signing (" + key + "): config failed"
			if err == nil || err.Error() != want {
				t.Errorf("expected error %q, got: %v", want, err)
			}
			if !errors.Is(err, configErr) {
				t.Errorf("expected wrapped config error, got: %v", err)
			}
		})
	}
}

func TestClientFactorySigningKey(t *testing.T) {
	t.Parallel()

	// Generate an SSH signing key.
	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "id_ed25519")
	if out, err := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "").CombinedOutput(); err != nil {
		t.Fatalf("generating SSH key: %v\n%s", err, out)
	}

	// Create a source repo with a commit.
	repoBase := t.TempDir()
	repoDir := filepath.Join(repoBase, "org", "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@test.test")
	runGit(t, repoDir, "config", "user.name", "test")
	runGit(t, repoDir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", "file.txt")
	runGit(t, repoDir, "commit", "-m", "initial")

	// Create a patch to apply.
	patchFile := filepath.Join(keyDir, "test.patch")
	patchContent := `From 0000000000000000000000000000000000000000 Mon Sep 17 00:00:00 2001
From: Test User <test@test.test>
Date: Thu, 01 Jan 2026 00:00:00 +0000
Subject: [PATCH] update file

---
 file.txt | 2 +-
 1 file changed, 1 insertion(+), 1 deletion(-)

diff --git a/file.txt b/file.txt
index ce01362..94954ab 100644
--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-hello
+world
--
2.40.0
`
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create factory with signing key.
	factory, err := NewLocalClientFactory(repoBase,
		func() (string, string, error) { return "test", "test@test.test", nil },
		func(in []byte) []byte { return in },
		WithSigningKeyPath(keyPath),
	)
	if err != nil {
		t.Fatalf("creating factory: %v", err)
	}
	defer factory.Clean()

	client, err := factory.ClientFor("org", "repo")
	if err != nil {
		t.Fatalf("getting client: %v", err)
	}
	defer client.Clean()

	if err := client.Config("user.name", "test"); err != nil {
		t.Fatalf("setting user.name: %v", err)
	}
	if err := client.Config("user.email", "test@test.test"); err != nil {
		t.Fatalf("setting user.email: %v", err)
	}

	// Apply patch — should produce a signed commit.
	if err := client.Am(patchFile); err != nil {
		t.Fatalf("git am: %v", err)
	}

	// Verify the commit is signed.
	out, err := exec.Command("git", "-C", client.Directory(), "cat-file", "-p", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("inspecting commit: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "BEGIN SSH SIGNATURE") {
		t.Errorf("expected commit to contain SSH signature, got:\n%s", out)
	}
}
