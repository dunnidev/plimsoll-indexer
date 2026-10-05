<p align="center">
  <img src="https://raw.githubusercontent.com/dunnidev/plimsoll-contracts/main/docs/banner.svg" alt="Plimsoll" width="100%" />
</p>

<p align="center">
  <a href="https://github.com/dunnidev/plimsoll-indexer/actions/workflows/ci.yml"><img src="https://github.com/dunnidev/plimsoll-indexer/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <img src="https://img.shields.io/badge/go-1.27-00ADD8" alt="Go" />
  <img src="https://img.shields.io/badge/postgres-16%2B-336791" alt="Postgres" />
  <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Apache-2.0" />
</p>

# Plimsoll indexer

The off-chain half of [Plimsoll](https://github.com/dunnidev/plimsoll-contracts).
One Go binary with three jobs and an HTTP API:

| Job | What it does |
| --- | --- |
| **Supply poster** | For every listed asset, sums Horizon's per-bucket holdings (trustlines in every auth state, claimable balances, liquidity pools, contract balances) and calls `post_supply` on the coverage ledger with the total and the SHA-256 of the breakdown. Posts when supply changes, or every `REPOST_AFTER` so freshness checks keep passing. |
| **Event ingester** | Reads `asset_listed`, `supply_posted`, `reserve_posted`, `reporter_set` and `reporter_revoked` from Soroban RPC into Postgres, keeping history past RPC's ~7-day retention. |
| **stellar.toml sync** | Resolves each issuer's `home_domain`, reads its SEP-1 `stellar.toml`, and stores the matching `[[CURRENCIES]]` entry, including `attestation_of_reserve`. |

| Repo | What it is |
| --- | --- |
| [plimsoll-contracts](https://github.com/dunnidev/plimsoll-contracts) | Rust/Soroban contracts |
| [plimsoll-app](https://github.com/dunnidev/plimsoll-app) | Web app and TypeScript SDK |
| **plimsoll-indexer** (this repo) | This service |

## Maintainers

| Maintainer | GitHub | Contact |
| --- | --- | --- |
| dunnidev | [@dunnidev](https://github.com/dunnidev) | [GitHub Discussions](https://github.com/dunnidev/plimsoll-indexer/discussions) |

## Verifiable by design

The poster is trusted to count, so its counting is checkable. The breakdown it
hashed is served byte-for-byte at `/v1/breakdowns/{hash}`; `sha256` of that
body equals the `breakdown_hash` in the on-chain `supply_posted` event, and each
bucket can be recomputed from Horizon's `/assets` endpoint.

## API

| Method and path | Returns |
| --- | --- |
| `GET /healthz` | `{status, ingested_to_ledger}` |
| `GET /v1/network` | Contract ids, RPC and Horizon URLs |
| `GET /v1/assets?min_tier=1` | Every asset with latest supply, latest report per tier, coverage and stellar.toml data |
| `GET /v1/assets/{sac}?min_tier=1` | One asset, same shape |
| `GET /v1/assets/{sac}/supply?limit=100` | Supply snapshot history |
| `GET /v1/assets/{sac}/reports?limit=100` | Reserve report history |
| `GET /v1/reporters` | Active registered reporters |
| `GET /v1/breakdowns/{hash}` | Exact bytes of a supply breakdown |

Coverage in the API uses the same selection and rounding rules as the contract
(`internal/coverage`), so the API and `coverage()` on-chain agree.

## Quick start

Go 1.27+. No Postgres or Docker needed locally:

```bash
git clone https://github.com/dunnidev/plimsoll-indexer
cd plimsoll-indexer
go run ./cmd/devdb &          # embedded Postgres on :54329
cp .env.example .env          # defaults point at the testnet deployment
set -a; . ./.env; set +a
go run ./cmd/plimsoll-indexer
curl localhost:8080/v1/assets
```

Leave `SUPPLY_POSTER_SECRET` empty to run read-only. To post supply, use the
secret of an account registered in the reporter registry with the
`SupplyPoster` role.

```bash
go test ./...
```

## Configuration

Every variable is documented in [.env.example](.env.example). Required:
`DATABASE_URL`, `COVERAGE_LEDGER_ID`, `REPORTER_REGISTRY_ID`.

## Deploying

[render.yaml](render.yaml) defines a Render web service plus a Postgres
database in the same region, connected over the internal URL. Set
`SUPPLY_POSTER_SECRET` in the Render dashboard; it is never committed.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Security: [SECURITY.md](SECURITY.md).

## Contributors

<a href="https://github.com/dunnidev/plimsoll-indexer/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=dunnidev/plimsoll-indexer" alt="Contributors" />
</a>

## License

[Apache-2.0](LICENSE)
