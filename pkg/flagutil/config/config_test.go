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

package flagutil

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/prow/pkg/config"
	"sigs.k8s.io/prow/pkg/flagutil"
)

func TestJobConfigPathFlag(t *testing.T) {
	testCases := []struct {
		name     string
		initial  ConfigOptions
		args     []string
		expected []string
	}{
		{
			name: "no flag",
		},
		{
			name:     "one path",
			args:     []string{"--job-config-path=a"},
			expected: []string{"a"},
		},
		{
			name:     "repeated flag keeps all the paths in order",
			args:     []string{"--job-config-path=a", "--job-config-path=b", "--job-config-path=c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "paths are not split on commas",
			args:     []string{"--job-config-path=a,b"},
			expected: []string{"a,b"},
		},
		{
			name:     "the flag name can be overridden",
			initial:  ConfigOptions{JobConfigPathFlagName: "prow-job-config-path"},
			args:     []string{"--prow-job-config-path=a", "--prow-job-config-path=b"},
			expected: []string{"a", "b"},
		},
		{
			name:     "a default is used when the flag is not given",
			initial:  ConfigOptions{JobConfigPaths: flagutil.NewStrings("default")},
			expected: []string{"default"},
		},
		{
			name:     "a default is replaced by every path of the flag",
			initial:  ConfigOptions{JobConfigPaths: flagutil.NewStrings("default")},
			args:     []string{"--job-config-path=a", "--job-config-path=b"},
			expected: []string{"a", "b"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			options := tc.initial
			fs := flag.NewFlagSet(tc.name, flag.ContinueOnError)
			options.AddFlags(fs)
			if err := fs.Parse(tc.args); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.expected, options.JobConfigPaths.Strings()); diff != "" {
				t.Errorf("unexpected job config paths (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateConfigOptional(t *testing.T) {
	testCases := []struct {
		name          string
		options       ConfigOptions
		expectedError string
	}{
		{
			name: "nothing is set",
		},
		{
			name:    "the config path alone is fine",
			options: ConfigOptions{ConfigPath: "config.yaml"},
		},
		{
			name:    "job config paths with the config path",
			options: ConfigOptions{ConfigPath: "config.yaml", JobConfigPaths: flagutil.NewStrings("a", "b")},
		},
		{
			name:          "job config paths without the config path",
			options:       ConfigOptions{JobConfigPaths: flagutil.NewStrings("a", "b"), ConfigPathFlagName: "config-path", JobConfigPathFlagName: "job-config-path"},
			expectedError: "if --job-config-path is given, --config-path must be given as well",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var actualError string
			if err := tc.options.ValidateConfigOptional(); err != nil {
				actualError = err.Error()
			}

			if diff := cmp.Diff(tc.expectedError, actualError); diff != "" {
				t.Errorf("unexpected error (-want +got):\n%s", diff)
			}
		})
	}
}

func writePeriodicJobConfig(t *testing.T, dir, file, name string) {
	t.Helper()
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatalf("fail to create dir: %v", err)
	}
	job := "periodics:\n- interval: 10m\n  agent: kubernetes\n  name: " + name + "\n  spec:\n    containers:\n    - image: alpine\n"
	if err := os.WriteFile(filepath.Join(dir, file), []byte(job), 0600); err != nil {
		t.Fatalf("fail to write job config: %v", err)
	}
}

func TestConfigAgentJobConfigPaths(t *testing.T) {
	testCases := []struct {
		name          string
		bFile         string
		expected      []string
		expectedError string
	}{
		{
			name:     "jobs of all the paths are loaded",
			bFile:    "b-jobs.yaml",
			expected: []string{"a", "b"},
		},
		{
			name:          "the same basename in two paths is rejected",
			bFile:         "jobs.yaml",
			expectedError: "duplicated basename is not allowed: {b}/jobs.yaml (already loaded from {a}/jobs.yaml)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			prowConfig := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(prowConfig, nil, 0600); err != nil {
				t.Fatalf("fail to write prow config: %v", err)
			}
			a := filepath.Join(dir, "a")
			writePeriodicJobConfig(t, a, "jobs.yaml", "a")
			b := filepath.Join(dir, "b")
			writePeriodicJobConfig(t, b, tc.bFile, "b")

			var options ConfigOptions
			fs := flag.NewFlagSet(tc.name, flag.ContinueOnError)
			options.AddFlags(fs)
			args := []string{"--config-path=" + prowConfig, "--job-config-path=" + a, "--job-config-path=" + b}
			if err := fs.Parse(args); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			agent, err := options.ConfigAgent()

			var actualError string
			if err != nil {
				actualError = err.Error()
			}
			expectedError := strings.NewReplacer("{a}", a, "{b}", b).Replace(tc.expectedError)
			if diff := cmp.Diff(expectedError, actualError); diff != "" {
				t.Fatalf("unexpected error (-want +got):\n%s", diff)
			}
			if tc.expectedError != "" {
				return
			}
			actual := sets.New[string]()
			for _, periodic := range agent.Config().AllPeriodics() {
				actual.Insert(periodic.Name)
			}
			if diff := cmp.Diff(sets.New(tc.expected...), actual); diff != "" {
				t.Errorf("unexpected periodics (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConfigLoadingPaths(t *testing.T) {
	for _, suffix := range []string{"_prowconfig.yaml", "_custom.yaml"} {
		t.Run(suffix, func(t *testing.T) {
			dir := t.TempDir()
			prowConfig := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(prowConfig, []byte("plank:\n  job_queue_capacities:\n    main: 1\n"), 0600); err != nil {
				t.Fatal(err)
			}
			var jobPaths, supplementalDirs []string
			for _, name := range []string{"a", "b"} {
				jobPath := filepath.Join(dir, "jobs-"+name)
				writePeriodicJobConfig(t, jobPath, name+".yaml", name)
				jobPaths = append(jobPaths, jobPath)
				supplementalDir := filepath.Join(dir, "supplemental-"+name)
				if err := os.Mkdir(supplementalDir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(supplementalDir, "queues"+suffix), []byte("plank:\n  job_queue_capacities:\n    "+name+": 1\n"), 0600); err != nil {
					t.Fatal(err)
				}
				// Files that do not match the selected suffix must be ignored.
				if err := os.WriteFile(filepath.Join(supplementalDir, "ignored.yaml"), []byte("invalid: ["), 0600); err != nil {
					t.Fatal(err)
				}
				supplementalDirs = append(supplementalDirs, supplementalDir)
			}
			options := ConfigOptions{
				ConfigPath:                            prowConfig,
				JobConfigPaths:                        flagutil.NewStrings(jobPaths...),
				SupplementalProwConfigDirs:            flagutil.NewStrings(supplementalDirs...),
				SupplementalProwConfigsFileNameSuffix: suffix,
			}
			cfg, err := options.LoadStrict()
			if err != nil {
				t.Fatalf("LoadStrict failed: %v", err)
			}
			agent, err := options.ConfigAgent()
			if err != nil {
				t.Fatalf("ConfigAgent failed: %v", err)
			}
			for name, loaded := range map[string]*config.Config{"LoadStrict": cfg, "ConfigAgent": agent.Config()} {
				actual := sets.New[string]()
				for _, periodic := range loaded.AllPeriodics() {
					actual.Insert(periodic.Name)
				}
				if diff := cmp.Diff(sets.New("a", "b"), actual); diff != "" {
					t.Errorf("%s: unexpected periodics (-want +got):\n%s", name, diff)
				}
				if diff := cmp.Diff(config.JobQueueCapacities{"main": 1, "a": 1, "b": 1}, loaded.Plank.JobQueueCapacities); diff != "" {
					t.Errorf("%s: unexpected queue capacities (-want +got):\n%s", name, diff)
				}
			}

			called := false
			wantErr := errors.New("additional validation failed")
			_, err = options.LoadStrict(func(got *config.Config) error {
				called = true
				if diff := cmp.Diff(cfg.Plank.JobQueueCapacities, got.Plank.JobQueueCapacities); diff != "" {
					t.Errorf("callback received unexpected config (-want +got):\n%s", diff)
				}
				return wantErr
			})
			if !called || err != wantErr {
				t.Errorf("callback called = %v, error = %v, want true and %v", called, err, wantErr)
			}
		})
	}
}

func TestLoadStrictErrors(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(valid, nil, 0600); err != nil {
		t.Fatal(err)
	}
	invalid := filepath.Join(dir, "invalid.yaml")
	if err := os.WriteFile(invalid, []byte("unknown_field: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing")
	for name, options := range map[string]ConfigOptions{
		"missing prow config": {ConfigPath: missing},
		"unknown prow field":  {ConfigPath: invalid},
		"missing job config":  {ConfigPath: valid, JobConfigPaths: flagutil.NewStrings(missing)},
		"unknown job field":   {ConfigPath: valid, JobConfigPaths: flagutil.NewStrings(invalid)},
		"missing supplemental directory": {
			ConfigPath: valid, SupplementalProwConfigDirs: flagutil.NewStrings(missing),
			SupplementalProwConfigsFileNameSuffix: "_custom.yaml",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, wantErr := config.LoadStrict(options.ConfigPath, options.JobConfigPaths.Strings(), options.SupplementalProwConfigDirs.Strings(), options.SupplementalProwConfigsFileNameSuffix)
			if wantErr == nil {
				t.Fatal("expected config.LoadStrict to fail")
			}
			cfg, err := options.LoadStrict()
			if err == nil || err.Error() != wantErr.Error() || cfg != nil {
				t.Errorf("LoadStrict() = (%v, %v), want (nil, %v)", cfg, err, wantErr)
			}
		})
	}
}
