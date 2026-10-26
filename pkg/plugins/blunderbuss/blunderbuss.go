/*
Copyright 2017 The Kubernetes Authors.

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

package blunderbuss

import (
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/prow/pkg/config"
	"sigs.k8s.io/prow/pkg/github"
	"sigs.k8s.io/prow/pkg/pluginhelp"
	"sigs.k8s.io/prow/pkg/plugins"
	"sigs.k8s.io/prow/pkg/reviewer"
)

const (
	PluginName = "blunderbuss"
)

func init() {
	plugins.RegisterPullRequestHandler(PluginName, handlePullRequestEvent, helpProvider)
	plugins.RegisterGenericCommentHandler(PluginName, handleGenericCommentEvent, helpProvider)
	plugins.RegisterStatusEventHandler(PluginName, handleStatusEvent, helpProvider)
}

func helpProvider(config *plugins.Configuration, _ []config.OrgRepo) (*pluginhelp.PluginHelp, error) {
	var reviewCount int
	if config.Blunderbuss.ReviewerCount != nil {
		reviewCount = *config.Blunderbuss.ReviewerCount
	}
	two := 2
	yamlSnippet, err := plugins.CommentMap.GenYaml(&plugins.Configuration{
		Blunderbuss: plugins.Blunderbuss{
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
		Description: "The blunderbuss plugin automatically requests reviews from reviewers when a new PR is created. The reviewers are selected randomly from the OWNERS files that apply to the files modified by the PR.",
		Config: map[string]string{
			"": reviewer.ConfigString(PluginName, reviewCount),
		},
		Snippet: yamlSnippet,
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/auto-cc",
		Featured:    false,
		Description: "Manually request reviews from reviewers for a PR. Useful if OWNERS file were updated since the PR was opened.",
		Examples:    []string{"/auto-cc"},
		WhoCanUse:   "Anyone",
	})
	return pluginHelp, nil
}

func handlePullRequestEvent(pc plugins.Agent, pre github.PullRequestEvent) error {
	return reviewer.HandlePullRequestEvent(pc, &pc.PluginConfig.Blunderbuss, reviewer.RandomStrategy{}, pre)
}

func handlePullRequest(ghc reviewer.GitHubClient, roc reviewer.RepoOwnersClient, log *logrus.Entry, config plugins.Blunderbuss, action github.PullRequestEventAction, pr *github.PullRequest, repo *github.Repo) error {
	return reviewer.HandlePullRequest(ghc, roc, log, &config, reviewer.RandomStrategy{}, action, pr, repo)
}

func handleGenericCommentEvent(pc plugins.Agent, ce github.GenericCommentEvent) error {
	return reviewer.HandleGenericCommentEvent(pc, &pc.PluginConfig.Blunderbuss, reviewer.RandomStrategy{}, ce)
}

func handleGenericComment(ghc reviewer.GitHubClient, roc reviewer.RepoOwnersClient, log *logrus.Entry, config plugins.Blunderbuss, action github.GenericCommentEventAction, isPR bool, prNumber int, issueState string, repo *github.Repo, body string) error {
	return reviewer.HandleGenericComment(ghc, roc, log, &config, reviewer.RandomStrategy{}, action, isPR, prNumber, issueState, repo, body)
}

func handleStatusEvent(pc plugins.Agent, se github.StatusEvent) error {
	return reviewer.HandleStatusEvent(pc, &pc.PluginConfig.Blunderbuss, reviewer.RandomStrategy{}, se)
}

func handleStatus(ghc reviewer.GitHubClient, roc reviewer.RepoOwnersClient, log *logrus.Entry, config plugins.Blunderbuss, sha string, context string, state string, description string, repo *github.Repo) error {
	return reviewer.HandleStatus(ghc, roc, log, &config, reviewer.RandomStrategy{}, sha, context, state, description, repo)
}

func handle(ghc reviewer.GitHubClient, roc reviewer.RepoOwnersClient, log *logrus.Entry, reviewerCount *int, maxReviewers int, excludeApprovers bool, useStatusAvailability bool, repo *github.Repo, pr *github.PullRequest) error {
	return reviewer.Handle(ghc, roc, log, reviewer.RandomStrategy{}, reviewerCount, maxReviewers, excludeApprovers, useStatusAvailability, repo, pr)
}
