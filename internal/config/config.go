// Package config loads indexer settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	TestnetRPC        = "https://soroban-testnet.stellar.org"
	TestnetHorizon    = "https://horizon-testnet.stellar.org"
	TestnetPassphrase = "Test SDF Network ; September 2015"
)

type Config struct {
	DatabaseURL        string
	Port               string
	RPCURL             string
	HorizonURL         string
	NetworkPassphrase  string
	CoverageLedgerID   string
	ReporterRegistryID string
	StartLedger        uint32
	PosterSecret       string
	IngestInterval     time.Duration
	PostInterval       time.Duration
	RepostAfter        time.Duration
	TomlInterval       time.Duration
	CORSOrigins        []string
	LogLevel           string
}

// Load reads the environment. Missing required values are reported together.
func Load() (Config, error) {
	c := Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		Port:               envOr("PORT", "8080"),
		RPCURL:             envOr("STELLAR_RPC_URL", TestnetRPC),
		HorizonURL:         strings.TrimRight(envOr("HORIZON_URL", TestnetHorizon), "/"),
		NetworkPassphrase:  envOr("STELLAR_NETWORK_PASSPHRASE", TestnetPassphrase),
		CoverageLedgerID:   os.Getenv("COVERAGE_LEDGER_ID"),
		ReporterRegistryID: os.Getenv("REPORTER_REGISTRY_ID"),
		PosterSecret:       os.Getenv("SUPPLY_POSTER_SECRET"),
		LogLevel:           envOr("LOG_LEVEL", "info"),
	}

	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.CoverageLedgerID == "" {
		errs = append(errs, errors.New("COVERAGE_LEDGER_ID is required"))
	}
	if c.ReporterRegistryID == "" {
		errs = append(errs, errors.New("REPORTER_REGISTRY_ID is required"))
	}

	start, err := strconv.ParseUint(envOr("START_LEDGER", "0"), 10, 32)
	if err != nil {
		errs = append(errs, fmt.Errorf("START_LEDGER: %w", err))
	}
	c.StartLedger = uint32(start)

	durations := []struct {
		name string
		def  string
		dst  *time.Duration
	}{
		{"INGEST_INTERVAL", "10s", &c.IngestInterval},
		{"POST_INTERVAL", "15m", &c.PostInterval},
		{"REPOST_AFTER", "6h", &c.RepostAfter},
		{"TOML_INTERVAL", "6h", &c.TomlInterval},
	}
	for _, d := range durations {
		v, err := time.ParseDuration(envOr(d.name, d.def))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.name, err))
			continue
		}
		*d.dst = v
	}

	for _, o := range strings.Split(envOr("CORS_ORIGINS", "*"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, o)
		}
	}

	return c, errors.Join(errs...)
}

// PostingEnabled reports whether this instance should post supply snapshots.
func (c Config) PostingEnabled() bool { return c.PosterSecret != "" }

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
