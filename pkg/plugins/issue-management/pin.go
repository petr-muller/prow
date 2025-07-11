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
	"fmt"

	"github.com/sirupsen/logrus"

	"sigs.k8s.io/prow/pkg/github"
	"sigs.k8s.io/prow/pkg/plugins"
)

// handlePin handles the /issue-pin command
func handlePin(gc githubClient, oc ownersClient, log *logrus.Entry, org, repo string, number int, user string, e github.GenericCommentEvent) error {
	log.WithFields(logrus.Fields{
		"org":    org,
		"repo":   repo,
		"number": number,
		"user":   user,
	}).Info("Handling issue pin command")

	// Check if user is authorized (top-level OWNERS approvers)
	if !authorizedTopLevelOwner(oc, log, org, repo, user) {
		return gc.CreateComment(
			org, repo, number,
			plugins.FormatResponseRaw(e.Body, e.HTMLURL, user, fmt.Sprintf("You are not authorized to pin issues. Only top-level OWNERS approvers can use this command.")),
		)
	}

	// Pin the issue
	if err := gc.PinIssue(org, repo, number); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"org":    org,
			"repo":   repo,
			"number": number,
		}).Error("Failed to pin issue")
		return gc.CreateComment(
			org, repo, number,
			plugins.FormatResponseRaw(e.Body, e.HTMLURL, user, fmt.Sprintf("Failed to pin issue #%d: %v", number, err)),
		)
	}

	log.WithFields(logrus.Fields{
		"org":    org,
		"repo":   repo,
		"number": number,
		"user":   user,
	}).Info("Successfully pinned issue")

	return gc.CreateComment(
		org, repo, number,
		plugins.FormatResponseRaw(e.Body, e.HTMLURL, user, fmt.Sprintf("Issue #%d has been pinned to the repository.", number)),
	)
}

// handleUnpin handles the /issue-unpin command
func handleUnpin(gc githubClient, oc ownersClient, log *logrus.Entry, org, repo string, number int, user string, e github.GenericCommentEvent) error {
	log.WithFields(logrus.Fields{
		"org":    org,
		"repo":   repo,
		"number": number,
		"user":   user,
	}).Info("Handling issue unpin command")

	// Check if user is authorized (top-level OWNERS approvers)
	if !authorizedTopLevelOwner(oc, log, org, repo, user) {
		return gc.CreateComment(
			org, repo, number,
			plugins.FormatResponseRaw(e.Body, e.HTMLURL, user, fmt.Sprintf("You are not authorized to unpin issues. Only top-level OWNERS approvers can use this command.")),
		)
	}

	// Unpin the issue
	if err := gc.UnpinIssue(org, repo, number); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"org":    org,
			"repo":   repo,
			"number": number,
		}).Error("Failed to unpin issue")
		return gc.CreateComment(
			org, repo, number,
			plugins.FormatResponseRaw(e.Body, e.HTMLURL, user, fmt.Sprintf("Failed to unpin issue #%d: %v", number, err)),
		)
	}

	log.WithFields(logrus.Fields{
		"org":    org,
		"repo":   repo,
		"number": number,
		"user":   user,
	}).Info("Successfully unpinned issue")

	return gc.CreateComment(
		org, repo, number,
		plugins.FormatResponseRaw(e.Body, e.HTMLURL, user, fmt.Sprintf("Issue #%d has been unpinned from the repository.", number)),
	)
}

// authorizedTopLevelOwner checks if the user is a top-level OWNERS approver
func authorizedTopLevelOwner(oc ownersClient, log *logrus.Entry, org, repo, user string) bool {
	// For issues, use the default branch (main/master)
	owners, err := oc.LoadRepoOwners(org, repo, "")
	if err != nil {
		log.WithError(err).Warnf("Cannot determine whether %s is a top-level owner of %s/%s", user, org, repo)
		return false
	}
	return owners.TopLevelApprovers().Has(github.NormLogin(user))
}