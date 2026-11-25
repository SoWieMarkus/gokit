package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SoWieMarkus/gokit/server"
)

type pinger struct{ err error }

func (p pinger) PingContext(context.Context) error { return p.err }

func TestReady(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		err    error
		status int
	}{
		{nil, http.StatusOK},
		{errors.New("down"), http.StatusServiceUnavailable},
	} {
		w := httptest.NewRecorder()
		server.Ready("1", pinger{tc.err})(w, httptest.NewRequest(http.MethodGet, server.ReadyPath, http.NoBody))
		if w.Code != tc.status {
			t.Errorf("err=%v: status = %d, want %d", tc.err, w.Code, tc.status)
		}
	}
}

func TestCrossOriginProtection(t *testing.T) {
	t.Parallel()
	mw, err := server.CrossOriginProtection("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	for origin, status := range map[string]int{
		"https://example.com": http.StatusNoContent,
		"https://evil.com":    http.StatusForbidden,
	} {
		r := httptest.NewRequest(http.MethodPost, "https://api.example.com/x", http.NoBody)
		r.Header.Set("Origin", origin)
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != status {
			t.Errorf("%s: status = %d, want %d", origin, w.Code, status)
		}
	}
}

func TestRealIPFromProxy(t *testing.T) {
	t.Parallel()
	var got string
	h := server.RealIPFromProxy(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = r.RemoteAddr }))
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.Header.Set("X-Real-IP", " 203.0.113.7 ")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got != "203.0.113.7:0" {
		t.Fatalf("RemoteAddr = %q", got)
	}
}
