# Contributing to Plimsoll indexer

## Picking up an issue

1. Find an open issue (`good first issue` for a first pull request).
2. Comment to be assigned and wait for assignment.
3. Ask in the issue if anything is unclear.

## Local setup

```bash
go run ./cmd/devdb &
cp .env.example .env && set -a && . ./.env && set +a
go run ./cmd/plimsoll-indexer
```

## Pull request checklist

- [ ] One change per pull request, linked to its issue.
- [ ] `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass.
- [ ] New behaviour has tests. HTTP and Horizon interactions are tested with `httptest`.
- [ ] Schema changes are additive and idempotent (`IF NOT EXISTS`) in `internal/store/schema.sql`.
- [ ] Conventional Commits: `feat(api): …`, `fix(poster): …`.

## Standards

- Wrap errors with context: `fmt.Errorf("load account: %w", err)`. Never drop an error silently.
- Log with `log/slog`, structured key/value pairs, no string concatenation.
- Amounts are `*big.Int` from Horizon to Postgres (`NUMERIC(40,0)`) to JSON (strings). No floats.
- The canonical breakdown encoding is part of the protocol. Changing `horizon.Breakdown` changes every future hash and needs an issue first.
- Secrets come from the environment only.

## AI-assisted contributions

Allowed if you understand and have tested every line. Untested generated code is closed.
