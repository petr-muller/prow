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
	"flag"
	"fmt"
	"os"

	"sigs.k8s.io/prow/pkg/config/secret"
	gitv2 "sigs.k8s.io/prow/pkg/git/v2"
)

// GitOptions holds options for interacting with git repositories.
type GitOptions struct {
	SigningKeyPath string
}

// AddFlags injects git options into the given FlagSet.
func (o *GitOptions) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&o.SigningKeyPath, "git-signing-key-path", "", "Path to an SSH private key for signing git commits. When set, all commits made by the git client are signed using SSH.")
}

// Validate checks that the signing key is a readable regular file without reading its contents.
func (o *GitOptions) Validate(bool) error {
	if o.SigningKeyPath != "" {
		info, err := os.Stat(o.SigningKeyPath)
		if err != nil {
			return fmt.Errorf("invalid --git-signing-key-path %q: %w", o.SigningKeyPath, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("invalid --git-signing-key-path %q: must be a regular file", o.SigningKeyPath)
		}
		key, err := os.Open(o.SigningKeyPath)
		if err != nil {
			return fmt.Errorf("cannot open --git-signing-key-path %q for reading: %w", o.SigningKeyPath, err)
		}
		if err := key.Close(); err != nil {
			return fmt.Errorf("cannot close --git-signing-key-path %q: %w", o.SigningKeyPath, err)
		}
	}

	return nil
}

// GitClientFactory returns git.ClientFactory. Passing non-empty cookieFilePath
// will result in git ClientFactory to work with Gerrit.
// GitHub options may be nil for Gerrit or anonymous access.
func (o *GitOptions) GitClientFactory(githubOptions *GitHubOptions, cookieFilePath string, cacheDir *string, dryRun, persistCache bool) (gitv2.ClientFactory, error) {
	opts := gitv2.ClientFactoryOpts{
		Censor:         secret.Censor,
		CookieFilePath: cookieFilePath,
		Persist:        &persistCache,
		SigningKeyPath: o.SigningKeyPath,
	}
	if githubOptions != nil {
		opts.Host = githubOptions.Host
	}
	if cacheDir != nil && *cacheDir != "" {
		opts.CacheDirBase = cacheDir
	}

	if cookieFilePath == "" && githubOptions != nil && (githubOptions.TokenPath != "" || githubOptions.AppPrivateKeyPath != "") {
		// Make a client with auth suitable for GitHub
		user, generator, err := githubOptions.getGitHubAuthentication(dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to get git authentication: %w", err)
		}
		opts.Username = func() (string, error) { return user, nil }
		opts.Token = generator
	}
	// If the client is for Gerrit we're already set with the cookie filepath.

	gitClientFactory, err := gitv2.NewClientFactory(opts.Apply)
	if err != nil {
		return nil, fmt.Errorf("failed to create git client factory: %w", err)
	}
	return gitClientFactory, nil
}
