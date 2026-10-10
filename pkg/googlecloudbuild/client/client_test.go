/*
Copyright 2021 The Kubernetes Authors.

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

package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1/v2"
	"cloud.google.com/go/cloudbuild/apiv1/v2/cloudbuildpb"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"sigs.k8s.io/prow/pkg/testutil"
)

func TestListBuildsByTag(t *testing.T) {
	tests := []struct {
		name      string
		pages     []string
		wantIDs   []string
		wantError bool
	}{
		{
			name:  "empty",
			pages: []string{`{}`},
		},
		{
			name:    "single page",
			pages:   []string{`{"builds":[{"id":"first"},{"id":"second"}]}`},
			wantIDs: []string{"first", "second"},
		},
		{
			name: "multiple pages including an empty page",
			pages: []string{
				`{"builds":[{"id":"first"},{"id":"second"}],"nextPageToken":"1"}`,
				`{"nextPageToken":"2"}`,
				`{"builds":[{"id":"third"}]}`,
			},
			wantIDs: []string{"first", "second", "third"},
		},
		{
			name:      "first page error",
			pages:     []string{""},
			wantError: true,
		},
		{
			name: "later page error discards partial results",
			pages: []string{
				`{"builds":[{"id":"first"}],"nextPageToken":"1"}`,
				"",
			},
			wantError: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/v1/projects/test-project/builds" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				query := r.URL.Query()
				if got := query.Get("pageSize"); got != "50" {
					t.Errorf("pageSize = %q, want 50", got)
				}
				if got := query.Get("filter"); got != "tags=first-tag AND tags=second-tag" {
					t.Errorf("filter = %q, want tags=first-tag AND tags=second-tag", got)
				}
				wantToken := ""
				if requests > 0 {
					wantToken = strconv.Itoa(requests)
				}
				if got := query.Get("pageToken"); got != wantToken {
					t.Errorf("pageToken = %q, want %q", got, wantToken)
				}
				if requests >= len(tc.pages) {
					t.Error("unexpected additional page request")
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				page := tc.pages[requests]
				requests++
				if page == "" {
					http.Error(w, "listing failed", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(page))
			}))
			defer server.Close()
			interactor, err := cloudbuild.NewRESTClient(context.Background(), option.WithEndpoint(server.URL), option.WithoutAuthentication())
			if err != nil {
				t.Fatalf("create Cloud Build client: %v", err)
			}
			defer interactor.Close()
			client := &Client{interactor: interactor}
			builds, err := client.ListBuildsByTag(context.Background(), "test-project", []string{"first-tag", "second-tag"})
			if tc.wantError {
				var apiError *googleapi.Error
				if !errors.As(err, &apiError) || apiError.Code != http.StatusBadRequest {
					t.Fatalf("error = %v, want Cloud Build HTTP 400 error", err)
				}
				if builds != nil {
					t.Errorf("builds = %v, want nil on error", builds)
				}
			} else if err != nil {
				t.Fatalf("ListBuildsByTag: %v", err)
			}
			var gotIDs []string
			for _, build := range builds {
				gotIDs = append(gotIDs, build.Id)
			}
			if diff := cmp.Diff(tc.wantIDs, gotIDs); diff != "" {
				t.Errorf("build IDs (-want +got):\n%s", diff)
			}
			if requests != len(tc.pages) {
				t.Errorf("requests = %d, want %d", requests, len(tc.pages))
			}
		})
	}
}

func TestNewClientAcceptsAuthorizedUserCredentials(t *testing.T) {
	client, err := NewClient(context.Background(), testutil.WriteAuthorizedUserCredentialsFile(t))
	if err != nil {
		t.Fatalf("NewClient() returned an error for authorized_user credentials: %v", err)
	}
	if err := client.interactor.Close(); err != nil {
		t.Errorf("close Cloud Build client: %v", err)
	}
}

func TestProwLabel(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  string
		want string
	}{
		{
			name: "empty",
			want: " ::: ",
		},
		{
			name: "empty-key",
			val:  "aaa",
			want: " ::: aaa",
		},
		{
			name: "empty-val",
			key:  "aaa",
			want: "aaa ::: ",
		},
		{
			name: "normal",
			key:  "aaa",
			val:  "aaa",
			want: "aaa ::: aaa",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := ProwLabel(tc.key, tc.val), tc.want; got != want {
				t.Errorf("want: %s, got: %s", want, got)
			}
		})
	}
}

func TestKvPairFromProwLabel(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want []string
	}{
		{
			name: "empty",
			tag:  " ::: ",
			want: []string{"", ""},
		},
		{
			name: "empty-key",
			tag:  " ::: aaa",
			want: []string{"", "aaa"},
		},
		{
			name: "empty-val",
			tag:  "aaa ::: ",
			want: []string{"aaa", ""},
		},
		{
			name: "normal",
			tag:  "aaa ::: aaa",
			want: []string{"aaa", "aaa"},
		},
		{
			name: "invalid-separator",
			tag:  "aaa ;;; aaa",
			want: []string{"aaa ;;; aaa", ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, gotVal := KvPairFromProwLabel(tc.tag)
			wantKey, wantVal := tc.want[0], tc.want[1]
			if gotKey != wantKey {
				t.Errorf("key mismatching. want: %s, got: %s", wantKey, gotKey)
			}
			if gotVal != wantVal {
				t.Errorf("val mismatching. want: %s, got: %s", wantVal, gotVal)
			}
		})
	}
}

func TestGetProwLabel(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want map[string]string
	}{
		{
			name: "single-label",
			tags: []string{"aaa ::: aaa"},
			want: map[string]string{"aaa": "aaa"},
		},
		{
			name: "multiple-label",
			tags: []string{"aaa ::: aaa", "bbb ::: bbb"},
			want: map[string]string{"aaa": "aaa", "bbb": "bbb"},
		},
		{
			name: "no-label",
			tags: []string{},
			want: map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bld := &cloudbuildpb.Build{
				Tags: tc.tags,
			}
			got := GetProwLabels(bld)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("got(+), want(-):\n%s", diff)
			}
		})
	}
}
