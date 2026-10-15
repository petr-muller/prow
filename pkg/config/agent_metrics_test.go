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
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func writeAgentConfig(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func agentMetrics(t *testing.T, agent *Agent) map[string]float64 {
	t.Helper()
	registry := prometheus.NewRegistry()
	if err := registry.Register(agent); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]float64)
	for _, family := range families {
		for _, metric := range family.Metric {
			name := family.GetName()
			for _, label := range metric.Label {
				name += "/" + label.GetValue()
			}
			if metric.Counter != nil {
				values[name] = metric.Counter.GetValue()
			} else {
				values[name] = metric.Gauge.GetValue()
			}
		}
	}
	return values
}

func waitForAgentMetric(t *testing.T, agent *Agent, key string, atLeast float64) map[string]float64 {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		got := agentMetrics(t, agent)
		if got[key] >= atLeast {
			return got
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("%s did not reach %v; got %v", key, atLeast, agentMetrics(t, agent))
	return nil
}

func TestAgentLoadMetrics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeAgentConfig(t, path, "{}\n")
	agent, other := &Agent{}, &Agent{}
	timestamp := "prow_config_last_successful_load_timestamp_seconds"
	success := "prow_config_load_attempts_total/success"
	failure := "prow_config_load_attempts_total/failure"
	periodics := "prow_config_static_job_definition/periodic"
	addDefinitions := func(c *Config) error {
		c.ConfigVersionSHA = "loaded"
		c.Periodics = []Periodic{{}, {}}
		return nil
	}

	got := agentMetrics(t, agent)
	if got[success] != 0 || got[failure] != 0 || got[timestamp] != 0 {
		t.Fatalf("uninitialized metrics = %v", got)
	}
	if _, ok := got[periodics]; ok {
		t.Fatalf("job counts emitted before acceptance: %v", got)
	}
	if err := agent.loadAndSet(path, "", nil, "", addDefinitions); err != nil {
		t.Fatal(err)
	}
	got = agentMetrics(t, agent)
	firstTimestamp := got[timestamp]
	if got[success] != 1 || got[failure] != 0 || firstTimestamp <= 0 || got[periodics] != 2 {
		t.Fatalf("first load metrics = %v", got)
	}

	writeAgentConfig(t, path, "[invalid\n")
	if err := agent.loadAndSet(path, "", nil, ""); err == nil {
		t.Fatal("expected invalid config to fail")
	}
	got = agentMetrics(t, agent)
	if got[success] != 1 || got[failure] != 1 || got[timestamp] != firstTimestamp || got[periodics] != 2 || agent.Config().ConfigVersionSHA != "loaded" {
		t.Fatalf("failed load changed accepted state: metrics=%v, config=%v", got, agent.Config())
	}

	writeAgentConfig(t, path, "{}\n")
	if err := agent.loadAndSet(path, "", nil, "", addDefinitions); err != nil {
		t.Fatal(err)
	}
	got = agentMetrics(t, agent)
	if got[success] != 2 || got[failure] != 1 || got[timestamp] < firstTimestamp {
		t.Fatalf("recovery metrics = %v", got)
	}
	agent.Set(cfg("manual"))
	agent.SetWithoutBroadcast(cfg("manual-again"))
	manual := agentMetrics(t, agent)
	if manual[success] != 2 || manual[failure] != 1 || manual[timestamp] != got[timestamp] {
		t.Fatalf("setters changed load metrics: %v", manual)
	}
	if separate := agentMetrics(t, other); separate[success] != 0 || separate[failure] != 0 || separate[timestamp] != 0 {
		t.Fatalf("other agent inherited metrics: %v", separate)
	}
}

func TestAgentInitialLoadOutcomes(t *testing.T) {
	for _, start := range []struct {
		name string
		call func(*Agent, string) error
	}{
		{"poll", func(a *Agent, path string) error { return a.Start(path, "", nil, "") }},
		{"watch", func(a *Agent, path string) error { return a.StartWatch(path, "", nil, "") }},
	} {
		t.Run(start.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			writeAgentConfig(t, path, "[invalid\n")
			agent := &Agent{}
			if err := start.call(agent, path); err == nil {
				t.Fatal("expected initial load failure")
			}
			got := agentMetrics(t, agent)
			if got["prow_config_load_attempts_total/failure"] != 1 || got["prow_config_load_attempts_total/success"] != 0 || got["prow_config_last_successful_load_timestamp_seconds"] != 0 {
				t.Fatalf("initial failure metrics = %v", got)
			}
		})
	}
}

func TestAgentPollLoadOutcomes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeAgentConfig(t, path, "{}\n")
	agent := &Agent{}
	if err := agent.Start(path, "", nil, ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1200 * time.Millisecond)
	if got := agentMetrics(t, agent); got["prow_config_load_attempts_total/success"] != 1 || got["prow_config_load_attempts_total/failure"] != 0 {
		t.Fatalf("unchanged poll counted a load: %v", got)
	}
	initialTimestamp := agentMetrics(t, agent)["prow_config_last_successful_load_timestamp_seconds"]
	accepted := agent.Config()
	writeAgentConfig(t, path, "[invalid\n")
	mtime := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	got := waitForAgentMetric(t, agent, "prow_config_load_attempts_total/failure", 1)
	if got["prow_config_last_successful_load_timestamp_seconds"] != initialTimestamp || agent.Config() != accepted {
		t.Fatalf("failed poll changed accepted config or timestamp: %v", got)
	}
	writeAgentConfig(t, path, "{}\n")
	mtime = mtime.Add(time.Second)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	got = waitForAgentMetric(t, agent, "prow_config_load_attempts_total/success", 2)
	if got["prow_config_last_successful_load_timestamp_seconds"] <= initialTimestamp {
		t.Fatalf("recovery did not advance timestamp: %v", got)
	}
}

func TestAgentWatcherLoadOutcomes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeAgentConfig(t, path, "{}\n")
	agent := &Agent{}
	if err := agent.StartWatch(path, "", nil, ""); err != nil {
		t.Fatal(err)
	}
	initialTimestamp := agentMetrics(t, agent)["prow_config_last_successful_load_timestamp_seconds"]
	accepted := agent.Config()
	writeAgentConfig(t, path, "[invalid\n")
	got := waitForAgentMetric(t, agent, "prow_config_load_attempts_total/failure", 1)
	if got["prow_config_last_successful_load_timestamp_seconds"] != initialTimestamp || agent.Config() != accepted {
		t.Fatalf("failed watcher load changed accepted state: %v", got)
	}
	writeAgentConfig(t, path, "{}\n")
	got = waitForAgentMetric(t, agent, "prow_config_load_attempts_total/success", 2)
	if got["prow_config_last_successful_load_timestamp_seconds"] <= initialTimestamp {
		t.Fatalf("watcher recovery did not advance timestamp: %v", got)
	}
}

func TestAgentLoadMetricsConcurrentCollect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeAgentConfig(t, path, "{}\n")
	agent := &Agent{}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 20 {
			if err := agent.loadAndSet(path, "", nil, ""); err != nil {
				t.Error(err)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for range 20 {
			agentMetrics(t, agent)
		}
	}()
	wg.Wait()
	if got := agentMetrics(t, agent); got["prow_config_load_attempts_total/success"] != 20 {
		t.Fatalf("concurrent load count = %v", got)
	}
}
