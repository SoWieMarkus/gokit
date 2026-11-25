package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/SoWieMarkus/gokit/apperr"
)

func init() {
	RegisterRule("even", func(fl validator.FieldLevel) bool {
		return fl.Field().Int()%2 == 0
	}, "must be even")
}

type tagBody struct {
	Name string `json:"name" validate:"required"`
}

type body struct {
	Name     string    `json:"name"     validate:"required,max=5"`
	Username string    `json:"username" validate:"omitempty,username"`
	Number   int       `json:"number"   validate:"even"`
	Tags     []tagBody `json:"tags"     validate:"max=2,dive"`
}

func decode(t *testing.T, contentType, raw string) (body, *httptest.ResponseRecorder) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(raw))
	r.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	b, err := Decode[body](w, r)
	if err != nil {
		Error(w, r, err)
	}
	return b, w
}

func problem(t *testing.T, w *httptest.ResponseRecorder) Problem {
	t.Helper()
	var p Problem
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	return p
}

func TestDecode(t *testing.T) {
	t.Parallel()

	b, w := decode(t, "application/json", `{"name":"abc"}`)
	if b.Name != "abc" || w.Body.Len() != 0 {
		t.Fatalf("valid body rejected: %s", w.Body)
	}

	cases := []struct {
		name, contentType, raw, code string
		status                       int
	}{
		{"wrong content type", "text/plain", `{}`, "unsupported_media_type", http.StatusUnsupportedMediaType},
		{"empty", "application/json", ``, "invalid_body", http.StatusBadRequest},
		{"unknown field", "application/json", `{"name":"a","x":1}`, "invalid_body", http.StatusBadRequest},
		{"two objects", "application/json", `{"name":"a"}{}`, "invalid_body", http.StatusBadRequest},
		{"validation", "application/json", `{"name":"toolong"}`, "validation_failed", http.StatusBadRequest},
		{"username rule", "application/json", `{"name":"a","username":"no spaces"}`, "validation_failed", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, w := decode(t, tc.contentType, tc.raw)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			if p := problem(t, w); p.Code != tc.code {
				t.Fatalf("code = %s, want %s", p.Code, tc.code)
			}
		})
	}
}

func TestFieldErrors(t *testing.T) {
	t.Parallel()
	_, w := decode(t, "application/json", `{"name":"a","number":3,"tags":[{"name":""}]}`)
	got := map[string]string{}
	for _, fe := range problem(t, w).Errors {
		got[fe.Field] = fe.Message
	}
	want := map[string]string{
		"number":       "must be even",
		"tags[0].name": "is required",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("field errors = %v, want %v", got, want)
	}

	_, w = decode(t, "application/json", `{"name":"a","tags":[{"name":"x"},{"name":"y"},{"name":"z"}]}`)
	if fe := problem(t, w).Errors; len(fe) != 1 || fe[0].Message != "must contain at most 2 entries" {
		t.Fatalf("slice max message = %v", fe)
	}
}

func TestErrorMapping(t *testing.T) {
	t.Parallel()
	cases := map[error]int{
		apperr.New(apperr.KindNotFound, "x", "x"):               http.StatusNotFound,
		apperr.New(apperr.KindConflict, "x", "x"):               http.StatusConflict,
		apperr.New(apperr.KindUnauthenticated, "x", "x"):        http.StatusUnauthorized,
		apperr.New(apperr.KindBadGateway, "x", "x"):             http.StatusBadGateway,
		apperr.New(apperr.KindUnavailable, "x", "x"):            http.StatusServiceUnavailable,
		fmt.Errorf("wrapped: %w", apperr.Invalid("bad", "bad")): http.StatusBadRequest,
		errors.New("database fire"):                             http.StatusInternalServerError,
	}
	for err, status := range cases {
		w := httptest.NewRecorder()
		Error(w, httptest.NewRequest(http.MethodGet, "/", http.NoBody), err)
		if w.Code != status {
			t.Errorf("%v: status = %d, want %d", err, w.Code, status)
		}
		if p := problem(t, w); status == http.StatusInternalServerError && strings.Contains(p.Detail, "fire") {
			t.Errorf("internal error leaked: %s", p.Detail)
		}
	}
}

func TestQueryHelpers(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/?limit=20&tag=1&tag=2&sort=score&bad=x&flag=true", http.NoBody)
	if v, err := QueryInt(r, "limit", 50, 1, 100); err != nil || v != 20 {
		t.Fatalf("QueryInt: %d %v", v, err)
	}
	if _, err := QueryInt(r, "bad", 0, 0, 1); err == nil {
		t.Fatal("expected error for non-integer")
	}
	if ids, err := QueryIDs(r, "tag"); err != nil || len(ids) != 2 {
		t.Fatalf("QueryIDs: %v %v", ids, err)
	}
	if v, err := QueryBool(r, "flag"); err != nil || !v {
		t.Fatalf("QueryBool: %v %v", v, err)
	}
	if v, err := QueryEnum(r, "sort", "newest", "score"); err != nil || v != "score" {
		t.Fatalf("QueryEnum: %s %v", v, err)
	}
	if v, err := QueryEnum(r, "missing", "newest", "score"); err != nil || v != "newest" {
		t.Fatalf("QueryEnum default: %s %v", v, err)
	}
}
