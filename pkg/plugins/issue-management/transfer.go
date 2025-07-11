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
	"strings"

	githubql "github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/prow/pkg/github"
	"sigs.k8s.io/prow/pkg/plugins"
)

// handleTransfer handles the /issue-transfer command
func handleTransfer(gc githubClient, log *logrus.Entry, org, repo string, number int, user string, e github.GenericCommentEvent) error {
	log.WithFields(logrus.Fields{
		"org":    org,
		"repo":   repo,
		"number": number,
		"user":   user,
	}).Info("Handling issue transfer command")

	// Parse the command to extract destination repo
	matches := transferRe.FindAllStringSubmatch(e.Body, -1)
	if len(matches) == 0 {
		return nil
	}
	if len(matches) != 1 || len(matches[0]) != 2 || len(matches[0][1]) == 0 {
		return gc.CreateComment(
			org, repo, number,
			plugins.FormatResponseRaw(e.Body, e.HTMLURL, user, "/issue-transfer must only be used once and with a single destination repo."),
		)
	}

	dstRepoName := strings.TrimSpace(matches[0][1])
	dstRepoPair := org + "/" + dstRepoName

	// Check if destination repo exists
	dstRepo, err := gc.GetRepo(org, dstRepoName)
	if err != nil {
		log.WithError(err).WithField("dstRepo", dstRepoPair).Warning("could not fetch destination repo")
		return gc.CreateComment(
			org, repo, number,
			plugins.FormatResponseRaw(e.Body, e.HTMLURL, user, fmt.Sprintf("Something went wrong or the destination repo %s does not exist.", dstRepoPair)),
		)
	}

	// Check if user is authorized (org member)
	isMember, err := gc.IsMember(org, user)
	if err != nil {
		return fmt.Errorf("unable to fetch if %s is an org member of %s: %w", user, org, err)
	}
	if !isMember {
		return gc.CreateComment(
			org, repo, number,
			plugins.FormatResponseRaw(e.Body, e.HTMLURL, user, "You must be an org member to transfer this issue."),
		)
	}

	// Transfer the issue using GraphQL mutation
	m, err := transferIssue(gc, org, dstRepo.NodeID, e.NodeID)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"issueNumber": number,
			"srcRepo":     org + "/" + repo,
			"dstRepo":     dstRepoPair,
		}).Error("issue could not be transferred")
		return gc.CreateComment(
			org, repo, number,
			plugins.FormatResponseRaw(e.Body, e.HTMLURL, user, fmt.Sprintf("Failed to transfer issue #%d to %s: %v", number, dstRepoPair, err)),
		)
	}

	log.WithFields(logrus.Fields{
		"user":        user,
		"org":         org,
		"srcRepo":     repo,
		"issueNumber": number,
		"dstURL":      m.TransferIssue.Issue.URL,
	}).Info("successfully transferred issue")

	return gc.CreateComment(
		org, repo, number,
		plugins.FormatResponseRaw(e.Body, e.HTMLURL, user, fmt.Sprintf("Issue #%d has been transferred to %s. New URL: %s", number, dstRepoPair, m.TransferIssue.Issue.URL)),
	)
}

// transferIssueMutation is a GraphQL mutation struct compatible with shurcooL/githubql's client
//
// See https://docs.github.com/en/graphql/reference/input-objects#transferissueinput
type transferIssueMutation struct {
	TransferIssue struct {
		Issue struct {
			URL githubql.URI
		}
	} `graphql:"transferIssue(input: $input)"`
}

// transferIssue will move an issue from one repo to another in the same org.
//
// See https://docs.github.com/en/graphql/reference/mutations#transferissue
func transferIssue(gc githubClient, org, dstRepoNodeID string, issueNodeID string) (*transferIssueMutation, error) {
	m := &transferIssueMutation{}
	input := githubql.TransferIssueInput{
		IssueID:      githubql.ID(issueNodeID),
		RepositoryID: githubql.ID(dstRepoNodeID),
	}
	err := gc.MutateWithGitHubAppsSupport(context.Background(), m, input, nil, org)
	return m, err
}