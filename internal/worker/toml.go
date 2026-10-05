package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dunnidev/plimsoll-indexer/internal/horizon"
	"github.com/dunnidev/plimsoll-indexer/internal/stellartoml"
	"github.com/dunnidev/plimsoll-indexer/internal/store"
)

// TomlSync records what each issuer's stellar.toml says about its asset,
// including the SEP-1 attestation_of_reserve link.
type TomlSync struct {
	Horizon *horizon.Client
	Fetcher *stellartoml.Fetcher
	Store   *store.Store
	// MaxAge is how long a lookup result is kept before it is refreshed.
	MaxAge time.Duration
	Log    *slog.Logger
}

func (t *TomlSync) Run(ctx context.Context) error {
	assets, err := t.Store.ListAssets(ctx)
	if err != nil {
		return fmt.Errorf("list assets: %w", err)
	}
	now := time.Now().UTC()
	for _, a := range assets {
		if a.TomlCheckedAt != nil && now.Sub(*a.TomlCheckedAt) < t.MaxAge {
			continue
		}
		info, lookupErr := t.lookup(ctx, a.Code, a.Issuer)
		var body []byte
		msg := ""
		if lookupErr != nil {
			msg = lookupErr.Error()
		}
		// Keep whatever we learned (e.g. home_domain) even on partial failure.
		if info.HomeDomain != "" || lookupErr == nil {
			body, _ = json.Marshal(info)
		}
		if err := t.Store.SaveToml(ctx, a.SAC, body, msg, now); err != nil {
			return fmt.Errorf("save toml for %s: %w", a.Code, err)
		}
		if lookupErr != nil {
			t.Log.Debug("stellar.toml lookup failed", "code", a.Code, "issuer", a.Issuer, "err", lookupErr)
		}
	}
	return nil
}

func (t *TomlSync) lookup(ctx context.Context, code, issuer string) (stellartoml.Info, error) {
	domain, err := t.Horizon.HomeDomain(ctx, issuer)
	if err != nil {
		return stellartoml.Info{}, err
	}
	return t.Fetcher.Fetch(ctx, domain, code, issuer)
}
