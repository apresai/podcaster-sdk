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

## v0.4.0 — 2026-04-15

### Added

- **`GenerateParams.AllowProviderSwap`**, **`EstimateParams.AllowProviderSwap`**,
  **`EstimateResponse.AllowProviderSwap`** — boolean fields, default
  `false`. When `true`, re-enables the server's legacy best-effort TTS
  behavior: pre-emptive `gemini → vertex-express` upgrade for `long` /
  `deep` durations, and mid-run fallback to a sibling provider on quota
  exhaustion or persistent empty-audio errors. Leave unset (the new
  default) for strict voice-identity pinning — the server uses the
  requested provider end-to-end and surfaces quota failures as typed
  errors you can handle.
- **`APIError.Code`**, **`APIError.Provider`**, **`APIError.ResetsAt`** —
  new fields on the existing error type. Populated for classified
  server-side failures (today only `Code == "quota_exhausted"`). The
  `APIError.Error()` string now includes the reset timestamp when one
  is known.
- **`IsQuotaError(err) bool`** — reports whether err is a typed
  quota-exhausted error. Use this in retry logic to SKIP quota errors
  (retrying daily quotas within the same day just wastes attempts).
- **`QuotaResetsAt(err) time.Time`** — extracts the quota reset
  timestamp from a typed quota error. Returns the zero value for
  non-quota errors or when the server could not calculate a reset time
  (e.g. ElevenLabs monthly cap).
- **`Podcast.ErrorCode`**, **`Podcast.ErrorProvider`**,
  **`Podcast.QuotaResetsAt`**, **`Podcast.RetryAfterSeconds`** — new
  fields populated on failed podcasts when the server classified the
  failure. Most callers should prefer `IsQuotaError(err)` on the error
  from `WaitForCompletion`; these fields are the raw values for
  callers that use `GetPodcast` directly.

### Server changes reflected in this release

These are changes on the production REST API
(`podcasts.apresai.dev/api/v1`) that landed alongside this SDK release.
They are live for every caller, regardless of SDK version — listing
them here so you know the server contract you're talking to.

- **Strict TTS provider pinning is now the default.** Previously the
  server auto-upgraded `gemini → vertex-express` when you picked a
  `long` or `deep` duration on gemini (to dodge Gemini AI Studio's
  100-request/day RPD ceiling). It also silently fell back to a sibling
  provider mid-run when the active provider hit its quota or returned
  persistent empty audio. Neither of those happens by default anymore
  — the server uses the requested provider end-to-end and fails loudly
  on quota errors. Set `AllowProviderSwap: true` on your
  `GenerateParams` to restore the old behavior (see Upgrade from
  v0.3.0 below).
- **Pre-flight quota check on `POST /api/v1/podcasts`.** When the
  requested provider + duration combination is detectable-ahead as
  exceeding the provider's daily quota (and `AllowProviderSwap` is
  false), the server returns HTTP 429 immediately instead of starting
  the job. No credits are deducted. The response includes
  `error_code: "quota_exhausted"`, `provider`, `resets_at`,
  `retry_after_seconds`, and a `Retry-After` header per RFC 7231. The
  SDK converts this into an `*APIError` with `Code == "quota_exhausted"`
  and a populated `ResetsAt`.
- **`GET /api/v1/podcasts/{id}` now exposes `error_code`,
  `error_provider`, `quota_resets_at`, and `retry_after_seconds`** on
  failed-status responses so mid-job quota failures (pre-flight passed
  but TTS hit quota during synthesis) are distinguishable from generic
  failures. The SDK's `WaitForCompletion` wraps these into the same
  typed `*APIError` you get from `Generate`.
- **Voice-identity consistency is the contract.** Same voice name
  (`Kore`) on Gemini AI Studio and Cloud TTS Chirp 3 HD is produced by
  different synthesis engines and sounds audibly different. The strict
  default protects callers doing brand voices or multi-episode series
  from silent mid-episode drift.

### Upgrade from v0.3.0

**No code change is required** — all new fields default to zero/false
which matches the server's new strict default. Existing code compiles
unchanged. However, your runtime behavior WILL change if you previously
relied on `gemini + long` or `gemini + deep` working through the
server's auto-upgrade. Those combinations will now fail with a quota
error, and your app needs to handle the new `*APIError` shape.

**Required action:** audit every `client.Generate` / `client.WaitForCompletion`
call site and decide whether each one wants strict pinning (the new
default) or best-effort fallback (opt in via `AllowProviderSwap: true`).

Mechanical handling pattern for every call site:

```go
job, err := client.Generate(ctx, params)
if err != nil {
    if podcaster.IsQuotaError(err) {
        resetsAt := podcaster.QuotaResetsAt(err)
        // Surface to user — DO NOT retry automatically. The quota is
        // daily; retrying within the same day just burns attempts.
        fmt.Printf("quota exhausted; resets at %s\n", resetsAt.Local())
        return
    }
    // Other errors keep the old shape — handle as before.
    return fmt.Errorf("generate: %w", err)
}
```

Apply the same `IsQuotaError(err)` check to errors from
`WaitForCompletion` and `GetPodcast`.

#### Agent upgrade checklist

1. Run `grep -rn 'podcaster\.GenerateParams' <user-project>`. For each
   call site, check the `TTS` and `Duration` fields:
   - **`TTS: "gemini"` combined with `Duration: podcaster.DurationLong`
     or `DurationDeep`** — ask the user whether this code needs strict
     voice pinning (new default, may fail with quota) or the old
     auto-upgrade behavior. If they want the old behavior, add
     `AllowProviderSwap: true` to the struct literal. If they want
     strict pinning, leave it unset but make sure the error handler
     checks `IsQuotaError(err)` and surfaces the reset time to the user.
   - **Any other combination** — no action required.
2. Run `grep -rn 'client\.Generate\b' <user-project>` and
   `grep -rn 'WaitForCompletion' <user-project>`. Every error handler
   that currently does `if err != nil` or
   `var apiErr *podcaster.APIError` needs an `IsQuotaError(err)` branch
   BEFORE any retry logic. Retrying on quota errors is strictly worse
   than failing loudly — the quota is daily, so the retry will just
   hit the same wall until midnight Pacific.
3. Run `grep -rn 'errors\.As.*APIError' <user-project>`. If the user is
   already switching on `apiErr.StatusCode`, suggest adding a case for
   `apiErr.Code == "quota_exhausted"` that surfaces
   `apiErr.ResetsAt.Local()` to the user.
4. For SDK callers that use `client.EstimatePodcast(...)` and read the
   `EffectiveTTS` / `TTSUpgraded` / `FallbackChain` fields: these are
   still emitted, but when `AllowProviderSwap` is false (the default)
   `EffectiveTTS == RequestedTTS`, `TTSUpgraded == false`, and
   `FallbackChain` is empty to match what the runtime will actually
   do. If a UI showed "Will use vertex-express instead of gemini," that
   banner will stop rendering — verify that's the desired behavior, or
   pass `AllowProviderSwap: true` on the estimate AND generate calls
   together.
5. Run `go build ./... && go vet ./...` to verify.

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
