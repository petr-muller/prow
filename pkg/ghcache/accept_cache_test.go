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

package ghcache

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cjwagner/httpcache"
)

type acceptCacheUpstream struct {
	t        *testing.T
	hitsLock sync.Mutex
	hits     map[string]int
	arrived  chan string
	release  chan struct{}
}

func (upstream *acceptCacheUpstream) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.String() != "https://api.github.com/repos/org/repo/pulls/1" {
		upstream.t.Errorf("upstream request used URL %q", req.URL.String())
	}
	accept := strings.Join(req.Header.Values("Accept"), ", ")
	upstream.hitsLock.Lock()
	upstream.hits[accept]++
	upstream.hitsLock.Unlock()

	if upstream.arrived != nil {
		select {
		case upstream.arrived <- accept:
		case <-time.After(10 * time.Second):
			upstream.t.Errorf("timed out reporting upstream request for Accept %q", accept)
			return nil, errors.New("timed out reporting upstream request")
		}
		<-upstream.release
	}

	etag := `"` + accept + `"`
	header := http.Header{}
	header.Set("ETag", etag)
	if req.Header.Get("If-None-Match") != "" {
		if req.Header.Get("If-None-Match") != etag {
			upstream.t.Errorf("request for Accept %q sent ETag %q, wanted %q", accept, req.Header.Get("If-None-Match"), etag)
		}
		return &http.Response{
			StatusCode: http.StatusNotModified,
			Header:     header,
			Body:       http.NoBody,
		}, nil
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       io.NopCloser(bytes.NewBufferString(accept)),
	}, nil
}

func TestNewFromCacheKeysGetRequestsByAccept(t *testing.T) {
	t.Parallel()

	accepts := []string{
		"application/vnd.github.v3+json",
		"application/vnd.github.v3.diff",
		"application/vnd.github.patch",
	}
	upstream := &acceptCacheUpstream{
		t:    t,
		hits: make(map[string]int),
	}
	partitionsLock := sync.Mutex{}
	partitions := make(map[string]int)
	roundTripper := NewFromCache(upstream,
		func(partitionKey string, _ *time.Time) httpcache.Cache {
			partitionsLock.Lock()
			defer partitionsLock.Unlock()
			partitions[partitionKey]++
			return httpcache.NewMemoryCache()
		},
		len(accepts),
		RequestThrottlingTimes{},
	)

	request := func(accept string) string {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/org/repo/pulls/1", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "same-token")
		req.Header.Set("Accept", accept)
		resp, err := roundTripper.RoundTrip(req)
		if err != nil {
			t.Errorf("request for Accept %q failed: %v", accept, err)
			return ""
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("failed to read response for Accept %q: %v", accept, err)
			return ""
		}
		if string(body) != accept {
			t.Errorf("request for Accept %q returned body %q", accept, string(body))
		}
		return resp.Header.Get(CacheModeHeader)
	}

	for _, accept := range accepts {
		if mode := request(accept); mode != string(ModeMiss) {
			t.Errorf("initial request for Accept %q had cache mode %q, wanted %q", accept, mode, ModeMiss)
		}
	}
	for _, accept := range accepts {
		if mode := request(accept); mode != string(ModeRevalidated) {
			t.Errorf("sequential revalidation for Accept %q had cache mode %q, wanted %q", accept, mode, ModeRevalidated)
		}
	}

	upstream.arrived = make(chan string, len(accepts))
	upstream.release = make(chan struct{})
	results := make([]string, len(accepts))
	wg := sync.WaitGroup{}
	for index, accept := range accepts {
		wg.Go(func() {
			results[index] = request(accept)
		})
	}
	arrived := make([]string, 0, len(accepts))
	for range accepts {
		select {
		case accept := <-upstream.arrived:
			arrived = append(arrived, accept)
		case <-time.After(10 * time.Second):
			t.Errorf("timed out waiting for overlapping revalidation; arrived: %v", arrived)
		}
	}
	close(upstream.release)
	wg.Wait()

	arrivedCounts := make(map[string]int, len(accepts))
	for _, accept := range arrived {
		arrivedCounts[accept]++
	}
	expectedArrivals := make(map[string]int, len(accepts))
	for _, accept := range accepts {
		expectedArrivals[accept] = 1
	}
	if !reflect.DeepEqual(arrivedCounts, expectedArrivals) {
		t.Errorf("overlapping upstream requests arrived as %v, wanted one request for each of %v", arrivedCounts, accepts)
	}
	for index, accept := range accepts {
		if results[index] != string(ModeRevalidated) {
			t.Errorf("overlapping revalidation for Accept %q had cache mode %q, wanted %q", accept, results[index], ModeRevalidated)
		}
	}
	expectedHits := make(map[string]int, len(accepts))
	for _, accept := range accepts {
		expectedHits[accept] = 3
	}
	if !reflect.DeepEqual(upstream.hits, expectedHits) {
		t.Errorf("unexpected upstream hits: %v", upstream.hits)
	}
	expectedPartition := map[string]int{getCachePartition(&http.Request{Header: http.Header{"Authorization": {"same-token"}}}): 1}
	if !reflect.DeepEqual(partitions, expectedPartition) {
		t.Errorf("unexpected cache partitions: %v", partitions)
	}
}
