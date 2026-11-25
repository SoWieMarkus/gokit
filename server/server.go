package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/SoWieMarkus/gokit/httpx"
)

// Paths of the probes registered by Mount.
const (
	HealthPath = "/healthz"
	ReadyPath  = "/readyz"
)

// Pinger is anything that can check its connection, like *sql.DB.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// HealthResponse is the body of the health and readiness probes.
type HealthResponse struct {
	Status  string `json:"status"  example:"ok"`
	Version string `json:"version" example:"1.0.0"`
}

// Health reports liveness.
func Health(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, HealthResponse{Status: "ok", Version: version})
	}
}

// Ready reports whether the service can reach its database.
func Ready(version string, db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "database_unavailable", "database is not reachable", nil)
			return
		}
		httpx.JSON(w, http.StatusOK, HealthResponse{Status: "ok", Version: version})
	}
}

// Serve runs srv until it is shut down. A regular shutdown is not an error.
func Serve(srv *http.Server, name, version string) error {
	slog.Info("listening", "server", name, "addr", srv.Addr, "version", version)
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("%s server: %w", name, err)
}

// Shutdown waits for ctx to end and then shuts all servers down gracefully
// within timeout. Run it in the same errgroup as Serve.
func Shutdown(ctx context.Context, timeout time.Duration, servers ...*http.Server) error {
	<-ctx.Done()
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	errs := make([]error, len(servers))
	for i, srv := range servers {
		errs[i] = srv.Shutdown(shutdownCtx)
	}
	return errors.Join(errs...)
}

// MetricsServer serves handler on GET /metrics at addr, separate from the
// public API so it is never exposed by accident.
func MetricsServer(addr string, handler http.Handler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", handler)
	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
