/*
Copyright 2020 The Kubernetes Authors.

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

package metrics

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlruntimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/prow/pkg/config"
	"sigs.k8s.io/prow/pkg/flagutil"
	"sigs.k8s.io/prow/pkg/interrupts"
)

type fakeListenAndServer struct {
	ctx    context.Context
	server *httptest.Server
}

func (fls *fakeListenAndServer) ListenAndServe() error {
	defer fls.server.Close()
	// Already listening and serving
	<-fls.ctx.Done()
	return http.ErrServerClosed
}

func (fls *fakeListenAndServer) Shutdown(ctx context.Context) error {
	return fls.server.Config.Shutdown(ctx)
}

func (fls *fakeListenAndServer) CreateServer(handler http.Handler) interrupts.ListenAndServer {
	fls.server = httptest.NewServer(handler)
	return fls
}

func TestExposeMetrics(t *testing.T) {
	ctx := t.Context()
	fls := fakeListenAndServer{ctx: ctx}

	ExposeMetricsWithRegistry("my-component", config.PushGateway{}, flagutil.DefaultMetricsPort, nil, fls.CreateServer)
	resp, err := http.Get(fls.server.URL + "/metrics")
	if err != nil {
		t.Fatalf("failed getting metrics: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("response status was not %d but %d", http.StatusOK, resp.StatusCode)
	}
}

// TestExposeMetricsWithControllerRuntimeCollectors verifies that the
// Unregister calls in ExposeMetricsWithRegistry match what controller-runtime
// v0.21.0 registers via its init in pkg/internal/controller/metrics. The test
// binary does not trigger that init, so we register the same collectors
// manually.
func TestExposeMetricsWithControllerRuntimeCollectors(t *testing.T) {
	goC := collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll))
	procC := collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})
	//nolint:staticcheck
	ctrlruntimemetrics.Registry.MustRegister(goC, procC)
	t.Cleanup(func() {
		//nolint:staticcheck
		ctrlruntimemetrics.Registry.Unregister(goC)
		//nolint:staticcheck
		ctrlruntimemetrics.Registry.Unregister(procC)
	})

	ctx := t.Context()
	fls := fakeListenAndServer{ctx: ctx}
	ExposeMetricsWithRegistry("test", config.PushGateway{}, flagutil.DefaultMetricsPort, nil, fls.CreateServer)

	resp, err := http.Get(fls.server.URL + "/metrics")
	if err != nil {
		t.Fatalf("failed getting metrics: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed reading body: %v", err)
	}
	if strings.Contains(string(body), "collected before with the same name") {
		t.Fatal("duplicate collector: Unregister options do not match controller-runtime")
	}
}

func TestExposeMetricsUsesDefaultRegistryAndAdditionalCollectors(t *testing.T) {
	defaultMetric := prometheus.NewGauge(prometheus.GaugeOpts{Name: "metrics_default_registry_test"})
	prometheus.MustRegister(defaultMetric)
	t.Cleanup(func() { prometheus.Unregister(defaultMetric) })
	additionalMetric := prometheus.NewGauge(prometheus.GaugeOpts{Name: "metrics_additional_test"})
	fls := fakeListenAndServer{ctx: t.Context()}
	ExposeMetricsWithRegistry("test", config.PushGateway{}, 0, nil, fls.CreateServer, additionalMetric)
	response, err := http.Get(fls.server.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("HTTP status: %d: %s", response.StatusCode, body)
	}
	for _, name := range []string{"metrics_default_registry_test", "metrics_additional_test"} {
		if !strings.Contains(string(body), "# HELP "+name+" ") {
			t.Errorf("missing %s", name)
		}
	}
}

func TestExposeMetricsPushAndHTTPUseSameGatherer(t *testing.T) {
	for _, serve := range []bool{false, true} {
		t.Run(map[bool]string{false: "push-only", true: "push-and-http"}[serve], func(t *testing.T) {
			customRegistry := prometheus.NewRegistry()
			customMetric := prometheus.NewGauge(prometheus.GaugeOpts{Name: "metrics_custom_registry_test"})
			customRegistry.MustRegister(customMetric, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
			extraMetric := prometheus.NewGauge(prometheus.GaugeOpts{Name: "metrics_extra_collector_test"})
			runtimeMetric := prometheus.NewGauge(prometheus.GaugeOpts{Name: "metrics_controller_runtime_test"})
			//nolint:staticcheck
			ctrlruntimemetrics.Registry.MustRegister(runtimeMetric)
			t.Cleanup(func() {
				//nolint:staticcheck
				ctrlruntimemetrics.Registry.Unregister(runtimeMetric)
			})
			pushed := make(chan []byte, 1)
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("push method: %s", r.Method)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read push: %v", err)
				}
				pushed <- body
				w.WriteHeader(http.StatusAccepted)
			}))
			defer gateway.Close()
			fls := fakeListenAndServer{ctx: t.Context()}
			var createServer CreateServer
			if serve {
				createServer = fls.CreateServer
			}
			ExposeMetricsWithRegistry("test", config.PushGateway{Endpoint: gateway.URL, Interval: &metav1.Duration{Duration: time.Hour}, ServeMetrics: serve}, 0, customRegistry, createServer, extraMetric)
			var body []byte
			select {
			case body = <-pushed:
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for push")
			}
			pushedFamilies := map[string]bool{}
			decoder := expfmt.NewDecoder(bytes.NewReader(body), expfmt.NewFormat(expfmt.TypeProtoDelim))
			for {
				var family dto.MetricFamily
				if err := decoder.Decode(&family); err == io.EOF {
					break
				} else if err != nil {
					t.Fatal(err)
				}
				pushedFamilies[family.GetName()] = true
			}
			for _, name := range []string{"metrics_custom_registry_test", "metrics_extra_collector_test", "metrics_controller_runtime_test", "go_goroutines", "process_cpu_seconds_total"} {
				if !pushedFamilies[name] {
					t.Errorf("push missing %s", name)
				}
			}
			if !serve {
				return
			}
			response, err := http.Get(fls.server.URL + "/metrics")
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			textBody, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusOK {
				t.Fatalf("HTTP status: %d: %s", response.StatusCode, textBody)
			}
			scrapedFamilies := map[string]bool{}
			for _, line := range strings.Split(string(textBody), "\n") {
				if strings.HasPrefix(line, "# HELP ") {
					fields := strings.Fields(line)
					scrapedFamilies[fields[2]] = true
				}
			}
			for name := range pushedFamilies {
				if !scrapedFamilies[name] {
					t.Errorf("HTTP missing pushed family %s", name)
				}
			}
			for name := range scrapedFamilies {
				if !pushedFamilies[name] {
					t.Errorf("push missing HTTP family %s", name)
				}
			}
		})
	}
}
