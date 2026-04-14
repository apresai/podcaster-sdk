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

## v0.3.0 — 2026-04-14

### Added

- **`Client.EstimatePodcast(ctx, params)`** — preflight estimate for a
  podcast generation without starting a job or spending credits. Returns the
  effective TTS provider after auto-upgrade rules, the credit cost, the
  segment count estimate, the expected runtime, the fallback chain, and any
  quota warnings. Use this before `Generate` to preview what a generation
  will cost and which provider will actually run it.
- **`Client.GetLimits(ctx)`** — returns the authenticated user's credit
  balance and subscription state, plus the static rate-limit table for every
  TTS provider and the credit cost for every duration. Read-only, no side
  effects. Use this when an app needs to render a "what does each plan/duration
  cost" comparison or check the user's remaining credits.
- New types: `EstimateParams`, `EstimateResponse`, `EstimateQuota`,
  `LimitsResponse`, `ProviderLimits`, `DurationInfo`, `SubscriptionInfo`.

### Server changes reflected in this release

These are changes on the production REST API (`podcasts.apresai.dev/api/v1`)
that landed alongside this SDK release. They are live for every caller,
regardless of SDK version.

- New endpoint `POST /api/v1/podcasts/estimate` — preflight estimate.
- New endpoint `GET /api/v1/limits` — credit balance, subscription state,
  provider rate-limit table, duration cost table.
- TTS retry resilience: per-minute `429` rate-limit responses on Vertex AI
  now retry across a 3-minute wall-clock budget (8 attempts, 60s max
  backoff) and fall over to the next provider in the chain
  (`vertex-express → gemini-vertex → polly`) instead of failing the whole
  job. Previously, the 5-attempt / 30s budget couldn't outlast a per-minute
  quota window. This is invisible to SDK consumers — no API change — but
  jobs that previously failed mid-synthesis will now complete.
- Granular progress reporting during batch TTS: `Podcast.ProgressPercent`
  now updates every ~8 seconds during the `20→90` window even when the
  pipeline runs in batch mode (vertex-express, gemini-vertex on long/deep
  durations). Previously the bar froze at 20 until the whole batch
  finished. The interpolation is time-based with a `0.85` saturation cap,
  so the real `0.90` "TTS complete" event always lands as a forward jump.
- Auto-upgrade `gemini → vertex-express` for `long`/`deep` durations now
  surfaces in the estimate response as `tts_upgraded: true` with a
  human-readable `upgrade_reason`.

### No upgrade required

This release is purely additive. Existing code calling `Generate`,
`GetPodcast`, `WaitForCompletion`, etc. continues to work unchanged. There
is no `Upgrade from v0.2.0` block because there are no breaking changes.

#### For AI agents introducing the new methods to existing code

If a user asks you to "use the new estimate API" or "show the user what a
generation will cost before they run it," the pattern is:

```go
est, err := client.EstimatePodcast(ctx, podcaster.EstimateParams{
    Duration: podcaster.DurationStandard,
    TTS:      "gemini",
    Format:   podcaster.FormatConversation,
})
if err != nil {
    return err
}
if est.TTSUpgraded {
    fmt.Printf("Will use %s instead of %s: %s\n",
        est.EffectiveTTS, est.RequestedTTS, est.UpgradeReason)
}
if !est.CanAfford {
    fmt.Printf("Insufficient credits: need %d, have %d\n",
        est.CreditCost, est.CreditBalance)
    return nil
}
// then call client.Generate(...) with the same params
```

Always run `est, err := client.EstimatePodcast(...)` BEFORE `client.Generate(...)`
when the user wants a credit-cost or provider preview. Do not duplicate the
upgrade logic client-side — the server is the authoritative source.

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
