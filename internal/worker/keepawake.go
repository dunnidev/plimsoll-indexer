package worker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// KeepAwake requests the service's own public URL so hosts that sleep idle
// services (Render's free tier sleeps after ~15 minutes without inbound
// traffic) keep this one, and its supply poster, running.
type KeepAwake struct {
	URL  string
	HTTP *http.Client
}

func NewKeepAwake(url string) *KeepAwake {
	return &KeepAwake{URL: url, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// Run makes one request and fails on any non-2xx answer.
func (k *KeepAwake) Run(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "plimsoll-indexer-keepawake")
	resp, err := k.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("keep-awake request: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("keep-awake %s: status %d", k.URL, resp.StatusCode)
	}
	return nil
}
