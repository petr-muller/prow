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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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

func TestRerunWithRetry(t *testing.T) {
	for _, tc := range []struct {
		name     string
		outcomes []string
		wantErr  string
		waits    []time.Duration
	}{
		{name: "success", outcomes: []string{"success"}},
		{name: "transport retries", outcomes: []string{"transport", "transport", "success"}, waits: []time.Duration{10 * time.Second, 20 * time.Second}},
		{name: "status retries", outcomes: []string{"status", "success"}, waits: []time.Duration{10 * time.Second}},
		{name: "exhausted status", outcomes: []string{"status", "status", "status"}, wantErr: "status not expected: 503", waits: []time.Duration{10 * time.Second, 20 * time.Second}},
		{name: "exhausted transport", outcomes: []string{"transport", "transport", "transport"}, wantErr: "could not make post request transport failure", waits: []time.Duration{10 * time.Second, 20 * time.Second}},
		{name: "response with error", outcomes: []string{"response error", "success"}, waits: []time.Duration{10 * time.Second}},
		{name: "body read error", outcomes: []string{"body"}, wantErr: "could not read body response read failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var bodies []*rerunTestBody
			checkClosed := func() {
				t.Helper()
				for i, body := range bodies {
					if body.closes != 1 {
						t.Fatalf("body %d closed %d times, want once", i, body.closes)
					}
				}
			}
			calls := 0
			req, err := http.NewRequest(http.MethodPost, "http://deck/rerun", nil)
			if err != nil {
				t.Fatal(err)
			}
			do := func(got *http.Request) (*http.Response, error) {
				checkClosed()
				if got != req {
					t.Fatal("request changed")
				}
				if calls >= len(tc.outcomes) {
					t.Fatal("unexpected additional retry")
				}
				outcome := tc.outcomes[calls]
				calls++
				if outcome == "transport" {
					return nil, errors.New("transport failure")
				}
				body := &rerunTestBody{Reader: strings.NewReader("rerun response"), readError: outcome == "body"}
				bodies = append(bodies, body)
				res := &http.Response{StatusCode: http.StatusOK, Body: body}
				if outcome == "status" {
					res.StatusCode = http.StatusServiceUnavailable
				}
				if outcome == "response error" {
					return res, errors.New("transport failure")
				}
				return res, nil
			}
			var waits []time.Duration
			body, err := rerunWithRetry(req, do, func(d time.Duration) {
				checkClosed()
				waits = append(waits, d)
			})
			checkClosed()
			if calls != len(tc.outcomes) {
				t.Errorf("requests = %d, want %d", calls, len(tc.outcomes))
			}
			if diff := cmp.Diff(tc.waits, waits); diff != "" {
				t.Errorf("waits (-want +got): %s", diff)
			}
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Errorf("error = %v, want %q", err, tc.wantErr)
				}
			} else if err != nil || string(body) != "rerun response" {
				t.Errorf("body = %q, error = %v", body, err)
			}
		})
	}
}

type rerunTestBody struct {
	io.Reader
	readError bool
	closes    int
}

func (b *rerunTestBody) Read(p []byte) (int, error) {
	if b.readError {
		return 0, errors.New("read failure")
	}
	return b.Reader.Read(p)
}

func (b *rerunTestBody) Close() error {
	b.closes++
	return nil
}
