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

package rifle

import (
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/prow/pkg/layeredsets"

	"sigs.k8s.io/prow/pkg/config"
	"sigs.k8s.io/prow/pkg/github"
	"sigs.k8s.io/prow/pkg/pluginhelp"
	"sigs.k8s.io/prow/pkg/plugins"
	"sigs.k8s.io/prow/pkg/reviewer"
)

const (
	PluginName = "rifle"
)

func init() {
	plugins.RegisterPullRequestHandler(PluginName, handlePullRequestEvent, helpProvider)
	plugins.RegisterGenericCommentHandler(PluginName, handleGenericCommentEvent, helpProvider)
	plugins.RegisterStatusEventHandler(PluginName, handleStatusEvent, helpProvider)
}

type rifleGitHubClient interface {
	reviewer.GitHubClient
	GetBlame(org, repo, ref, path string) ([]github.BlameRange, error)
	GetMergeBase(org, repo, base, head string) (string, error)
}

func helpProvider(config *plugins.Configuration, _ []config.OrgRepo) (*pluginhelp.PluginHelp, error) {
	var reviewCount int
	if config.Rifle.ReviewerCount != nil {
		reviewCount = *config.Rifle.ReviewerCount
	}
	two := 2
	yamlSnippet, err := plugins.CommentMap.GenYaml(&plugins.Configuration{
		Rifle: plugins.Rifle{
			ReviewerCount:         &two,
			MaxReviewerCount:      3,
			ExcludeApprovers:      true,
			UseStatusAvailability: true,
			IgnoreAuthors:         []string{},
		},
	})
	if err != nil {
		logrus.WithError(err).Warnf("cannot generate comments for %s plugin", PluginName)
	}
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "The rifle plugin automatically requests reviews from reviewers when a new PR is created. Unlike blunderbuss (which selects randomly), rifle uses git blame data to identify reviewers most familiar with the changed code.",
		Config: map[string]string{
			"": reviewer.ConfigString(PluginName, reviewCount),
		},
		Snippet: yamlSnippet,
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/auto-cc",
		Featured:    false,
		Description: "Manually request reviews from reviewers for a PR. Useful if OWNERS files were updated since the PR was opened, or PR was changed significantly.",
		Examples:    []string{"/auto-cc"},
		WhoCanUse:   "Anyone",
	})
	return pluginHelp, nil
}

// blameStrategy selects reviewers using git blame data, falling back to random
// selection when blame scores are unavailable.
type blameStrategy struct {
	ghc rifleGitHubClient
}

func (s blameStrategy) NewSelection(ctx reviewer.SelectionContext) reviewer.Selection {
	allReviewerCandidates := sets.New[string]()
	allApproverCandidates := sets.New[string]()
	for _, file := range ctx.Changes {
		allReviewerCandidates = allReviewerCandidates.Union(ctx.OwnersClient.Reviewers(file.Filename).Set())
		allApproverCandidates = allApproverCandidates.Union(ctx.OwnersClient.Approvers(file.Filename).Set())
	}

	// PR file patches are three-dot diffs: old-side hunk line numbers refer to
	// the merge-base of head and base, not the current tip of the base branch.
	ref := ctx.PR.Base.Ref
	if ctx.PR.Head.SHA != "" {
		mergeBase, err := s.ghc.GetMergeBase(ctx.Repo.Owner.Login, ctx.Repo.Name, ctx.PR.Base.Ref, ctx.PR.Head.SHA)
		if err != nil {
			ctx.Log.WithError(err).Warn("Failed to resolve merge-base for blame; falling back to base ref")
		} else if mergeBase != "" {
			ref = mergeBase
		}
	}

	scorer := &reviewerScorer{
		ghc:       s.ghc,
		org:       ctx.Repo.Owner.Login,
		repo:      ctx.Repo.Name,
		ref:       ref,
		approvers: allApproverCandidates,
		reviewers: allReviewerCandidates,
		now:       time.Now(),
		log:       ctx.Log,
	}
	var blameScores map[string]float64
	scores, err := scorer.scoreReviewers(ctx.Changes)
	if err != nil {
		ctx.Log.WithError(err).Warn("Failed to compute blame scores, falling back to random selection")
	} else {
		blameScores = scores
	}

	return &blameSelection{
		blameScores:           blameScores,
		ghc:                   s.ghc,
		oc:                    ctx.OwnersClient,
		log:                   ctx.Log,
		useStatusAvailability: ctx.UseStatusAvailability,
		busyReviewers:         ctx.BusyReviewers,
	}
}

type blameSelection struct {
	blameScores           map[string]float64
	ghc                   rifleGitHubClient
	oc                    reviewer.OwnersClient
	log                   *logrus.Entry
	useStatusAvailability bool
	busyReviewers         *sets.Set[string]
}

func (s *blameSelection) Selector(candidates *layeredsets.String) string {
	if hasBlameScores(s.blameScores) {
		return selectBestReviewer(s.blameScores, candidates, s.busyReviewers, s.ghc, s.log, s.useStatusAvailability)
	}
	return reviewer.FindReviewer(s.ghc, s.log, s.useStatusAvailability, s.busyReviewers, candidates)
}

func (s *blameSelection) Fallback(exclude sets.Set[string], needed int) []string {
	return findFallbackReviewers(s.blameScores, s.oc, exclude, needed, s.busyReviewers, s.ghc, s.log, s.useStatusAvailability)
}

func handlePullRequestEvent(pc plugins.Agent, pre github.PullRequestEvent) error {
	return reviewer.HandlePullRequestEvent(pc, &pc.PluginConfig.Rifle, blameStrategy{ghc: pc.GitHubClient}, pre)
}

func handlePullRequest(ghc rifleGitHubClient, roc reviewer.RepoOwnersClient, log *logrus.Entry, cfg plugins.Rifle, action github.PullRequestEventAction, pr *github.PullRequest, repo *github.Repo) error {
	return reviewer.HandlePullRequest(ghc, roc, log, &cfg, blameStrategy{ghc: ghc}, action, pr, repo)
}

func handleGenericCommentEvent(pc plugins.Agent, ce github.GenericCommentEvent) error {
	return reviewer.HandleGenericCommentEvent(pc, &pc.PluginConfig.Rifle, blameStrategy{ghc: pc.GitHubClient}, ce)
}

func handleGenericComment(ghc rifleGitHubClient, roc reviewer.RepoOwnersClient, log *logrus.Entry, cfg plugins.Rifle, action github.GenericCommentEventAction, isPR bool, prNumber int, issueState string, repo *github.Repo, body string) error {
	return reviewer.HandleGenericComment(ghc, roc, log, &cfg, blameStrategy{ghc: ghc}, action, isPR, prNumber, issueState, repo, body)
}

func handleStatusEvent(pc plugins.Agent, se github.StatusEvent) error {
	return reviewer.HandleStatusEvent(pc, &pc.PluginConfig.Rifle, blameStrategy{ghc: pc.GitHubClient}, se)
}

func handleStatus(ghc rifleGitHubClient, roc reviewer.RepoOwnersClient, log *logrus.Entry, cfg plugins.Rifle, sha string, context string, state string, description string, repo *github.Repo) error {
	return reviewer.HandleStatus(ghc, roc, log, &cfg, blameStrategy{ghc: ghc}, sha, context, state, description, repo)
}

func handle(ghc rifleGitHubClient, roc reviewer.RepoOwnersClient, log *logrus.Entry, reviewerCount *int, maxReviewers int, excludeApprovers bool, useStatusAvailability bool, repo *github.Repo, pr *github.PullRequest) error {
	return reviewer.Handle(ghc, roc, log, blameStrategy{ghc: ghc}, reviewerCount, maxReviewers, excludeApprovers, useStatusAvailability, repo, pr)
}

func selectBestReviewer(
	scores map[string]float64,
	candidates *layeredsets.String,
	busyReviewers *sets.Set[string],
	ghc reviewer.GitHubClient,
	log *logrus.Entry,
	useStatusAvailability bool,
) string {
	type candidate struct {
		login string
		score float64
	}

	var ranked []candidate
	candidateSet := candidates.Set()
	for login := range candidateSet {
		if busyReviewers.Has(login) {
			continue
		}
		ranked = append(ranked, candidate{login: login, score: scores[login]})
	}

	rand.Shuffle(len(ranked), func(i, j int) {
		ranked[i], ranked[j] = ranked[j], ranked[i]
	})
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	for _, c := range ranked {
		if !isReviewerAvailable(c.login, busyReviewers, ghc, log, useStatusAvailability) {
			continue
		}
		candidates.Delete(c.login)
		return c.login
	}
	return ""
}

// isReviewerAvailable reports whether login can be asked for a review. Users
// already known to be busy are rejected without another API call; when
// useStatusAvailability is enabled, the remaining candidates are checked and
// busy users are recorded in busyReviewers. Errors are treated as available.
func isReviewerAvailable(
	login string,
	busyReviewers *sets.Set[string],
	ghc reviewer.GitHubClient,
	log *logrus.Entry,
	useStatusAvailability bool,
) bool {
	if busyReviewers.Has(login) {
		return false
	}
	if !useStatusAvailability {
		return true
	}
	busy, err := reviewer.IsUserBusy(ghc, login)
	if err != nil {
		log.WithError(err).WithField("user", login).Warn("Failed to check user availability")
		return true
	}
	if busy {
		busyReviewers.Insert(login)
		return false
	}
	return true
}

func hasBlameScores(scores map[string]float64) bool {
	for _, s := range scores {
		if !math.IsNaN(s) && s > 0 {
			return true
		}
	}
	return false
}

func findFallbackReviewers(
	blameScores map[string]float64,
	oc reviewer.OwnersClient,
	exclude sets.Set[string],
	needed int,
	busyReviewers *sets.Set[string],
	ghc reviewer.GitHubClient,
	log *logrus.Entry,
	useStatusAvailability bool,
) []string {
	allOwners := oc.AllOwners()
	eligible := allOwners.Difference(exclude)
	if eligible.Len() == 0 {
		return nil
	}

	var result []string
	if hasBlameScores(blameScores) {
		type scored struct {
			login string
			score float64
		}
		var candidates []scored
		for login := range eligible {
			if s := blameScores[login]; s > 0 {
				candidates = append(candidates, scored{login, s})
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].score > candidates[j].score
		})
		for i := 0; i < len(candidates) && len(result) < needed; i++ {
			if !isReviewerAvailable(candidates[i].login, busyReviewers, ghc, log, useStatusAvailability) {
				continue
			}
			result = append(result, candidates[i].login)
		}
	}

	if remaining := needed - len(result); remaining > 0 {
		picked := sets.New[string](result...)
		var pool []string
		for login := range eligible {
			if !picked.Has(login) {
				pool = append(pool, login)
			}
		}
		rand.Shuffle(len(pool), func(i, j int) {
			pool[i], pool[j] = pool[j], pool[i]
		})
		for i := 0; i < len(pool) && len(result) < needed; i++ {
			if !isReviewerAvailable(pool[i], busyReviewers, ghc, log, useStatusAvailability) {
				continue
			}
			result = append(result, pool[i])
		}
	}

	return result
}
