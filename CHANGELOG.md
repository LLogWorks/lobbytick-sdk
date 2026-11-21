# Changelog

All three SDKs are versioned independently and released by tag (`typescript/v*`,
`python/v*`, `go/v*`). The format follows [Keep a Changelog](https://keepachangelog.com/)
and the versions [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-11-21

First release of all three SDKs (TypeScript `@lobbytick/gamedata`, Python
`lobbytick-gamedata`, Go `github.com/LLogWorks/lobbytick-sdk/go`).

### Added

- Every route of the public game-data API v1: list games, get a game, get a version
  (`latest` allowed), list languages, list items (`lang`, `q`, `kind`, `limit`, `offset`),
  get an item.
- An iterator over every item that fetches pages as needed and pins the version resolved
  from `latest`.
- Typed models for every response and typed errors carrying the HTTP status and the API's
  error message.
- 429 handling: `retryAfter` from `Retry-After` / `RateLimit-Reset`, optional automatic
  retries with a cap and a documented backoff.
- Helpers for an item's icon URL and its sprite rectangle in the version's atlas.
- Configurable base URL, timeout, HTTP client / fetch / transport and User-Agent.
