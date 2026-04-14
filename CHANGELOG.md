# Changelog

All notable changes to `github.com/apresai/podcaster-sdk` are documented here.

This project follows [Semantic Versioning](https://semver.org/). Breaking
changes bump the minor version while the SDK is pre-1.0.

> **For AI coding assistants:** when a user asks you to upgrade this SDK, read
> every **"Upgrade from"** block between their current version and the target
> version. Each block lists exact code transformations — treat them as
> mechanical rewrites, not suggestions. Apply them in order, then run
> `go build ./...` and `go vet ./...` to verify. If a block says "ask the user,"
> pause and ask before making that change.

---

## v0.2.0 — 2026-04-13

### Changed

- **`Podcast.ProgressPercent` is now `int` (was `float64`).** The server
  now serializes progress as an integer `0–100` instead of a float `0.0–1.0`.
  The SDK field type matches the new wire contract.
- `Generate()` godoc now documents the `422` / `502` error contract and
  the private-by-default visibility for SDK callers.

### Added

- README error handling section now lists every `APIError.StatusCode`
  returned by `POST /podcasts` (`401`, `402`, `422`, `502`) with a switch
  example.
- README Generate example passes `Visibility: podcaster.VisibilityPublic`
  explicitly and calls out the new private-by-default behavior.

### Server changes reflected in this release

These are changes on the production REST API (`podcasts.apresai.dev/api/v1`)
that landed between v0.1.0 and v0.2.0. They are live for every caller,
regardless of SDK version — listing them here so you know the server
contract you're talking to.

- `progress_percent` field type flipped from float `0.0–1.0` to integer `0–100`.
- `POST /podcasts` now returns `422` for input errors (bad URL, extraction
  failure, validation) instead of `202` with an unparseable null body.
- `POST /podcasts` now returns `502` (instead of a bogus `202`) when the
  MCP response is missing a `podcast_id`.
- SDK / API callers who omit `visibility` now get **private** podcasts by
  default. Portal users are unaffected — the portal explicitly passes
  `visibility=public`. Records created before this change continue to
  resolve to `public` via server-side `effectiveVisibility()`.
- `GET /podcasts` (list) is now scoped to the authenticated caller instead
  of returning every user's public podcasts.
- `GET /voices` is now served directly from the REST API Lambda and cached
  at CloudFront for 24h (provider is the only cache key).

### Upgrade from v0.1.0

**Breaking:** `Podcast.ProgressPercent` changed type and value range.

| Before (v0.1.0) | After (v0.2.0) |
|---|---|
| `float64` in range `0.0–1.0` | `int` in range `0–100` |
| `fmt.Printf("%.0f%%", p.ProgressPercent*100)` | `fmt.Printf("%d%%", p.ProgressPercent)` |
| `int(p.ProgressPercent * 100)` | `p.ProgressPercent` |
| `if p.ProgressPercent >= 1.0` | `if p.ProgressPercent >= 100` |
| `if p.ProgressPercent > 0.5` | `if p.ProgressPercent > 50` |

**Non-code behavior changes to verify:**

- **Private-by-default.** If your app never passed `Visibility`, new podcasts
  created after you upgrade will be private and won't show up on the public
  feed. Add `Visibility: podcaster.VisibilityPublic` to preserve old behavior.
- **422 errors.** Previously, bad input (unreachable URL, extraction failure)
  could surface as a confusing "202 with null id" response. Now you'll get
  a proper `*APIError` with `StatusCode == 422` and a descriptive
  `Message`. If you have retry logic, make sure you do **not** retry 422s
  (they're deterministic) but **do** retry 502s (transient infra errors).

#### Agent upgrade checklist

1. Run `grep -rn 'ProgressPercent' <user-project>`. Every match is a
   candidate for rewrite.
2. For each match, check the surrounding code:
   - `* 100` or `*100` — **delete** the multiplication.
   - `%.Nf` or `%f` format verb in the same `Printf`/`Sprintf` — change
     to `%d`.
   - Float comparisons (`>= 1.0`, `> 0.5`, `== 0.0`) — convert to `100`,
     `50`, `0`.
   - Assigned into a `float64` variable or struct field — change the
     receiving type to `int`.
3. Run `go build ./... && go vet ./...`. `%d`-vs-`%f` mismatches surface
   immediately as vet warnings.
4. `grep -rn 'podcaster\.GenerateParams' <user-project>`. For every
   construction site that does **not** set `Visibility`, **ask the user**
   whether they want the new private default or want to preserve public
   behavior with `Visibility: podcaster.VisibilityPublic`. Do not make
   this change silently — it's a product decision, not a mechanical one.
5. `grep -rn 'APIError' <user-project>`. If the user's error handling
   only checks for non-nil `err`, suggest adding a `StatusCode` switch
   so they can distinguish 422 (don't retry, surface to user) from 502
   (retry with backoff).

---

## v0.1.0 — 2026-04-12

Initial public release. Go client for the Podcaster REST API.

### Added

- `Client` with `NewClient`, `WithBaseURL`, `WithHTTPClient` options.
- Generation: `Generate`, `GetPodcast`, `ListPodcasts`, `Download`,
  `DownloadToFile`, `WaitForCompletion` (with polling and `OnProgress`).
- Discovery: `ListCategories`, `GetCategory`, `ListVoices`.
- Typed `APIError` wrapping HTTP status code and message.
- Constants for `Duration*`, `Format*`, `Tone*`, `Visibility*`.
