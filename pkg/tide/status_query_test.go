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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	githubql "github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/prow/pkg/config"
)

func TestStatusSearchHeadRollup(t *testing.T) {
	for _, tc := range []struct {
		name         string
		rollup       string
		wantFallback bool
	}{
		{
			name: "head rollup",
			rollup: `{"commit":{"oid":"head","status":{"contexts":[
				{"context":"tide","state":"PENDING","description":"Not mergeable. Needs lgtm label."}]}},
				"contexts":{"nodes":[{"name":"ci/check","conclusion":"FAILURE","status":"COMPLETED"},{}]}}`,
		},
		{name: "missing rollup", rollup: `null`, wantFallback: true},
		{name: "missing commit", rollup: `{"commit":null,"contexts":{"nodes":[]}}`, wantFallback: true},
		{
			name: "rollup for a different head",
			rollup: `{"commit":{"oid":"old-head","status":{"contexts":[
				{"context":"wrong-commit","state":"SUCCESS"}]}},"contexts":{"nodes":[]}}`,
			wantFallback: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var statusCalls, checkCalls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case http.MethodPost:
					var req struct{ Query string }
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if strings.Contains(req.Query, "commits(") {
						t.Error("status search still fetches PR commit history")
					}
					for _, field := range []string{"statusCheckRollup", "mergeable", "mergeStateStatus", "canBeRebased", "reviewDecision", "authorMetadata:author", "contexts(last: 100)"} {
						if !strings.Contains(req.Query, field) {
							t.Errorf("status query is missing %s", field)
						}
					}
					fmt.Fprintf(w, `{"data":{"search":{"pageInfo":{"hasNextPage":false},"nodes":[{
						"number":1,"headRefOid":"head","headRefName":"feature","baseRef":{"name":"main","prefix":"refs/heads/"},
						"repository":{"name":"repo","nameWithOwner":"org/repo","owner":{"login":"org"}},
						"author":{"login":"bot"},"authorMetadata":{"__typename":"Bot"},
						"mergeable":"CONFLICTING","mergeStateStatus":"BLOCKED","canBeRebased":true,
						"reviewDecision":"APPROVED","labels":{"nodes":[{"name":"approved"}]},"milestone":{"title":"v1"},
						"body":"body","title":"title","updatedAt":"2026-10-05T22:30:00Z","statusCheckRollup":%s
					}]}}}`, tc.rollup)
				case http.MethodGet:
					switch r.URL.Path {
					case "/repos/org/repo/commits/head/status":
						statusCalls++
						fmt.Fprint(w, `{"statuses":[{"context":"tide","state":"pending","description":"Not mergeable. Needs lgtm label."}]}`)
					case "/repos/org/repo/commits/head/check-runs":
						checkCalls++
						fmt.Fprint(w, `{"check_runs":[{"name":"ci/check","conclusion":"failure","status":"completed"}]}`)
					default:
						t.Errorf("unexpected fallback URL %s", r.URL)
						w.WriteHeader(http.StatusNotFound)
					}
				default:
					t.Errorf("unexpected method %s", r.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))
			defer server.Close()
			ghc := newTestGitHubClient(t, server)
			log := logrus.WithField("test", t.Name())
			provider := &GitHubProvider{ghc: ghc, logger: log}
			sc := &statusController{
				ghc: ghc, ghProvider: provider, logger: log,
				config: func() *config.Config {
					return &config.Config{ProwConfig: config.ProwConfig{Tide: config.Tide{
						TideGitHubConfig: config.TideGitHubConfig{Queries: config.TideQueries{{Repos: []string{"org/repo"}}}},
					}}}
				},
			}
			prs := sc.search()
			if len(prs) != 1 {
				t.Fatalf("got %d PRs, want 1", len(prs))
			}
			pr := &prs[0]
			if pr.Org != "org" || pr.Repo != "repo" || pr.HeadRefOID != "head" || pr.BaseRefName != "main" || pr.HeadRefName != "feature" || pr.AuthorLogin != "bot" || pr.Title != "title" || pr.Body != "body" {
				t.Errorf("PR metadata changed: %+v", pr)
			}
			if pr.GitHub.Mergeable != githubql.MergeableStateConflicting || pr.GitHub.MergeStateStatus != MergeStateStatusBlocked || !pr.GitHub.CanBeRebased || pr.GitHub.ReviewDecision != githubql.PullRequestReviewDecisionApproved || pr.GitHub.AuthorMetadata.TypeName != "Bot" || pr.GitHub.Milestone.Title != "v1" || pr.GitHub.Labels.Nodes[0].Name != "approved" {
				t.Errorf("status decision fields changed: %+v", pr.GitHub)
			}
			contexts, err := provider.headContexts(pr)
			if err != nil {
				t.Fatal(err)
			}
			want := []Context{
				{Context: "tide", State: githubql.StatusStatePending, Description: "Not mergeable. Needs lgtm label."},
				{Context: "ci/check", State: githubql.StatusStateFailure},
			}
			if diff := cmp.Diff(want, contexts); diff != "" {
				t.Errorf("head contexts differ (-want +got):\n%s", diff)
			}
			// Fallback data must also be available to requirementDiff, which reads
			// the PR's commits directly. A second lookup should use the cached head.
			if _, err := provider.headContexts(pr); err != nil {
				t.Fatal(err)
			}
			desc, _ := requirementDiff(pr.GitHub, &config.TideQuery{Labels: []string{"approved"}}, &config.TideContextPolicy{OptionalContexts: []string{"tide"}}, config.GitHubMergeBlocksIgnore)
			if desc != " Job ci/check has not succeeded." {
				t.Errorf("unexpected status explanation: %q", desc)
			}
			wantCalls := 0
			if tc.wantFallback {
				wantCalls = 1
			}
			if statusCalls != wantCalls || checkCalls != wantCalls {
				t.Errorf("fallback calls: statuses=%d, checks=%d, want %d each", statusCalls, checkCalls, wantCalls)
			}
		})
	}
}

func TestStatusSearchPagination(t *testing.T) {
	for _, tc := range []struct {
		name          string
		persistent    bool
		wantSizes     []int
		wantPRs       []int
		wantWatermark string
	}{
		{
			name: "recover second page", wantSizes: []int{37, 37, 18}, wantPRs: []int{1, 2},
			wantWatermark: "2026-10-05T22:30:30Z",
		},
		{
			name: "retain first page on GraphQL failure", persistent: true,
			wantSizes: []int{37, 37}, wantPRs: []int{1},
			wantWatermark: "2026-10-05T22:29:30Z",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sizes []int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Variables struct {
						SearchCursor   *string `json:"searchCursor"`
						SearchPageSize int     `json:"searchPageSize"`
					} `json:"variables"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				sizes = append(sizes, req.Variables.SearchPageSize)
				w.Header().Set("Content-Type", "application/json")
				if len(sizes) == 1 {
					if req.Variables.SearchCursor != nil {
						t.Error("first request has a cursor")
					}
					fmt.Fprint(w, `{"data":{"search":{"pageInfo":{"hasNextPage":true,"endCursor":"one"},"nodes":[{"number":1,"updatedAt":"2026-10-05T22:30:00Z"}]}}}`)
					return
				}
				if req.Variables.SearchCursor == nil || *req.Variables.SearchCursor != "one" {
					t.Errorf("second-page cursor was not preserved: %v", req.Variables.SearchCursor)
				}
				if tc.persistent {
					// Incomplete data must not enter the results or move the saved timestamp.
					fmt.Fprint(w, `{"data":{"search":{"nodes":[{"number":999,"updatedAt":"2030-01-01T00:00:00Z"}]}},"errors":[{"message":"injected query failure"}]}`)
					return
				}
				if req.Variables.SearchPageSize > 18 {
					w.WriteHeader(http.StatusBadGateway)
					fmt.Fprint(w, nginxErrorPage(http.StatusBadGateway))
					return
				}
				fmt.Fprint(w, `{"data":{"search":{"pageInfo":{"hasNextPage":false},"nodes":[{"number":2,"updatedAt":"2026-10-05T22:31:00Z"}]}}}`)
			}))
			defer server.Close()
			ghc := newTestGitHubClient(t, server)
			log := logrus.WithField("test", t.Name())
			sc := &statusController{
				ghc: ghc, ghProvider: &GitHubProvider{ghc: ghc}, logger: log,
				config: func() *config.Config {
					return &config.Config{ProwConfig: config.ProwConfig{Tide: config.Tide{
						TideGitHubConfig: config.TideGitHubConfig{Queries: config.TideQueries{{Repos: []string{"org/repo"}}}},
					}}}
				},
			}
			var numbers []int
			for _, pr := range sc.search() {
				numbers = append(numbers, pr.Number)
			}
			if diff := cmp.Diff(tc.wantPRs, numbers); diff != "" {
				t.Errorf("unexpected PRs (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantSizes, sizes); diff != "" {
				t.Errorf("unexpected page sizes (-want +got):\n%s", diff)
			}
			if got := sc.storedState[""].LatestPR.Format(time.RFC3339); got != tc.wantWatermark {
				t.Errorf("saved timestamp = %s, want %s", got, tc.wantWatermark)
			}
		})
	}
}
