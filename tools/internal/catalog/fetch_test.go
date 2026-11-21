package catalog

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchRetriesTransientFailures(t *testing.T) {
	fetchBackoff = time.Millisecond
	for _, c := range []struct {
		name  string
		first func(http.ResponseWriter)
	}{
		{"connection reset", func(w http.ResponseWriter) {
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
		}},
		{"server error", func(w http.ResponseWriter) { w.WriteHeader(http.StatusBadGateway) }},
		{"rate limit", func(w http.ResponseWriter) { w.WriteHeader(http.StatusTooManyRequests) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					c.first(w)
					return
				}
				w.Write([]byte("ok"))
			}))
			defer srv.Close()
			data, err := Fetch(srv.URL)
			if err != nil || string(data) != "ok" {
				t.Fatalf("Fetch = %q, %v; want ok", data, err)
			}
			if calls.Load() != 2 {
				t.Errorf("server saw %d requests, want 2", calls.Load())
			}
		})
	}
}

func TestFetchDoesNotRetryClientErrors(t *testing.T) {
	fetchBackoff = time.Millisecond
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	if _, err := Fetch(srv.URL); err == nil {
		t.Fatal("Fetch accepted a 404")
	}
	if calls.Load() != 1 {
		t.Errorf("server saw %d requests, want 1", calls.Load())
	}
}

func TestFetchGivesUp(t *testing.T) {
	fetchBackoff = time.Millisecond
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	if _, err := Fetch(srv.URL); err == nil {
		t.Fatal("Fetch succeeded against a failing server")
	}
	if calls.Load() != fetchAttempts {
		t.Errorf("server saw %d requests, want %d", calls.Load(), fetchAttempts)
	}
}
