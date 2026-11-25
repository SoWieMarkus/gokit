package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/SoWieMarkus/gokit/metrics"
)

func TestRegistry(t *testing.T) {
	t.Parallel()
	m := metrics.New("app", "1.2.3", nil)
	custom := prometheus.NewCounter(prometheus.CounterOpts{Namespace: m.Namespace(), Name: "things_total", Help: "Things."})
	m.MustRegister(custom)
	custom.Inc()

	r := chi.NewRouter()
	r.Use(m.Middleware)
	r.Get("/items/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items/7", http.NoBody))

	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	out, _ := io.ReadAll(w.Body)
	for _, want := range []string{
		`app_http_requests_total{method="GET",route="/items/{id}",status="418"} 1`,
		`app_build_info{version="1.2.3"} 1`,
		`app_things_total 1`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in output", want)
		}
	}
}
