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

package issuemanagement

import (
	"context"
	"fmt"
	"testing"

	githubql "github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/prow/pkg/github"
	"sigs.k8s.io/prow/pkg/github/fakegithub"
	"sigs.k8s.io/prow/pkg/layeredsets"
	"sigs.k8s.io/prow/pkg/plugins/ownersconfig"
	"sigs.k8s.io/prow/pkg/repoowners"
)

type fakeOwnersClient struct {
	owners map[string]repoowners.RepoOwner
}

func (f *fakeOwnersClient) LoadRepoOwners(org, repo, base string) (repoowners.RepoOwner, error) {
	key := fmt.Sprintf("%s/%s", org, repo)
	if owner, exists := f.owners[key]; exists {
		return owner, nil
	}
	return &fakeRepoOwner{approvers: map[string]bool{}}, nil
}

type fakeRepoOwner struct {
	approvers map[string]bool
}

func (f *fakeRepoOwner) TopLevelApprovers() sets.Set[string] {
	result := sets.New[string]()
	for approver := range f.approvers {
		result.Insert(approver)
	}
	return result
}

func (f *fakeRepoOwner) FindApproverOwnersForFile(path string) string { return "" }
func (f *fakeRepoOwner) FindReviewersOwnersForFile(path string) string { return "" }
func (f *fakeRepoOwner) FindLabelsForFile(path string) sets.Set[string] { return sets.New[string]() }
func (f *fakeRepoOwner) IsNoParentOwners(path string) bool { return false }
func (f *fakeRepoOwner) IsAutoApproveUnownedSubfolders(directory string) bool { return false }
func (f *fakeRepoOwner) LeafApprovers(path string) sets.Set[string] { return sets.New[string]() }
func (f *fakeRepoOwner) Approvers(path string) layeredsets.String { return layeredsets.String{} }
func (f *fakeRepoOwner) LeafReviewers(path string) sets.Set[string] { return sets.New[string]() }
func (f *fakeRepoOwner) Reviewers(path string) layeredsets.String { return layeredsets.String{} }
func (f *fakeRepoOwner) RequiredReviewers(path string) sets.Set[string] { return sets.New[string]() }
func (f *fakeRepoOwner) ParseSimpleConfig(path string) (repoowners.SimpleConfig, error) {
	return repoowners.SimpleConfig{}, nil
}
func (f *fakeRepoOwner) ParseFullConfig(path string) (repoowners.FullConfig, error) {
	return repoowners.FullConfig{}, nil
}
func (f *fakeRepoOwner) Filenames() ownersconfig.Filenames { return ownersconfig.Filenames{} }
func (f *fakeRepoOwner) AllOwners() sets.Set[string] { return sets.New[string]() }
func (f *fakeRepoOwner) AllApprovers() sets.Set[string] { return sets.New[string]() }
func (f *fakeRepoOwner) AllReviewers() sets.Set[string] { return sets.New[string]() }

type fakeGitHubClient struct {
	*fakegithub.FakeClient
	pinCalls   []string
	unpinCalls []string
}

func (f *fakeGitHubClient) PinIssue(org, repo string, number int) error {
	f.pinCalls = append(f.pinCalls, fmt.Sprintf("%s/%s#%d", org, repo, number))
	return f.FakeClient.PinIssue(org, repo, number)
}

func (f *fakeGitHubClient) UnpinIssue(org, repo string, number int) error {
	f.unpinCalls = append(f.unpinCalls, fmt.Sprintf("%s/%s#%d", org, repo, number))
	return f.FakeClient.UnpinIssue(org, repo, number)
}

func (f *fakeGitHubClient) MutateWithGitHubAppsSupport(ctx context.Context, m interface{}, input githubql.Input, vars map[string]interface{}, org string) error {
	// Mock implementation for transfer - just return success
	return nil
}

func TestHandlePin(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		isApprover     bool
		expectComment  bool
		expectPinCall  bool
		expectError    bool
	}{
		{
			name:           "pin command by approver",
			body:           "/issue-pin",
			isApprover:     true,
			expectComment:  true,
			expectPinCall:  true,
			expectError:    false,
		},
		{
			name:           "pin command by non-approver",
			body:           "/issue-pin",
			isApprover:     false,
			expectComment:  true,
			expectPinCall:  false,
			expectError:    false,
		},
		{
			name:           "unpin command by approver",
			body:           "/issue-unpin",
			isApprover:     true,
			expectComment:  true,
			expectPinCall:  false,
			expectError:    false,
		},
		{
			name:           "no matching command",
			body:           "/some-other-command",
			isApprover:     true,
			expectComment:  false,
			expectPinCall:  false,
			expectError:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := &fakeGitHubClient{
				FakeClient: &fakegithub.FakeClient{
					Issues: map[int]*github.Issue{
						1: {
							Number: 1,
							NodeID: "issue-node-id",
						},
					},
					IssueComments: map[int][]github.IssueComment{},
				},
			}

			ownersClient := &fakeOwnersClient{
				owners: map[string]repoowners.RepoOwner{},
			}

			if test.isApprover {
				ownersClient.owners["test-org/test-repo"] = &fakeRepoOwner{
					approvers: map[string]bool{"test-user": true},
				}
			}

			log := logrus.NewEntry(logrus.New())

			event := github.GenericCommentEvent{
				Action: github.GenericCommentActionCreated,
				Body:   test.body,
				Number: 1,
				Repo: github.Repo{
					Owner: github.User{Login: "test-org"},
					Name:  "test-repo",
				},
				User:    github.User{Login: "test-user"},
				HTMLURL: "https://github.com/test-org/test-repo/issues/1",
				NodeID:  "issue-node-id",
			}

			err := handleIssueManagement(fakeClient, ownersClient, log, event)

			if test.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !test.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if test.expectComment {
				if len(fakeClient.IssueComments[1]) == 0 {
					t.Error("Expected comment but none was created")
				}
			} else {
				if len(fakeClient.IssueComments[1]) > 0 {
					t.Error("Unexpected comment was created")
				}
			}

			if test.expectPinCall {
				if len(fakeClient.pinCalls) == 0 && len(fakeClient.unpinCalls) == 0 {
					t.Error("Expected pin/unpin call but none was made")
				}
			}
		})
	}
}

func TestHandleTransfer(t *testing.T) {
	fakeClient := &fakeGitHubClient{
		FakeClient: &fakegithub.FakeClient{
			Issues: map[int]*github.Issue{
				1: {
					Number: 1,
					NodeID: "issue-node-id",
				},
			},
			IssueComments: map[int][]github.IssueComment{},
			OrgMembers:    map[string][]string{"test-org": {"test-user"}},
		},
	}

	ownersClient := &fakeOwnersClient{
		owners: map[string]repoowners.RepoOwner{},
	}

	log := logrus.NewEntry(logrus.New())

	event := github.GenericCommentEvent{
		Action: github.GenericCommentActionCreated,
		Body:   "/issue-transfer destination",
		Number: 1,
		Repo: github.Repo{
			Owner: github.User{Login: "test-org"},
			Name:  "test-repo",
		},
		User:    github.User{Login: "test-user"},
		HTMLURL: "https://github.com/test-org/test-repo/issues/1",
		NodeID:  "issue-node-id",
	}

	err := handleIssueManagement(fakeClient, ownersClient, log, event)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(fakeClient.IssueComments[1]) == 0 {
		t.Error("Expected comment but none was created")
	}
}