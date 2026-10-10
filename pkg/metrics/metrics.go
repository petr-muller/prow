/*
Copyright 2017 The Kubernetes Authors.

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

// Package metrics contains utilities for working with metrics in prow.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	ctrlruntimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/prow/pkg/config"
	"sigs.k8s.io/prow/pkg/interrupts"
)

type CreateServer func(http.Handler) interrupts.ListenAndServer

// ExposeMetricsWithRegistry serves and/or pushes the same metrics from the supplied registry
// (or the default registry when nil), controller-runtime's registry, and additional collectors.
// Additional collectors are registered once and included in both paths. They must not already be
// registered in any of the included registries.
func ExposeMetricsWithRegistry(component string, pushGateway config.PushGateway, port int, reg prometheus.Gatherer, createServer CreateServer, additionalCollectors ...prometheus.Collector) {
	if reg == nil {
		reg = prometheus.DefaultGatherer
	}
	gatherer := prometheus.Gatherers{reg, ctrlruntimemetrics.Registry}
	if len(additionalCollectors) > 0 {
		additionalRegistry := prometheus.NewRegistry()
		additionalRegistry.MustRegister(additionalCollectors...)
		gatherer = append(gatherer, additionalRegistry)
	}

	// These get registered in controller-runtimes registry via an init in the internal/controller/metrics package. if
	// we dont unregister them, metrics break if that package is somehow imported.
	// Setting the default prometheus registry in controller-runtime is unfortunately not an option, because that would
	// result in all metrics that got registered in controller-runtime via an init to vanish, as inits of dependencies
	// always get executed before our own init.
	// Exempted from linting, it uses a deprecated Prometheus function but must match what controller-runtime does so
	// we can not change it.

	//nolint:staticcheck
	ctrlruntimemetrics.Registry.Unregister(
		collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll)),
	)
	//nolint:staticcheck
	ctrlruntimemetrics.Registry.Unregister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	if pushGateway.Endpoint != "" {
		pushMetrics(component, pushGateway.Endpoint, pushGateway.Interval.Duration, gatherer)
		if !pushGateway.ServeMetrics {
			return
		}
	}
	handler := promhttp.HandlerFor(
		gatherer,
		promhttp.HandlerOpts{},
	)
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", handler)
	var server interrupts.ListenAndServer
	if createServer == nil {
		server = &http.Server{Addr: ":" + strconv.Itoa(port), Handler: metricsMux}
	} else {
		server = createServer(handler)
	}
	interrupts.ListenAndServe(server, 5*time.Second)
}

// ExposeMetrics chooses whether to serve or push metrics for the service.
func ExposeMetrics(component string, pushGateway config.PushGateway, port int, additionalCollectors ...prometheus.Collector) {
	ExposeMetricsWithRegistry(component, pushGateway, port, nil, nil, additionalCollectors...)
}

// pushMetrics is meant to run in a goroutine and continuously push
// metrics to the provided endpoint.
func pushMetrics(component, endpoint string, interval time.Duration, gatherer prometheus.Gatherer) {
	interrupts.TickLiteral(func() {
		if err := fromGatherer(component, hostnameGroupingKey(), endpoint, gatherer); err != nil {
			logrus.WithField("component", component).WithError(err).Error("Failed to push metrics.")
		}
	}, interval)
}
