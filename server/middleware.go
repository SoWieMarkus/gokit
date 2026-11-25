// Package server holds the HTTP plumbing every service shares: health and
// readiness endpoints, request logging, security headers, proxy and CSRF
// handling and a ListenAndServe wrapper for graceful shutdown.
package server

import (
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/SoWieMarkus/gokit/apperr"
	"github.com/SoWieMarkus/gokit/httpx"
)

// RequestLogger writes one structured log line per request. Server errors are
// logged as errors, health and readiness probes at debug level.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		level := slog.LevelInfo
		switch {
		case ww.Status() >= http.StatusInternalServerError:
			level = slog.LevelError
		case r.URL.Path == HealthPath || r.URL.Path == ReadyPath:
			level = slog.LevelDebug
		}
		slog.Log(r.Context(), level, "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration", time.Since(start),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}

// SecurityHeaders sets conservative defaults for an API that serves JSON and
// images only.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// AllowScripts removes the Content-Security-Policy set by SecurityHeaders,
// for routes like the swagger UI that need scripts and styles.
func AllowScripts(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Del("Content-Security-Policy")
		next.ServeHTTP(w, r)
	})
}

// RealIPFromProxy replaces RemoteAddr with the X-Real-IP header. Only use it
// behind a reverse proxy that always overwrites that header, otherwise
// clients could pick their own IP.
func RealIPFromProxy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); err == nil {
			r.RemoteAddr = net.JoinHostPort(ip.String(), "0")
		}
		next.ServeHTTP(w, r)
	})
}

// ErrCrossOrigin is returned to browsers that send a state changing request
// from another site.
var ErrCrossOrigin = apperr.New(apperr.KindForbidden, "cross_origin", "cross origin requests are not allowed")

// CrossOriginProtection rejects state changing requests a browser sends from
// another site (CSRF), based on Sec-Fetch-Site and Origin. trustedOrigins
// (scheme://host[:port]) are allowed anyway, for example the website when it
// runs on another host than the API.
func CrossOriginProtection(trustedOrigins ...string) (func(http.Handler) http.Handler, error) {
	cop := http.NewCrossOriginProtection()
	for _, origin := range trustedOrigins {
		if err := cop.AddTrustedOrigin(origin); err != nil {
			return nil, err
		}
	}
	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, ErrCrossOrigin)
	}))
	return cop.Handler, nil
}
