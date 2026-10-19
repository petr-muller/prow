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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/yaml"
)

func TestSupplementalTideContextMergeConflictPreservesInputs(t *testing.T) {
	for _, field := range []string{"skip-unknown-contexts", "from-branch-protection", "overwrite-pending-contexts"} {
		for _, scope := range []struct {
			name, format, path string
		}{
			{"global", "%s: %t", "tide.context_options"},
			{"org", "orgs: {acme: {%s: %t}}", `tide.context_options.orgs["acme"]`},
			{"repo", "orgs: {acme: {repos: {widget: {%s: %t}}}}", `tide.context_options.orgs["acme"].repos["widget"]`},
			{"branch", "orgs: {acme: {repos: {widget: {branches: {main: {%s: %t}}}}}}", `tide.context_options.orgs["acme"].repos["widget"].branches["main"]`},
		} {
			for _, receiverValue := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/%t", field, scope.name, receiverValue), func(t *testing.T) {
					fixture := func(value bool, side string) TideContextPolicyOptions {
						options := supplementalContextSiblingFixture(side)
						var conflict TideContextPolicyOptions
						if err := yaml.Unmarshal([]byte(fmt.Sprintf(scope.format, field, value)), &conflict); err != nil {
							t.Fatal(err)
						}
						if err := options.mergeFrom(conflict); err != nil {
							t.Fatal(err)
						}
						return options
					}
					receiver := fixture(receiverValue, "existing")
					incoming := fixture(!receiverValue, "incoming")
					wantReceiver := fixture(receiverValue, "existing")
					wantIncoming := fixture(!receiverValue, "incoming")
					err := receiver.mergeFrom(incoming)
					wantError := scope.path + ": conflicting values for " + field
					if err == nil || err.Error() != wantError {
						t.Fatalf("expected error %q, got %v", wantError, err)
					}
					if diff := cmp.Diff(wantReceiver, receiver); diff != "" {
						t.Errorf("failed merge changed receiver (-want +got):\n%s", diff)
					}
					if diff := cmp.Diff(wantIncoming, incoming); diff != "" {
						t.Errorf("failed merge changed incoming policy (-want +got):\n%s", diff)
					}
				})
			}
		}
	}
}

// Each side contributes siblings at every map level, including nested maps in
// orgs and repos which occur on only one side of the merge.
func supplementalContextSiblingFixture(side string) TideContextPolicyOptions {
	policy := TideContextPolicy{RequiredContexts: []string{side}}
	skipUnknown, fromBranchProtection, overwritePending := true, false, true
	siblingPolicy := TideContextPolicy{
		SkipUnknownContexts: &skipUnknown, FromBranchProtection: &fromBranchProtection, OverwritePendingContexts: &overwritePending,
		RequiredContexts: []string{side}, RequiredIfPresentContexts: []string{side + "-conditional"},
		OptionalContexts: []string{side + "-optional"}, OptionalRegexContexts: []string{side + "-regex"},
	}
	repo := func() TideRepoContextPolicy {
		return TideRepoContextPolicy{
			TideContextPolicy: policy,
			Branches:          map[string]TideContextPolicy{"main": policy, side + "-branch": siblingPolicy},
		}
	}
	org := func() TideOrgContextPolicy {
		return TideOrgContextPolicy{
			TideContextPolicy: policy,
			Repos:             map[string]TideRepoContextPolicy{"widget": repo(), side + "-repo": repo()},
		}
	}
	return TideContextPolicyOptions{
		TideContextPolicy: policy,
		Orgs:              map[string]TideOrgContextPolicy{"acme": org(), side + "-org": org()},
	}
}

func TestSupplementalTideContextMergePreservesSiblingMaps(t *testing.T) {
	receiver := supplementalContextSiblingFixture("existing")
	previous := receiver // Retain the maps held by the receiver before merging.
	incoming := supplementalContextSiblingFixture("incoming")
	if err := receiver.mergeFrom(incoming); err != nil {
		t.Fatal(err)
	}

	want := supplementalContextSiblingFixture("existing")
	other := supplementalContextSiblingFixture("incoming")
	want.RequiredContexts = []string{"existing", "incoming"}
	want.Orgs["incoming-org"] = other.Orgs["incoming-org"]
	org := want.Orgs["acme"]
	org.RequiredContexts = []string{"existing", "incoming"}
	org.Repos["incoming-repo"] = other.Orgs["acme"].Repos["incoming-repo"]
	repo := org.Repos["widget"]
	repo.RequiredContexts = []string{"existing", "incoming"}
	repo.Branches["main"] = TideContextPolicy{RequiredContexts: []string{"existing", "incoming"}}
	repo.Branches["incoming-branch"] = other.Orgs["acme"].Repos["widget"].Branches["incoming-branch"]
	org.Repos["widget"] = repo
	want.Orgs["acme"] = org
	if diff := cmp.Diff(want, receiver); diff != "" {
		t.Fatalf("lost sibling policies (-want +got):\n%s", diff)
	}

	// Mutate every resulting map, including maps beneath one-sided orgs/repos.
	for orgName, org := range receiver.Orgs {
		for repoName, repo := range org.Repos {
			for branch := range repo.Branches {
				delete(repo.Branches, branch)
			}
			repo.Branches["new-branch"] = TideContextPolicy{}
			delete(org.Repos, repoName)
		}
		org.Repos["new-repo"] = TideRepoContextPolicy{}
		delete(receiver.Orgs, orgName)
	}
	receiver.Orgs["new-org"] = TideOrgContextPolicy{}
	if diff := cmp.Diff(supplementalContextSiblingFixture("existing"), previous); diff != "" {
		t.Errorf("result maps alias previous receiver maps (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(supplementalContextSiblingFixture("incoming"), incoming); diff != "" {
		t.Errorf("result maps alias incoming maps (-want +got):\n%s", diff)
	}
}

func TestSupplementalTideContextMergeOrderAndIdempotence(t *testing.T) {
	fields := []string{"required-contexts", "required-if-present-contexts", "optional-contexts", "optional-regex-contexts"}
	scopes := []string{"global", "org", "repo", "branch"}
	shard := func(id string) string {
		var out strings.Builder
		out.WriteString("tide:\n  context_options:\n")
		for depth, scope := range scopes {
			indent := strings.Repeat(" ", 4+depth*4)
			if depth > 0 {
				parentIndent := strings.Repeat(" ", depth*4)
				out.WriteString(parentIndent + []string{"", "orgs:\n      acme:\n", "repos:\n          widget:\n", "branches:\n              main:\n"}[depth])
			}
			value := depth%2 == 0
			fmt.Fprintf(&out, "%sskip-unknown-contexts: %t\n%sfrom-branch-protection: %t\n%soverwrite-pending-contexts: %t\n", indent, value, indent, !value, indent, value)
			for _, field := range fields {
				fmt.Fprintf(&out, "%s%s: [%s-%s-%s, %s-%s-shared, %s-%s-%s]\n", indent, field, scope, field, id, scope, field, scope, field, id)
			}
		}
		return out.String()
	}
	shards := []string{shard("c"), shard("a"), shard("b")}
	wantPolicy := func(first, last int) TideContextPolicy {
		value, opposite := last%2 == 0, last%2 != 0
		policy := TideContextPolicy{SkipUnknownContexts: &value, FromBranchProtection: &opposite, OverwritePendingContexts: &value}
		lists := []*[]string{&policy.RequiredContexts, &policy.RequiredIfPresentContexts, &policy.OptionalContexts, &policy.OptionalRegexContexts}
		for i, field := range fields {
			for _, scope := range scopes[first : last+1] {
				for _, suffix := range []string{"a", "b", "c", "shared"} {
					*lists[i] = append(*lists[i], scope+"-"+field+"-"+suffix)
				}
			}
			sort.Strings(*lists[i])
		}
		return policy
	}
	for _, order := range [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			for _, repeat := range []bool{false, true} {
				t.Run(fmt.Sprintf("repeat=%t", repeat), func(t *testing.T) {
					supplemental := []string{shards[order[1]], shards[order[2]]}
					if repeat {
						supplemental = []string{"tide: {}", shards[order[1]], "tide:\n  context_options: {}", shards[order[0]], shards[order[2]], shards[order[1]], "tide: {}"}
					}
					cfg, err := loadConfigYaml(shards[order[0]], t, supplemental...)
					if err != nil {
						t.Fatal(err)
					}
					options := cfg.Tide.ContextOptions
					policies := []TideContextPolicy{options.TideContextPolicy, options.Orgs["acme"].TideContextPolicy, options.Orgs["acme"].Repos["widget"].TideContextPolicy, options.Orgs["acme"].Repos["widget"].Branches["main"]}
					for depth, policy := range policies {
						if diff := cmp.Diff(wantPolicy(depth, depth), policy); diff != "" {
							t.Errorf("%s shard union (-want +got):\n%s", scopes[depth], diff)
						}
					}
					for depth, target := range [][3]string{{"other", "other", "other"}, {"acme", "other", "other"}, {"acme", "widget", "other"}, {"acme", "widget", "main"}} {
						policy := ParseTideContextPolicyOptions(target[0], target[1], target[2], options)
						if diff := cmp.Diff(wantPolicy(0, depth), policy); diff != "" {
							t.Errorf("%s effective policy (-want +got):\n%s", scopes[depth], diff)
						}
					}
				})
			}
		})
	}
}

func TestLoadSupplementalTideContextConflictSources(t *testing.T) {
	for _, scope := range []struct {
		name, format, path string
	}{
		{"global", "%s: %t", "tide.context_options"},
		{"org", "orgs: {acme: {%s: %t}}", `tide.context_options.orgs["acme"]`},
		{"repo", "orgs: {acme: {repos: {widget: {%s: %t}}}}", `tide.context_options.orgs["acme"].repos["widget"]`},
		{"branch", "orgs: {acme: {repos: {widget: {branches: {main: {%s: %t}}}}}}", `tide.context_options.orgs["acme"].repos["widget"].branches["main"]`},
	} {
		for _, field := range []string{"skip-unknown-contexts", "from-branch-protection", "overwrite-pending-contexts"} {
			for _, scenario := range []string{"main-to-shard", "shard-to-shard", "repeated-identical"} {
				t.Run(scope.name+"/"+field+"/"+scenario, func(t *testing.T) {
					root := t.TempDir()
					firstDir, secondDir := filepath.Join(root, "first"), filepath.Join(root, "second")
					for _, dir := range []string{firstDir, secondDir} {
						if err := os.Mkdir(dir, 0700); err != nil {
							t.Fatal(err)
						}
					}
					write := func(path, content string) {
						t.Helper()
						if err := os.WriteFile(path, []byte(content), 0600); err != nil {
							t.Fatal(err)
						}
					}
					policy := func(value bool) string {
						return "tide: {context_options: {" + fmt.Sprintf(scope.format, field, value) + "}}"
					}
					mainPath := filepath.Join(root, "main.yaml")
					firstPath := filepath.Join(firstDir, "01-first.yaml")
					conflictPath := filepath.Join(secondDir, "03-conflict.yaml")
					// A different field at the exact scope must not own the target field.
					otherField := "skip-unknown-contexts"
					if field == otherField {
						otherField = "from-branch-protection"
					}
					write(mainPath, "tide: {context_options: {"+fmt.Sprintf(scope.format, otherField, false)+"}}")
					if scenario == "main-to-shard" {
						firstPath = mainPath
					} else {
						// The same field at a different scope is a valid inherited override.
						otherScope := "%s: %t"
						if scope.name == "global" {
							otherScope = "orgs: {other: {%s: %t}}"
						}
						write(filepath.Join(firstDir, "00-other-scope.yaml"), "tide: {context_options: {"+fmt.Sprintf(otherScope, field, false)+"}}")
					}
					write(firstPath, policy(true))
					if scenario == "repeated-identical" {
						write(filepath.Join(firstDir, "02-identical.yaml"), policy(true))
						write(filepath.Join(secondDir, "02-identical.yaml"), policy(true))
					}
					write(conflictPath, policy(false))
					_, err := loadConfig(mainPath, "", []string{firstDir, secondDir}, ".yaml")
					if err == nil {
						t.Fatal("expected conflicting boolean to fail loading")
					}
					for _, want := range []string{
						"failed to merge in config from " + conflictPath + ":",
						scope.path + ": conflicting values for " + field,
						"previously set in " + firstPath + ")",
					} {
						if !strings.Contains(err.Error(), want) {
							t.Errorf("error %q does not contain %q", err, want)
						}
					}
				})
			}
		}
	}
}

// Other config types can fail after context options have merged successfully.
// The loader keeps walking the directory and must retain that accepted source.
func TestLoadSupplementalTideContextSourceAfterUnrelatedMergeError(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.yaml")
	shardDir := filepath.Join(root, "shards")
	if err := os.Mkdir(shardDir, 0700); err != nil {
		t.Fatal(err)
	}
	firstPath := filepath.Join(shardDir, "01-first.yaml")
	conflictPath := filepath.Join(shardDir, "02-conflict.yaml")
	for path, content := range map[string]string{
		mainPath:     "{}",
		firstPath:    "config_version_sha: invalid-supplemental-field\ntide: {context_options: {skip-unknown-contexts: true}}",
		conflictPath: "tide: {context_options: {skip-unknown-contexts: false}}",
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, err := loadConfig(mainPath, "", []string{shardDir}, ".yaml")
	if err == nil {
		t.Fatal("expected load failure")
	}
	for _, want := range []string{
		"all other fields have no merging logic yet",
		"failed to merge in config from " + conflictPath + ":",
		"tide.context_options: conflicting values for skip-unknown-contexts (previously set in " + firstPath + ")",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}
