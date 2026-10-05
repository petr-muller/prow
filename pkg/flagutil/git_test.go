/*
Copyright 2018 The Kubernetes Authors.

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

package flagutil

import (
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitOptions_ValidateSigningKeyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing-key")
	if err := os.WriteFile(keyPath, []byte("key contents must not be read or logged"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "empty path disables signing"},
		{name: "readable file", path: keyPath},
		{name: "missing file", path: filepath.Join(dir, "missing"), wantErr: true},
		{name: "directory", path: dir, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := &GitOptions{SigningKeyPath: tc.path}
			err := opts.Validate(false)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, want error: %t", err, tc.wantErr)
			}
			if err != nil && (!strings.Contains(err.Error(), "--git-signing-key-path") || !strings.Contains(err.Error(), tc.path)) {
				t.Errorf("error must identify the flag and configured path: %v", err)
			}
			if opts.SigningKeyPath != tc.path {
				t.Errorf("SigningKeyPath = %q, want %q", opts.SigningKeyPath, tc.path)
			}
		})
	}
}

func TestGitOptionsSigningFlag(t *testing.T) {
	fs := flag.NewFlagSet("git", flag.ContinueOnError)
	var opts GitOptions
	opts.AddFlags(fs)
	if err := fs.Parse([]string{"--git-signing-key-path=/path/to/key"}); err != nil {
		t.Fatal(err)
	}
	if opts.SigningKeyPath != "/path/to/key" {
		t.Fatalf("SigningKeyPath = %q", opts.SigningKeyPath)
	}

	var githubOpts GitHubOptions
	githubFlags := flag.NewFlagSet("combined", flag.ContinueOnError)
	githubOpts.AddFlags(githubFlags)
	opts.AddFlags(githubFlags)
}

func TestGitOptionsFactory(t *testing.T) {
	// Gerrit configures cookies globally; isolate the config from other tests and the user.
	globalConfig := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	for _, tc := range []struct {
		name       string
		github     *GitHubOptions
		cookieFile string
		persist    bool
		dryRun     bool
	}{
		{name: "anonymous"},
		{name: "persistent cache", persist: true},
		{name: "dry run", dryRun: true},
		{name: "Gerrit without GitHub", cookieFile: "/path/to/cookies"},
		{
			name:       "Gerrit cookies take precedence over GitHub auth",
			cookieFile: "/path/to/cookies",
			github: &GitHubOptions{
				TokenPath: "/missing/github/token",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			opts := GitOptions{SigningKeyPath: "/path/to/signing-key"}
			factory, err := opts.GitClientFactory(tc.github, tc.cookieFile, &cacheDir, tc.dryRun, tc.persist)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := factory.Clean(); err != nil {
					t.Error(err)
				}
			})
			entries, err := os.ReadDir(cacheDir)
			if err != nil {
				t.Fatal(err)
			}
			if tc.persist && len(entries) != 0 {
				t.Fatalf("persistent cache created a temporary directory: %v", entries)
			}
			if !tc.persist && (len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "gitcache")) {
				t.Fatalf("expected temporary cache under configured base: %v", entries)
			}
			if tc.cookieFile != "" {
				output, err := exec.Command("git", "config", "--global", "http.cookiefile").CombinedOutput()
				if err != nil || strings.TrimSpace(string(output)) != tc.cookieFile {
					t.Fatalf("cookie configuration = %q, %v", output, err)
				}
			}

			repoDir := t.TempDir()
			if output, err := exec.Command("git", "init", repoDir).CombinedOutput(); err != nil {
				t.Fatalf("git init: %v: %s", err, output)
			}
			if _, err := factory.ClientFromDir("org", "repo", repoDir); err != nil {
				t.Fatal(err)
			}
			for key, want := range map[string]string{
				"gpg.format":      "ssh",
				"user.signingkey": opts.SigningKeyPath,
				"commit.gpgsign":  "true",
			} {
				output, err := exec.Command("git", "-C", repoDir, "config", "--local", "--get", key).CombinedOutput()
				if err != nil || strings.TrimSpace(string(output)) != want {
					t.Errorf("%s = %q, %v; want %q", key, output, err, want)
				}
			}
		})
	}
}

func TestGitOptionsFactoryGitHubAuthentication(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		called := false
		githubOpts := GitHubOptions{
			TokenPath: "/already/loaded/token",
			userGenerator: func() (string, error) {
				called = true
				return "", errors.New("authentication failed")
			},
		}
		_, err := (&GitOptions{}).GitClientFactory(&githubOpts, "", nil, dryRun, false)
		if !called || err == nil || !strings.Contains(err.Error(), "failed to get git authentication") {
			t.Fatalf("dryRun=%t: authentication called=%t, error=%v", dryRun, called, err)
		}
	}
}
