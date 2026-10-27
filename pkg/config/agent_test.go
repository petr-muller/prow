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
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/util/sets"
)

func cfg(sha string) *Config {
	return &Config{ProwConfig: ProwConfig{ConfigVersionSHA: sha}}
}

func TestSetCoalescesForBusySubscriber(t *testing.T) {
	agent := &Agent{}
	sub := agent.Subscribe()

	// Leave updates unread to exercise coalescing.
	agent.Set(cfg("v1"))
	agent.Set(cfg("v2"))
	agent.Set(cfg("v3"))

	var got Delta
	select {
	case got = <-sub:
	default:
		t.Fatal("expected a coalesced delta in the buffer, found none")
	}
	if got.Before.ConfigVersionSHA != "" {
		t.Errorf("Before = %q, want %q (the initial config, not skipped)", got.Before.ConfigVersionSHA, "")
	}
	if got.After.ConfigVersionSHA != "v3" {
		t.Errorf("After = %q, want %q (the newest config)", got.After.ConfigVersionSHA, "v3")
	}

	select {
	case extra := <-sub:
		t.Fatalf("unexpected second delta: before=%q after=%q", extra.Before.ConfigVersionSHA, extra.After.ConfigVersionSHA)
	default:
	}
}

func TestSetDeliversChainedDeltasWhenDrained(t *testing.T) {
	agent := &Agent{}
	sub := agent.Subscribe()

	for _, want := range []struct{ before, after string }{
		{"", "v1"},
		{"v1", "v2"},
		{"v2", "v3"},
	} {
		agent.Set(cfg(want.after))
		got := receiveDelta(t, sub) // Drain before the next Set to prevent coalescing.
		if got.Before.ConfigVersionSHA != want.before || got.After.ConfigVersionSHA != want.after {
			t.Errorf("delta = {%q -> %q}, want {%q -> %q}", got.Before.ConfigVersionSHA, got.After.ConfigVersionSHA, want.before, want.after)
		}
	}
}

func TestSetPreservesDeltaChainDuringConcurrentReceive(t *testing.T) {
	agent := &Agent{}
	agent.Set(cfg("0"))
	sub := agent.Subscribe()

	for i := range 1000 {
		before := strconv.Itoa(2 * i)
		pending := strconv.Itoa(2*i + 1)
		after := strconv.Itoa(2*i + 2)
		agent.Set(cfg(pending))

		// Race a receive against replacement of a full buffer.
		start := make(chan struct{})
		received := make(chan Delta, 1)
		sent := make(chan struct{})
		go func() {
			<-start
			received <- <-sub
		}()
		go func() {
			<-start
			agent.Set(cfg(after))
			close(sent)
		}()
		close(start)

		got := receiveDelta(t, received)
		if got.Before.ConfigVersionSHA != before {
			t.Fatalf("iteration %d: Before = %q, want %q", i, got.Before.ConfigVersionSHA, before)
		}
		switch got.After.ConfigVersionSHA {
		case pending:
			got = receiveDelta(t, sub)
			if got.Before.ConfigVersionSHA != pending || got.After.ConfigVersionSHA != after {
				t.Fatalf("iteration %d: delta = {%q -> %q}, want {%q -> %q}", i, got.Before.ConfigVersionSHA, got.After.ConfigVersionSHA, pending, after)
			}
		case after:
		default:
			t.Fatalf("iteration %d: After = %q, want %q or %q", i, got.After.ConfigVersionSHA, pending, after)
		}
		<-sent
	}

	select {
	case extra := <-sub:
		t.Fatalf("unexpected extra delta: before=%q after=%q", extra.Before.ConfigVersionSHA, extra.After.ConfigVersionSHA)
	default:
	}
}

func TestSetCoalescesSubscribersIndependently(t *testing.T) {
	agent := &Agent{}
	agent.Set(cfg("0"))
	fast := agent.Subscribe()
	slow := agent.Subscribe()
	slowBefore := "0"

	for i := 1; i <= 6; i++ {
		after := strconv.Itoa(i)
		agent.Set(cfg(after))
		got := receiveDelta(t, fast)
		before := strconv.Itoa(i - 1)
		if got.Before.ConfigVersionSHA != before || got.After.ConfigVersionSHA != after {
			t.Fatalf("fast subscriber: delta = {%q -> %q}, want {%q -> %q}", got.Before.ConfigVersionSHA, got.After.ConfigVersionSHA, before, after)
		}

		// Let the slow subscriber catch up without another Set.
		if i%3 == 0 {
			got = receiveDelta(t, slow)
			if got.Before.ConfigVersionSHA != slowBefore || got.After.ConfigVersionSHA != after {
				t.Fatalf("slow subscriber: delta = {%q -> %q}, want {%q -> %q}", got.Before.ConfigVersionSHA, got.After.ConfigVersionSHA, slowBefore, after)
			}
			slowBefore = after
		}
	}
}

func receiveDelta(t *testing.T, sub DeltaChan) Delta {
	t.Helper()
	select {
	case delta := <-sub:
		return delta
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for config delta")
		return Delta{}
	}
}

type configTestLayout struct {
	prowConfig string
	jobConfigs []string
}

func newConfigTestLayout(t *testing.T, prowVersion string, periodics ...string) configTestLayout {
	t.Helper()
	dir := t.TempDir()
	prowConfig := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(prowConfig, []byte("config_version_sha: "+prowVersion+"\n"), 0600); err != nil {
		t.Fatalf("write prow config: %v", err)
	}
	layout := configTestLayout{prowConfig: prowConfig}
	for i, periodic := range periodics {
		root := filepath.Join(dir, "jobs-"+strconv.Itoa(i))
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatalf("create job config root: %v", err)
		}
		writePeriodicJob(t, root, periodic)
		layout.jobConfigs = append(layout.jobConfigs, root)
	}
	return layout
}

func writePeriodicJob(t *testing.T, root, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.Base(root)+".yaml"), []byte(validPeriodicJobConfig(name)), 0600); err != nil {
		t.Fatalf("write job config: %v", err)
	}
}

func setModTime(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("set modification time of %s: %v", path, err)
	}
}

func assertPeriodics(t *testing.T, c *Config, want ...string) {
	t.Helper()
	if diff := cmp.Diff(sets.New(want...), periodicNames(c.AllPeriodics())); diff != "" {
		t.Errorf("unexpected periodics (-want +got):\n%s", diff)
	}
}

func TestObserveConfigInputs(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	layout := newConfigTestLayout(t, "v1", "job-a", "job-b")
	wantTimes := map[string]time.Time{
		layout.prowConfig:    base,
		layout.jobConfigs[0]: base.Add(2 * time.Hour),
		layout.jobConfigs[1]: base.Add(time.Hour),
	}
	for path, modTime := range wantTimes {
		setModTime(t, path, modTime)
	}

	got, err := observeConfigInputs(layout.prowConfig, layout.jobConfigs)
	if err != nil {
		t.Fatalf("observeConfigInputs failed: %v", err)
	}
	if len(got) != len(wantTimes) {
		t.Fatalf("observed %d inputs, want %d", len(got), len(wantTimes))
	}
	for path, wantTime := range wantTimes {
		input, ok := got[path]
		if !ok {
			t.Errorf("input %s was not observed", path)
			continue
		}
		if !input.modTime.Equal(wantTime) {
			t.Errorf("input %s modTime = %v, want %v", path, input.modTime, wantTime)
		}
	}

	missing := filepath.Join(filepath.Dir(layout.prowConfig), "missing")
	if _, err := observeConfigInputs(layout.prowConfig, append(layout.jobConfigs, missing)); err == nil {
		t.Fatal("expected an error when a configured input is missing")
	}
}

func TestConfigInputModificationsChanged(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	newLayout := func(t *testing.T) configTestLayout {
		layout := newConfigTestLayout(t, "v1", "job-a", "job-b")
		setModTime(t, layout.prowConfig, base)
		setModTime(t, layout.jobConfigs[0], base.Add(2*time.Hour))
		setModTime(t, layout.jobConfigs[1], base.Add(time.Hour))
		return layout
	}

	testCases := []struct {
		name        string
		mutate      func(*testing.T, configTestLayout)
		wantChanged bool
	}{
		{
			name:        "no change",
			mutate:      func(*testing.T, configTestLayout) {},
			wantChanged: false,
		},
		{
			name: "prow config change below another input's timestamp",
			mutate: func(t *testing.T, layout configTestLayout) {
				setModTime(t, layout.prowConfig, base.Add(30*time.Minute))
			},
			wantChanged: true,
		},
		{
			name: "first job root change",
			mutate: func(t *testing.T, layout configTestLayout) {
				setModTime(t, layout.jobConfigs[0], base.Add(3*time.Hour))
			},
			wantChanged: true,
		},
		{
			name: "job root change below another input's timestamp",
			mutate: func(t *testing.T, layout configTestLayout) {
				setModTime(t, layout.jobConfigs[1], base.Add(30*time.Minute))
			},
			wantChanged: true,
		},
		{
			name: "backwards timestamp change",
			mutate: func(t *testing.T, layout configTestLayout) {
				setModTime(t, layout.prowConfig, base.Add(-time.Hour))
			},
			wantChanged: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			layout := newLayout(t)
			before, err := observeConfigInputs(layout.prowConfig, layout.jobConfigs)
			if err != nil {
				t.Fatalf("observe before change: %v", err)
			}
			tc.mutate(t, layout)
			after, err := observeConfigInputs(layout.prowConfig, layout.jobConfigs)
			if err != nil {
				t.Fatalf("observe after change: %v", err)
			}
			if got := after.changed(before); got != tc.wantChanged {
				t.Errorf("changed = %t, want %t", got, tc.wantChanged)
			}
		})
	}
}

func newTestConfigPoller(t *testing.T, layout configTestLayout) (*Agent, *configPoller) {
	t.Helper()
	last, err := observeConfigInputs(layout.prowConfig, layout.jobConfigs)
	if err != nil {
		t.Fatalf("observe config inputs: %v", err)
	}
	c, err := Load(layout.prowConfig, layout.jobConfigs, nil, "")
	if err != nil {
		t.Fatalf("load initial config: %v", err)
	}
	agent := &Agent{}
	agent.Set(c)
	return agent, newConfigPoller(layout.prowConfig, layout.jobConfigs, nil, "", nil, last, agent.Set)
}

func TestConfigPollerDetectsChangeBelowAnotherTimestamp(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	layout := newConfigTestLayout(t, "v1", "job-a", "job-b")
	setModTime(t, layout.prowConfig, base)
	setModTime(t, layout.jobConfigs[0], base.Add(2*time.Hour))
	setModTime(t, layout.jobConfigs[1], base.Add(time.Hour))
	agent, poller := newTestConfigPoller(t, layout)

	writePeriodicJob(t, layout.jobConfigs[1], "job-c")
	setModTime(t, layout.jobConfigs[1], base.Add(30*time.Minute))

	if err := poller.poll(); err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	assertPeriodics(t, agent.Config(), "job-a", "job-c")
}

func TestConfigPollerDetectsBackwardsTimestampChange(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	layout := newConfigTestLayout(t, "v1", "job-a")
	setModTime(t, layout.prowConfig, base)
	setModTime(t, layout.jobConfigs[0], base.Add(time.Hour))
	agent, poller := newTestConfigPoller(t, layout)

	if err := os.WriteFile(layout.prowConfig, []byte("config_version_sha: v2\n"), 0600); err != nil {
		t.Fatalf("write prow config: %v", err)
	}
	setModTime(t, layout.prowConfig, base.Add(-time.Hour))

	if err := poller.poll(); err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if got := agent.Config().ConfigVersionSHA; got != "v2" {
		t.Errorf("ConfigVersionSHA = %q, want %q", got, "v2")
	}
}

func TestConfigPollerPreservesConfigWhenSourceMissingAndRecovers(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	layout := newConfigTestLayout(t, "v1", "job-a", "job-b")
	setModTime(t, layout.prowConfig, base)
	setModTime(t, layout.jobConfigs[0], base.Add(time.Hour))
	setModTime(t, layout.jobConfigs[1], base.Add(2*time.Hour))
	agent, poller := newTestConfigPoller(t, layout)

	if err := os.RemoveAll(layout.jobConfigs[1]); err != nil {
		t.Fatalf("remove job config root: %v", err)
	}
	if err := poller.poll(); err != nil {
		t.Fatalf("poll with missing source failed: %v", err)
	}
	assertPeriodics(t, agent.Config(), "job-a", "job-b")

	if err := os.Mkdir(layout.jobConfigs[1], 0700); err != nil {
		t.Fatalf("recreate job config root: %v", err)
	}
	writePeriodicJob(t, layout.jobConfigs[1], "job-c")
	if err := poller.poll(); err != nil {
		t.Fatalf("poll after recovery failed: %v", err)
	}
	assertPeriodics(t, agent.Config(), "job-a", "job-c")
}

func TestConfigPollerPreservesConfigWhenSourceInvalidAndRecovers(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	layout := newConfigTestLayout(t, "v1", "job-a", "job-b")
	setModTime(t, layout.prowConfig, base)
	setModTime(t, layout.jobConfigs[0], base.Add(time.Hour))
	setModTime(t, layout.jobConfigs[1], base.Add(2*time.Hour))
	agent, poller := newTestConfigPoller(t, layout)

	if err := os.WriteFile(layout.prowConfig, []byte("config_version_sha: v2\n"), 0600); err != nil {
		t.Fatalf("write prow config: %v", err)
	}
	setModTime(t, layout.prowConfig, base.Add(3*time.Hour))
	writePeriodicJob(t, layout.jobConfigs[0], "job-c")
	setModTime(t, layout.jobConfigs[0], base.Add(4*time.Hour))
	if err := os.WriteFile(filepath.Join(layout.jobConfigs[1], filepath.Base(layout.jobConfigs[1])+".yaml"), []byte("periodics: ["), 0600); err != nil {
		t.Fatalf("write invalid job config: %v", err)
	}
	setModTime(t, layout.jobConfigs[1], base.Add(5*time.Hour))

	if err := poller.poll(); err == nil {
		t.Fatal("expected an error for an invalid source")
	}
	if got := agent.Config().ConfigVersionSHA; got != "v1" {
		t.Errorf("ConfigVersionSHA = %q, want the last accepted %q", got, "v1")
	}
	assertPeriodics(t, agent.Config(), "job-a", "job-b")

	writePeriodicJob(t, layout.jobConfigs[1], "job-d")
	setModTime(t, layout.jobConfigs[1], base.Add(6*time.Hour))
	if err := poller.poll(); err != nil {
		t.Fatalf("poll after recovery failed: %v", err)
	}
	if got := agent.Config().ConfigVersionSHA; got != "v2" {
		t.Errorf("ConfigVersionSHA = %q, want %q", got, "v2")
	}
	assertPeriodics(t, agent.Config(), "job-c", "job-d")
}

func TestConfigPollerFallsBackWithoutMetadataChange(t *testing.T) {
	state := configInputModifications{
		"config": {modTime: time.Unix(1_700_000_000, 0)},
	}
	loads := 0
	var accepted *Config
	poller := &configPoller{
		last:  state,
		skips: configReloadFallbackSkips - 1,
		observe: func() (configInputModifications, error) {
			return state, nil
		},
		load: func() (*Config, error) {
			loads++
			return cfg("fallback"), nil
		},
		accept: func(c *Config) {
			accepted = c
		},
	}

	if err := poller.poll(); err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if loads != 0 {
		t.Fatalf("loaded %d times before the fallback, want 0", loads)
	}
	if poller.skips != configReloadFallbackSkips {
		t.Fatalf("skips = %d, want %d", poller.skips, configReloadFallbackSkips)
	}

	if err := poller.poll(); err != nil {
		t.Fatalf("fallback poll failed: %v", err)
	}
	if loads != 1 {
		t.Errorf("loaded %d times at the fallback, want 1", loads)
	}
	if poller.skips != 0 {
		t.Errorf("skips = %d after a successful reload, want 0", poller.skips)
	}
	if accepted == nil || accepted.ConfigVersionSHA != "fallback" {
		t.Errorf("accepted config = %v, want version %q", accepted, "fallback")
	}
}

func TestStartReturnsInitialLoadError(t *testing.T) {
	agent := &Agent{}
	if err := agent.Start(filepath.Join(t.TempDir(), "missing.yaml"), nil, nil, ""); err == nil {
		t.Fatal("expected Start to return the initial load error")
	}
	if agent.Config() != nil {
		t.Error("config was set despite the initial load failure")
	}
}
