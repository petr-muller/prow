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

package tide

import (
	"context"

	githubql "github.com/shurcooL/githubv4"
)

// statusPullRequest omits the PR body and title and fetches status contexts
// and check runs only for the PR's head commit.
type statusPullRequest struct {
	Number githubql.Int
	Author struct {
		Login githubql.String
	}
	AuthorMetadata struct {
		TypeName githubql.String `graphql:"__typename"`
	} `graphql:"authorMetadata:author"`
	BaseRef struct {
		Name   githubql.String
		Prefix githubql.String
	}
	HeadRefName      githubql.String `graphql:"headRefName"`
	HeadRefOID       githubql.String `graphql:"headRefOid"`
	Mergeable        githubql.MergeableState
	Merged           githubql.Boolean `graphql:"merged"`
	MergeStateStatus githubql.String  `graphql:"mergeStateStatus"`
	CanBeRebased     githubql.Boolean `graphql:"canBeRebased"`
	Repository       struct {
		Name          githubql.String
		NameWithOwner githubql.String
		Owner         struct {
			Login githubql.String
		}
	}
	ReviewDecision    githubql.PullRequestReviewDecision `graphql:"reviewDecision"`
	StatusCheckRollup *statusHeadRollup
	Labels            Labels `graphql:"labels(first: 100)"`
	Milestone         *Milestone
	UpdatedAt         githubql.DateTime
}

type statusHeadRollup struct {
	Commit   *statusHeadCommit
	Contexts StatusCheckRollupContext `graphql:"contexts(last: 100)"`
}

type statusHeadCommit struct {
	OID    githubql.String `graphql:"oid"`
	Status CommitStatus
}

func (pr statusPullRequest) pullRequest() PullRequest {
	result := PullRequest{
		Number:           pr.Number,
		Author:           pr.Author,
		AuthorMetadata:   pr.AuthorMetadata,
		BaseRef:          pr.BaseRef,
		HeadRefName:      pr.HeadRefName,
		HeadRefOID:       pr.HeadRefOID,
		Mergeable:        pr.Mergeable,
		Merged:           pr.Merged,
		MergeStateStatus: pr.MergeStateStatus,
		CanBeRebased:     pr.CanBeRebased,
		Repository:       pr.Repository,
		ReviewDecision:   pr.ReviewDecision,
		Labels:           pr.Labels,
		Milestone:        pr.Milestone,
		UpdatedAt:        pr.UpdatedAt,
	}
	// Missing or stale rollup data must use headContexts' existing fallback,
	// which retrieves statuses and check runs for HeadRefOID explicitly.
	if rollup := pr.StatusCheckRollup; rollup != nil && rollup.Commit != nil && rollup.Commit.OID == pr.HeadRefOID {
		result.Commits.Nodes = []struct{ Commit Commit }{{Commit: Commit{
			OID:               rollup.Commit.OID,
			Status:            rollup.Commit.Status,
			StatusCheckRollup: StatusCheckRollup{Contexts: rollup.Contexts},
		}}}
	}
	return result
}

type statusSearchQuery struct {
	RateLimit struct {
		Cost      githubql.Int
		Remaining githubql.Int
	}
	Search struct {
		PageInfo struct {
			HasNextPage githubql.Boolean
			EndCursor   githubql.String
		}
		Nodes []struct {
			PullRequest statusPullRequest `graphql:"... on PullRequest"`
		}
	} `graphql:"search(type: ISSUE, first: $searchPageSize, after: $searchCursor, query: $query)"`
}

// queryStatusPRs adapts the head-only query to search's common pagination and
// recovery loop. Only complete pages are converted to its searchQuery result.
func (gi *GitHubProvider) queryStatusPRs(ctx context.Context, result interface{}, vars map[string]interface{}, org string) error {
	var status statusSearchQuery
	if err := gi.ghc.QueryWithGitHubAppsSupport(ctx, &status, vars, org); err != nil {
		return err
	}
	sq := result.(*searchQuery)
	sq.RateLimit = status.RateLimit
	sq.Search.PageInfo = status.Search.PageInfo
	for _, node := range status.Search.Nodes {
		sq.Search.Nodes = append(sq.Search.Nodes, PRNode{PullRequest: node.PullRequest.pullRequest()})
	}
	return nil
}
