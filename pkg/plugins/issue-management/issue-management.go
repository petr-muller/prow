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

// Package issuemanagement implements issue management commands including
// /issue-pin, /issue-unpin, and /issue-transfer
package issuemanagement

import (
	"context"
	"regexp"

	githubql "github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/prow/pkg/config"
	"sigs.k8s.io/prow/pkg/github"
	"sigs.k8s.io/prow/pkg/pluginhelp"
	"sigs.k8s.io/prow/pkg/plugins"
	"sigs.k8s.io/prow/pkg/repoowners"
)

const pluginName = "issue-management"

var (
	pinRe      = regexp.MustCompile(`(?mi)^/issue-pin\s*$`)
	unpinRe    = regexp.MustCompile(`(?mi)^/issue-unpin\s*$`)
	transferRe = regexp.MustCompile(`(?mi)^/issue-transfer(?: +(.*))?$`)
)

type githubClient interface {
	CreateComment(org, repo string, number int, comment string) error
	IsMember(org, user string) (bool, error)
	PinIssue(org, repo string, number int) error
	UnpinIssue(org, repo string, number int) error
	GetRepo(org, name string) (github.FullRepo, error)
	MutateWithGitHubAppsSupport(context.Context, interface{}, githubql.Input, map[string]interface{}, string) error
}

type ownersClient interface {
	LoadRepoOwners(org, repo, base string) (repoowners.RepoOwner, error)
}

func init() {
	plugins.RegisterGenericCommentHandler(pluginName, handleGenericComment, helpProvider)
}

func helpProvider(config *plugins.Configuration, _ []config.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "The issue-management plugin provides issue management commands for pinning and transferring issues.",
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/issue-pin",
		Description: "Pin an issue to the repository",
		Featured:    true,
		WhoCanUse:   "Top-level OWNERS file approvers",
		Examples:    []string{"/issue-pin"},
	})
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/issue-unpin",
		Description: "Unpin an issue from the repository",
		Featured:    true,
		WhoCanUse:   "Top-level OWNERS file approvers",
		Examples:    []string{"/issue-unpin"},
	})
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/issue-transfer <destination repo in same org>",
		Description: "Transfer an issue to a different repository in the same organization",
		Featured:    true,
		WhoCanUse:   "Organization members",
		Examples:    []string{"/issue-transfer kubectl", "/issue-transfer test-infra"},
	})
	return pluginHelp, nil
}

func handleGenericComment(pc plugins.Agent, e github.GenericCommentEvent) error {
	return handleIssueManagement(pc.GitHubClient, pc.OwnersClient, pc.Logger, e)
}

func handleIssueManagement(gc githubClient, oc ownersClient, log *logrus.Entry, e github.GenericCommentEvent) error {
	// Only handle comments on issues, not PRs
	if e.IsPR || e.Action != github.GenericCommentActionCreated {
		return nil
	}

	org := e.Repo.Owner.Login
	repo := e.Repo.Name
	number := e.Number
	user := e.User.Login

	// Check for pin command
	if pinRe.MatchString(e.Body) {
		return handlePin(gc, oc, log, org, repo, number, user, e)
	}

	// Check for unpin command
	if unpinRe.MatchString(e.Body) {
		return handleUnpin(gc, oc, log, org, repo, number, user, e)
	}

	// Check for transfer command
	if transferRe.MatchString(e.Body) {
		return handleTransfer(gc, log, org, repo, number, user, e)
	}

	return nil
}