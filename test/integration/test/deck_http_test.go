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

package integration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

func TestDeckHTTPScraperCancellation(t *testing.T) {
	for _, stage := range []string{"headers", "body"} {
		t.Run(stage, func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if stage == "body" {
					w.Header().Set("Content-Length", "100")
					fmt.Fprint(w, "partial body")
					w.(http.Flusher).Flush()
				}
				close(started)
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer server.Close()
			defer close(release)

			var body []byte
			done := make(chan error, 1)
			go func() {
				done <- wait.PollUntilContextTimeout(context.Background(), time.Millisecond, time.Second, true, deckHTTPScraper(t, server.URL, &body))
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("request did not reach server")
			}
			select {
			case err := <-done:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected poll deadline, got %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("poll deadline did not cancel stalled " + stage)
			}
			if len(body) != 0 {
				t.Fatalf("stored incomplete response body: %q", body)
			}
		})
	}
}

func TestDeckHTTPScraperRetries(t *testing.T) {
	for _, failure := range []string{"request", "status", "body"} {
		t.Run(failure, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) == 1 {
					switch failure {
					case "request":
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Errorf("hijack: %v", err)
							return
						}
						conn.Close()
					case "status":
						w.WriteHeader(http.StatusServiceUnavailable)
					case "body":
						w.Header().Set("Content-Length", "100")
						fmt.Fprint(w, "truncated")
					}
					return
				}
				fmt.Fprint(w, "<title>Prow Status</title>")
			}))
			defer server.Close()
			var body []byte
			err := wait.PollUntilContextTimeout(context.Background(), time.Millisecond, 5*time.Second, true, deckHTTPScraper(t, server.URL, &body))
			if err != nil {
				t.Fatalf("poll failed after transient %s failure: %v", failure, err)
			}
			if got := requests.Load(); got != 2 {
				t.Errorf("expected two requests, got %d", got)
			}
			if string(body) != "<title>Prow Status</title>" {
				t.Errorf("unexpected response body: %q", body)
			}
		})
	}
}
