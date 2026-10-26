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
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type watchedSections struct {
	enabled map[string]*bool
}

func watchTestSections() (func(*Config) watchedSections, <-chan struct{}) {
	ready := make(chan struct{})
	var once sync.Once
	return func(c *Config) watchedSections {
		once.Do(func() { close(ready) })
		return watchedSections{c.InRepoConfig.Enabled}
	}, ready
}

func runWatch(t *testing.T, agent *Agent, extraDone func() <-chan struct{}, onChange func(*Config) error) (context.CancelFunc, <-chan struct{}, <-chan error) {
	t.Helper()
	sections, ready := watchTestSections()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- WatchConfig(ctx, agent, extraDone, sections, onChange)
	}()
	return cancel, ready, errCh
}

func waitWatchError(t *testing.T, errCh <-chan error) error {
	t.Helper()
	select {
	case err := <-errCh:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for WatchConfig to return")
		return nil
	}
}

func TestWatchConfigUnchangedSnapshotDoesNotNotify(t *testing.T) {
	agent := &Agent{}
	agent.Set(&Config{})
	notifications := make(chan *Config, 4)
	snapshotCalls := make(chan struct{})
	sections := func(c *Config) watchedSections {
		snapshotCalls <- struct{}{}
		return watchedSections{c.InRepoConfig.Enabled}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- WatchConfig(ctx, agent, nil, sections, func(c *Config) error {
			notifications <- c
			return nil
		})
	}()
	waitForSnapshot := func() {
		t.Helper()
		select {
		case <-snapshotCalls:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for the sections callback")
		}
	}

	waitForSnapshot() // Initial snapshot.
	agent.Set(&Config{ProwConfig: ProwConfig{ConfigVersionSHA: "unrelated-1"}})
	waitForSnapshot() // The first unrelated delta has been snapshotted.

	// A second unrelated delta acts as a barrier: by the time it is
	// snapshotted, the hook for the first one has already run.
	agent.Set(&Config{ProwConfig: ProwConfig{ConfigVersionSHA: "unrelated-2"}})
	waitForSnapshot()
	select {
	case got := <-notifications:
		t.Fatalf("hook fired for a config whose watched sections did not change: %+v", got)
	default:
	}

	enabled := true
	agent.Set(&Config{ProwConfig: ProwConfig{InRepoConfig: InRepoConfig{
		Enabled: map[string]*bool{"org/repo": &enabled},
	}}})
	waitForSnapshot()

	select {
	case got := <-notifications:
		if got.InRepoConfig.Enabled["org/repo"] == nil || !*got.InRepoConfig.Enabled["org/repo"] {
			t.Fatalf("hook received the wrong config: %+v", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for a notification")
	}

	cancel()
	if err := waitWatchError(t, errCh); err != nil {
		t.Fatalf("WatchConfig returned error: %v", err)
	}
	select {
	case extra := <-notifications:
		t.Fatalf("unexpected extra notification: %+v", extra)
	default:
	}
}

func TestWatchConfigChangedSnapshotNotifies(t *testing.T) {
	agent := &Agent{}
	agent.Set(&Config{})
	notifications := make(chan *Config, 1)
	cancel, ready, errCh := runWatch(t, agent, nil, func(c *Config) error {
		notifications <- c
		return nil
	})
	defer cancel()
	<-ready

	enabled := true
	changed := &Config{ProwConfig: ProwConfig{
		ConfigVersionSHA: "changed",
		InRepoConfig: InRepoConfig{
			Enabled: map[string]*bool{"org/repo": &enabled},
		},
	}}
	agent.Set(changed)

	select {
	case got := <-notifications:
		if got.ConfigVersionSHA != changed.ConfigVersionSHA {
			t.Errorf("hook received config %q, want %q", got.ConfigVersionSHA, changed.ConfigVersionSHA)
		}
		if got.InRepoConfig.Enabled["org/repo"] == nil || !*got.InRepoConfig.Enabled["org/repo"] {
			t.Errorf("hook received the wrong watched sections: %+v", got.InRepoConfig)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for a notification")
	}

	cancel()
	if err := waitWatchError(t, errCh); err != nil {
		t.Fatalf("WatchConfig returned error: %v", err)
	}
}

func TestWatchConfigContextCancellationReturns(t *testing.T) {
	agent := &Agent{}
	agent.Set(&Config{})
	cancel, ready, errCh := runWatch(t, agent, nil, func(*Config) error {
		t.Error("hook called unexpectedly")
		return nil
	})
	<-ready
	cancel()

	if err := waitWatchError(t, errCh); err != nil {
		t.Fatalf("WatchConfig returned error on context cancellation: %v", err)
	}
}

func TestWatchConfigExtraDoneReturns(t *testing.T) {
	agent := &Agent{}
	agent.Set(&Config{})
	done := make(chan struct{})
	cancel, ready, errCh := runWatch(t, agent, func() <-chan struct{} { return done }, func(*Config) error {
		t.Error("hook called unexpectedly")
		return nil
	})
	defer cancel()
	<-ready
	close(done)

	if err := waitWatchError(t, errCh); !errors.Is(err, ErrWatchDone) {
		t.Fatalf("WatchConfig returned %v, want %v", err, ErrWatchDone)
	}
}

func TestWatchConfigHookErrorPropagates(t *testing.T) {
	agent := &Agent{}
	agent.Set(&Config{})
	hookErr := errors.New("hook failed")
	cancel, ready, errCh := runWatch(t, agent, nil, func(*Config) error {
		return hookErr
	})
	defer cancel()
	<-ready

	enabled := true
	agent.Set(&Config{ProwConfig: ProwConfig{InRepoConfig: InRepoConfig{
		Enabled: map[string]*bool{"org/repo": &enabled},
	}}})

	if err := waitWatchError(t, errCh); !errors.Is(err, hookErr) {
		t.Fatalf("WatchConfig returned %v, want %v", err, hookErr)
	}
}
