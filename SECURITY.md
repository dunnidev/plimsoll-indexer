# Security policy

The indexer is unaudited and runs against Stellar testnet.

## Reporting

Report privately via
[Security → Report a vulnerability](https://github.com/dunnidev/plimsoll-indexer/security/advisories/new).
Include the component, impact and reproduction steps. No public issues for
vulnerabilities, please. We aim to respond within 72 hours.

## Scope

In scope: anything that could make the poster sign or post a wrong supply
figure, leak `SUPPLY_POSTER_SECRET`, serve a breakdown that does not match its
hash, SQL injection, SSRF through stellar.toml fetching, and API responses that
disagree with on-chain `coverage()`.

Out of scope: RPC and Horizon availability, and reserve figures posted by
third-party reporters.

## Operational notes

- The poster key should hold only enough XLM for fees and have no other role.
- stellar.toml fetches are limited to 100KB with a 10-second timeout.
