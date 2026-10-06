package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestKeepAwakeRequestsURL(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/healthz" || r.Header.Get("User-Agent") != "plimsoll-indexer-keepawake" {
			t.Errorf("unexpected request %s %q", r.URL.Path, r.Header.Get("User-Agent"))
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	if err := NewKeepAwake(srv.URL + "/healthz").Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d", hits.Load())
	}
}

func TestKeepAwakeFailsOnErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	if err := NewKeepAwake(srv.URL).Run(context.Background()); err == nil {
		t.Fatal("expected an error for 503")
	}
}

func TestKeepAwakeFailsWhenUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	if err := NewKeepAwake(url).Run(context.Background()); err == nil {
		t.Fatal("expected an error for a closed server")
	}
}
