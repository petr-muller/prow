/*
Copyright 2026 The Kubernetes Authors.

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

package reviewer

import (
	"fmt"
	"slices"

	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/prow/pkg/layeredsets"

	"sigs.k8s.io/prow/pkg/github"
	"sigs.k8s.io/prow/pkg/plugins"
	"sigs.k8s.io/prow/pkg/plugins/assign"
)

// SelectionContext carries the inputs a SelectionStrategy may need to prepare
// reviewer selection for one pull request.
type SelectionContext struct {
	GitHubClient          GitHubClient
	OwnersClient          OwnersClient
	Log                   *logrus.Entry
	PR                    *github.PullRequest
	Repo                  *github.Repo
	Changes               []github.PullRequestChange
	UseStatusAvailability bool
	BusyReviewers         *sets.Set[string]
}

// Selection selects reviewers for a single pull request.
type Selection interface {
	Selector(candidates *layeredsets.String) string
}

// FallbackSelection is implemented by strategies that can propose additional
// reviewers from a broader OWNERS scope when the requested count cannot be met.
type FallbackSelection interface {
	Selection
	Fallback(exclude sets.Set[string], needed int) []string
}

// SelectionStrategy provides the plugin-specific reviewer selection behavior
// used by the shared event handlers.
type SelectionStrategy interface {
	NewSelection(ctx SelectionContext) Selection
}

// RandomStrategy selects reviewers randomly using FindReviewer. It is the
// strategy used by the blunderbuss plugin.
type RandomStrategy struct{}

func (RandomStrategy) NewSelection(ctx SelectionContext) Selection {
	return randomSelection{ctx: ctx}
}

type randomSelection struct {
	ctx SelectionContext
}

func (s randomSelection) Selector(candidates *layeredsets.String) string {
	return FindReviewer(s.ctx.GitHubClient, s.ctx.Log, s.ctx.UseStatusAvailability, s.ctx.BusyReviewers, candidates)
}

// HandlePullRequestEvent handles pull request events for a reviewer selection plugin.
func HandlePullRequestEvent(pc plugins.Agent, cfg *plugins.ReviewerSelectionConfig, strategy SelectionStrategy, pre github.PullRequestEvent) error {
	return HandlePullRequest(
		pc.GitHubClient,
		pc.OwnersClient,
		pc.Logger,
		cfg,
		strategy,
		pre.Action,
		&pre.PullRequest,
		&pre.Repo,
	)
}

// HandlePullRequest requests reviewers for a pull request event.
func HandlePullRequest(ghc GitHubClient, roc RepoOwnersClient, log *logrus.Entry, cfg *plugins.ReviewerSelectionConfig, strategy SelectionStrategy, action github.PullRequestEventAction, pr *github.PullRequest, repo *github.Repo) error {
	if !(action == github.PullRequestActionOpened || action == github.PullRequestActionReadyForReview) || assign.CCRegexp.MatchString(pr.Body) || cfg.WaitForStatus != nil {
		return nil
	}
	if pr.Draft && cfg.IgnoreDrafts {
		return nil
	}
	if slices.Contains(cfg.IgnoreAuthors, pr.User.Login) {
		return nil
	}
	return Handle(
		ghc,
		roc,
		log,
		strategy,
		cfg.ReviewerCount,
		cfg.MaxReviewerCount,
		cfg.ExcludeApprovers,
		cfg.UseStatusAvailability,
		repo,
		pr,
	)
}

// HandleGenericCommentEvent handles generic comment events for a reviewer selection plugin.
func HandleGenericCommentEvent(pc plugins.Agent, cfg *plugins.ReviewerSelectionConfig, strategy SelectionStrategy, ce github.GenericCommentEvent) error {
	return HandleGenericComment(
		pc.GitHubClient,
		pc.OwnersClient,
		pc.Logger,
		cfg,
		strategy,
		ce.Action,
		ce.IsPR,
		ce.Number,
		ce.IssueState,
		&ce.Repo,
		ce.Body,
	)
}

// HandleGenericComment requests reviewers in response to an /auto-cc comment.
func HandleGenericComment(ghc GitHubClient, roc RepoOwnersClient, log *logrus.Entry, cfg *plugins.ReviewerSelectionConfig, strategy SelectionStrategy, action github.GenericCommentEventAction, isPR bool, prNumber int, issueState string, repo *github.Repo, body string) error {
	if action != github.GenericCommentActionCreated || !isPR || issueState == "closed" {
		return nil
	}

	if !AutoCCMatch.MatchString(body) {
		return nil
	}

	pr, err := ghc.GetPullRequest(repo.Owner.Login, repo.Name, prNumber)
	if err != nil {
		return fmt.Errorf("error loading PullRequest: %w", err)
	}

	return Handle(
		ghc,
		roc,
		log,
		strategy,
		cfg.ReviewerCount,
		cfg.MaxReviewerCount,
		cfg.ExcludeApprovers,
		cfg.UseStatusAvailability,
		repo,
		pr,
	)
}

// HandleStatusEvent handles commit status events for a reviewer selection plugin.
func HandleStatusEvent(pc plugins.Agent, cfg *plugins.ReviewerSelectionConfig, strategy SelectionStrategy, se github.StatusEvent) error {
	if se.State == "" || se.Context == "" || se.Description == "" {
		return fmt.Errorf("invalid status event delivered with empty state/context/description")
	}

	return HandleStatus(
		pc.GitHubClient,
		pc.OwnersClient,
		pc.Logger,
		cfg,
		strategy,
		se.SHA,
		se.Context,
		se.State,
		se.Description,
		&se.Repo,
	)
}

// HandleStatus requests reviewers after a configured commit status is observed.
func HandleStatus(ghc GitHubClient, roc RepoOwnersClient, log *logrus.Entry, cfg *plugins.ReviewerSelectionConfig, strategy SelectionStrategy, sha string, context string, state string, description string, repo *github.Repo) error {
	wfs := cfg.WaitForStatus
	if wfs == nil {
		return nil
	}

	if context != wfs.Context {
		return nil
	}

	if state != wfs.State {
		return nil
	}

	if !wfs.DescriptionRe.MatchString(description) {
		return nil
	}

	org := repo.Owner.Login
	log.Info("Searching for PRs matching the commit.")

	issues, err := ghc.FindIssuesWithOrg(org, fmt.Sprintf("%s repo:%s/%s type:pr state:open", sha, org, repo.Name), "", false)
	if err != nil {
		return fmt.Errorf("error searching for issues matching commit: %w", err)
	}
	log.Infof("Found %d PRs matching commit.", len(issues))

	var errs []error
	for _, issue := range issues {
		l := log.WithField("pr", issue.Number)

		l.Info("Getting pull request info.")
		pr, err := ghc.GetPullRequest(org, repo.Name, issue.Number)
		if err != nil {
			l.WithError(err).Warning("Unable to fetch pull request.")
			continue
		}

		if pr.Head.SHA != sha {
			l.Info("Event is not for PR HEAD, skipping.")
			continue
		}

		if pr.Draft && cfg.IgnoreDrafts {
			continue
		}

		if slices.Contains(cfg.IgnoreAuthors, pr.User.Login) {
			continue
		}

		if len(pr.RequestedReviewers) > 0 {
			continue
		}

		if err := Handle(
			ghc,
			roc,
			l,
			strategy,
			cfg.ReviewerCount,
			cfg.MaxReviewerCount,
			cfg.ExcludeApprovers,
			cfg.UseStatusAvailability,
			repo,
			pr); err != nil {
			l.WithError(err).Warning("Error processing event from commit status update")
			errs = append(errs, err)
		}
	}
	return utilerrors.NewAggregate(errs)
}

// Handle requests reviewers for a pull request using the given strategy.
func Handle(ghc GitHubClient, roc RepoOwnersClient, log *logrus.Entry, strategy SelectionStrategy, reviewerCount *int, maxReviewers int, excludeApprovers bool, useStatusAvailability bool, repo *github.Repo, pr *github.PullRequest) error {
	oc, err := roc.LoadRepoOwners(repo.Owner.Login, repo.Name, pr.Base.Ref)
	if err != nil {
		return fmt.Errorf("error loading RepoOwners: %w", err)
	}

	changes, err := ghc.GetPullRequestChanges(repo.Owner.Login, repo.Name, pr.Number)
	if err != nil {
		return fmt.Errorf("error getting PR changes: %w", err)
	}

	busyReviewers := sets.New[string]()
	selection := strategy.NewSelection(SelectionContext{
		GitHubClient:          ghc,
		OwnersClient:          oc,
		Log:                   log,
		PR:                    pr,
		Repo:                  repo,
		Changes:               changes,
		UseStatusAvailability: useStatusAvailability,
		BusyReviewers:         &busyReviewers,
	})
	selector := selection.Selector

	var reviewers []string
	var requiredReviewers []string
	if reviewerCount != nil {
		reviewers, requiredReviewers, err = GetReviewers(oc, selector, log, pr.User.Login, changes, *reviewerCount)
		if err != nil {
			return err
		}
		if missing := *reviewerCount - len(reviewers); missing > 0 {
			if !excludeApprovers {
				frc := FallbackReviewersClient{OwnersClient: oc}
				approvers, _, err := GetReviewers(frc, selector, log, pr.User.Login, changes, *reviewerCount)
				if err != nil {
					return err
				}
				var added int
				combinedReviewers := sets.New[string](reviewers...)
				for _, approver := range approvers {
					if !combinedReviewers.Has(approver) {
						reviewers = append(reviewers, approver)
						combinedReviewers.Insert(approver)
						added++
					}
				}
				log.Infof("Added %d approvers as reviewers. %d/%d reviewers found.", added, combinedReviewers.Len(), *reviewerCount)
			}
		}
		if missing := *reviewerCount - len(reviewers); missing > 0 {
			if fallback, ok := selection.(FallbackSelection); ok {
				log.Debugf("Not enough reviewers found in OWNERS files for files touched by this PR. %d/%d reviewers found.", len(reviewers), *reviewerCount)

				excludeFromFallback := sets.New[string](reviewers...)
				excludeFromFallback.Insert(github.NormLogin(pr.User.Login))
				fallbackReviewers := fallback.Fallback(excludeFromFallback, missing)
				if len(fallbackReviewers) > 0 {
					reviewers = append(reviewers, fallbackReviewers...)
					log.Infof("Fallback added %d reviewer(s) from broader OWNERS scope: %v", len(fallbackReviewers), fallbackReviewers)
				}
			}
		}
	}

	if maxReviewers > 0 && len(reviewers) > maxReviewers {
		log.Infof("Limiting request of %d reviewers to %d maxReviewers.", len(reviewers), maxReviewers)
		reviewers = reviewers[:maxReviewers]
	}

	reviewers = append(reviewers, requiredReviewers...)

	if len(reviewers) > 0 {
		log.Infof("Requesting reviews from users %s.", reviewers)
		return ghc.RequestReview(repo.Owner.Login, repo.Name, pr.Number, reviewers)
	}
	return nil
}
