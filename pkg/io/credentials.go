/*
Copyright The Kubernetes Authors.

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

package io

import (
	"encoding/json"
	"fmt"
	"os"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"google.golang.org/api/option"
)

// supportedCredentialTypes maps the "type" discriminator of a Google
// credentials file to the matching credentials.CredType. A file is loaded with
// the typed loader for its own type rather than with the deprecated
// DetectOptions.CredentialsFile field, which does not validate the type at all.
var supportedCredentialTypes = map[string]credentials.CredType{
	string(credentials.ServiceAccount):                credentials.ServiceAccount,
	string(credentials.AuthorizedUser):                credentials.AuthorizedUser,
	string(credentials.ExternalAccount):               credentials.ExternalAccount,
	string(credentials.ImpersonatedServiceAccount):    credentials.ImpersonatedServiceAccount,
	string(credentials.GDCHServiceAccount):            credentials.GDCHServiceAccount,
	string(credentials.ExternalAccountAuthorizedUser): credentials.ExternalAccountAuthorizedUser,
}

// GoogleCredentialsFileOption returns a client option that authenticates with
// the credentials in file, which may be of any type supported by Google's auth
// library (service_account, authorized_user, external_account, ...). Scopes
// must be given explicitly because the returned credentials bypass the
// client's default scopes.
func GoogleCredentialsFileOption(file string, scopes ...string) (option.ClientOption, error) {
	creds, err := loadGoogleCredentials(file, scopes)
	if err != nil {
		return nil, fmt.Errorf("load Google credentials from %q: %w", file, err)
	}
	return option.WithAuthCredentials(creds), nil
}

// loadGoogleCredentials reads the type discriminator from file and loads it
// with the credential-type-specific loader for that type. Only the type is
// looked at here; validating the rest of the file is left to the auth library.
func loadGoogleCredentials(file string, scopes []string) (*auth.Credentials, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var discriminator struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(content, &discriminator); err != nil {
		return nil, err
	}
	credType, ok := supportedCredentialTypes[discriminator.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported credentials type %q", discriminator.Type)
	}
	return credentials.NewCredentialsFromFile(credType, file, &credentials.DetectOptions{Scopes: scopes})
}
