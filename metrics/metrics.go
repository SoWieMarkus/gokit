// Package metrics provides the Prometheus setup every service shares: a
// registry with Go, process, build info and database pool collectors plus
// HTTP request metrics. Applications embed *Registry and register their
// business metrics on it with MustRegister.
package metrics

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry is a Prometheus registry with the shared collectors attached.
type Registry struct {
	*prometheus.Registry
	namespace string

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	httpInFlight prometheus.Gauge
}

// New creates a registry whose metrics are prefixed with namespace. db may be
// nil, which is handy in tests.
func New(namespace, version string, db *sql.DB) *Registry {
	reg := prometheus.NewRegistry()
	r := &Registry{
		Registry:  reg,
		namespace: namespace,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "http", Name: "requests_total",
			Help: "HTTP requests by route, method and status code.",
		}, []string{"route", "method", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Subsystem: "http", Name: "request_duration_seconds",
			Help:    "HTTP request latency by route and method.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method"}),
		httpInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: "http", Name: "requests_in_flight",
			Help: "HTTP requests currently being served.",
		}),
	}
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: namespace, Name: "build_info",
			Help:        "Build information, always 1.",
			ConstLabels: prometheus.Labels{"version": version},
		}, func() float64 { return 1 }),
		r.httpRequests, r.httpDuration, r.httpInFlight,
	)
	if db != nil {
		reg.MustRegister(collectors.NewDBStatsCollector(db, namespace))
	}
	return r
}

// Namespace is the metric name prefix passed to New. Use it for the
// application's own metrics.
func (r *Registry) Namespace() string {
	return r.namespace
}

// Handler serves the metrics in the Prometheus exposition format.
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.Registry, promhttp.HandlerOpts{Registry: r.Registry})
}

// Middleware records request metrics. It labels by chi route pattern instead
// of raw path to keep cardinality bounded.
func (r *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		r.httpInFlight.Inc()
		defer r.httpInFlight.Dec()

		ww := middleware.NewWrapResponseWriter(w, req.ProtoMajor)
		next.ServeHTTP(ww, req)

		route := "unmatched"
		if rc := chi.RouteContext(req.Context()); rc != nil && rc.RoutePattern() != "" {
			route = rc.RoutePattern()
		}
		r.httpRequests.WithLabelValues(route, req.Method, strconv.Itoa(ww.Status())).Inc()
		r.httpDuration.WithLabelValues(route, req.Method).Observe(time.Since(start).Seconds())
	})
}
