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
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
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

func TestExposeMetricsWithSharedGatherer(t *testing.T) {
	for _, tc := range []struct {
		name           string
		customRegistry bool
		serveMetrics   bool
	}{
		{name: "default registry", serveMetrics: true},
		{name: "custom registry", customRegistry: true, serveMetrics: true},
		{name: "push only", customRegistry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defaultMetric := prometheus.NewGauge(prometheus.GaugeOpts{Name: "prow_metrics_test_default"})
			prometheus.MustRegister(defaultMetric)
			t.Cleanup(func() { prometheus.Unregister(defaultMetric) })

			customRegistry := prometheus.NewRegistry()
			customRegistry.MustRegister(
				prometheus.NewGauge(prometheus.GaugeOpts{Name: "prow_metrics_test_custom"}),
				collectors.NewGoCollector(),
				prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
			)
			additionalMetric := prometheus.NewGauge(prometheus.GaugeOpts{Name: "prow_metrics_test_additional"})
			runtimeMetric := prometheus.NewGauge(prometheus.GaugeOpts{Name: "prow_metrics_test_runtime"})
			// Match the collectors registered by controller-runtime's init to ensure
			// duplicate runtime collectors are removed even when only pushing.
			goC := collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll))
			procC := prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{})
			//nolint:staticcheck
			ctrlruntimemetrics.Registry.MustRegister(runtimeMetric, goC, procC)
			t.Cleanup(func() {
				ctrlruntimemetrics.Registry.Unregister(runtimeMetric)
				//nolint:staticcheck
				ctrlruntimemetrics.Registry.Unregister(goC)
				//nolint:staticcheck
				ctrlruntimemetrics.Registry.Unregister(procC)
			})

			pushBody := make(chan string, 1)
			pushServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("failed reading pushed metrics: %v", err)
				}
				pushBody <- string(body)
				w.WriteHeader(http.StatusAccepted)
			}))
			defer pushServer.Close()

			var registry prometheus.Gatherer
			if tc.customRegistry {
				registry = customRegistry
			}
			fls := fakeListenAndServer{ctx: t.Context()}
			ExposeMetricsWithRegistry("test", config.PushGateway{
				Endpoint:     pushServer.URL,
				Interval:     &metav1.Duration{Duration: time.Hour},
				ServeMetrics: tc.serveMetrics,
			}, flagutil.DefaultMetricsPort, registry, fls.CreateServer, additionalMetric)

			checkMetrics := func(output, body string) {
				for name, want := range map[string]bool{
					"prow_metrics_test_default":    !tc.customRegistry,
					"prow_metrics_test_custom":     tc.customRegistry,
					"prow_metrics_test_additional": true,
					"prow_metrics_test_runtime":    true,
				} {
					if got := strings.Contains(body, name); got != want {
						t.Errorf("%s contains %s: got %v, want %v", output, name, got, want)
					}
				}
			}
			if tc.serveMetrics {
				resp, err := http.Get(fls.server.URL + "/metrics")
				if err != nil {
					t.Fatalf("failed getting metrics: %v", err)
				}
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("failed reading metrics: %v", err)
				}
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("scrape returned %d: %s", resp.StatusCode, body)
				}
				checkMetrics("scrape", string(body))
			} else if fls.server != nil {
				t.Error("push-only configuration started a scrape server")
			}

			select {
			case body := <-pushBody:
				checkMetrics("push", body)
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for metrics push")
			}
		})
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
