// Package podcaster provides a Go SDK client for the Podcaster REST API.
//
// The SDK lets third-party Go applications generate podcasts from URLs or text,
// poll for completion, and download the resulting audio files.
//
// Quick start:
//
//	client := podcaster.NewClient("pk_your_api_key")
//
//	// Start generation
//	job, err := client.Generate(ctx, podcaster.GenerateParams{
//	    InputURL: "https://example.com/article",
//	    Category: "wine-food-blog",
//	    Duration: DurationStandard,
//	})
//
//	// Wait for completion (polls automatically)
//	podcast, err := client.WaitForCompletion(ctx, job.ID, nil)
//
//	// Download the MP3
//	audioData, err := client.Download(ctx, podcast.ID)
package podcaster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the production Podcaster REST API endpoint.
	DefaultBaseURL = "https://podcasts.apresai.dev/api/v1"

	// DefaultPollInterval is the default interval between status polls.
	DefaultPollInterval = 5 * time.Second

	// DefaultTimeout is the default timeout for WaitForCompletion.
	DefaultTimeout = 10 * time.Minute
)

// Duration presets for episode length.
const (
	DurationShort    = "short"    // ~3-4 minutes
	DurationStandard = "standard" // ~8-10 minutes
	DurationLong     = "long"     // ~15 minutes
	DurationDeep     = "deep"     // ~30-35 minutes
)

// Format presets for show style.
const (
	FormatConversation = "conversation"
	FormatInterview    = "interview"
	FormatDeepDive     = "deep-dive"
	FormatExplainer    = "explainer"
	FormatDebate       = "debate"
	FormatNews         = "news"
	FormatStorytelling = "storytelling"
	FormatChallenger   = "challenger"
	FormatNewscast     = "newscast"
)

// Tone presets for conversation style.
const (
	ToneCasual      = "casual"
	ToneTechnical   = "technical"
	ToneEducational = "educational"
)

// Visibility controls who can see the podcast.
const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)

// Client is a Podcaster REST API client.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithBaseURL overrides the default API base URL.
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(url, "/")
	}
}

// WithHTTPClient provides a custom http.Client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// NewClient creates a new Podcaster API client.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// --- Generate ---

// GenerateParams holds parameters for starting podcast generation.
type GenerateParams struct {
	InputURL       string  `json:"input_url,omitempty"`
	InputText      string  `json:"input_text,omitempty"`
	Category       string  `json:"category,omitempty"`
	Model          string  `json:"model,omitempty"`
	TTS            string  `json:"tts,omitempty"`
	Tone           string  `json:"tone,omitempty"`
	Duration       string  `json:"duration,omitempty"`
	Format         string  `json:"format,omitempty"`
	Voices         int     `json:"voices,omitempty"`
	Topic          string  `json:"topic,omitempty"`
	Style          string  `json:"style,omitempty"`
	Voice1         string  `json:"voice1,omitempty"`
	Voice2         string  `json:"voice2,omitempty"`
	Voice3         string  `json:"voice3,omitempty"`
	TTSModel       string  `json:"tts_model,omitempty"`
	TTSSpeed       float64 `json:"tts_speed,omitempty"`
	TTSPitch       float64 `json:"tts_pitch,omitempty"`
	TTSTemperature float64 `json:"tts_temperature,omitempty"`
	Visibility     string  `json:"visibility,omitempty"`
}

// GenerateResponse is returned when a generation job is started.
type GenerateResponse struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	Message          string `json:"message"`
	PollURL          string `json:"poll_url"`
	EstimatedMinutes int    `json:"estimated_minutes"`
}

// Generate starts a podcast generation job and returns immediately.
// Use GetPodcast or WaitForCompletion to check on the result.
//
// If params.Visibility is empty the podcast defaults to private. Pass
// VisibilityPublic explicitly to list the podcast on the public feed.
//
// Returns an *APIError with StatusCode 422 when the server rejects the
// input (bad URL, validation failure) and 502 for infrastructure errors.
func (c *Client) Generate(ctx context.Context, params GenerateParams) (*GenerateResponse, error) {
	var resp GenerateResponse
	if err := c.post(ctx, "/podcasts", params, &resp); err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	return &resp, nil
}

// --- Get / List / Download ---

// Podcast represents a podcast's status and metadata.
type Podcast struct {
	ID              string    `json:"id"`
	Status          string    `json:"status"`
	Title           string    `json:"title,omitempty"`
	Summary         string    `json:"summary,omitempty"`
	AudioURL        string    `json:"audio_url,omitempty"`
	DownloadURL     string    `json:"download_url,omitempty"`
	ScriptURL       string    `json:"script_url,omitempty"`
	Duration        string    `json:"duration,omitempty"`
	FileSizeMB      float64   `json:"file_size_mb,omitempty"`
	ProgressPercent int       `json:"progress_percent,omitempty"`
	StageMessage    string    `json:"stage_message,omitempty"`
	Model           string    `json:"model,omitempty"`
	TTSProvider     string    `json:"tts_provider,omitempty"`
	Format          string    `json:"format,omitempty"`
	Tone            string    `json:"tone,omitempty"`
	Style           string    `json:"style,omitempty"`
	Topic           string    `json:"topic,omitempty"`
	Voices          int       `json:"voices,omitempty"`
	Voice1          string    `json:"voice1,omitempty"`
	Voice2          string    `json:"voice2,omitempty"`
	Voice3          string    `json:"voice3,omitempty"`
	TTSModel        string    `json:"tts_model,omitempty"`
	Visibility      string    `json:"visibility,omitempty"`
	Category        string    `json:"category,omitempty"`
	Citations       []Citation `json:"citations,omitempty"`
	PlayCount       int       `json:"play_count,omitempty"`
	CreatedAt       string    `json:"created_at,omitempty"`
	Error           string    `json:"error,omitempty"`

	// Typed failure metadata — populated when Status == "failed" and the
	// server recognized the failure class. Today only ErrorCode ==
	// "quota_exhausted" is emitted. Use IsQuotaError(err) on errors
	// returned from WaitForCompletion instead of parsing these fields by
	// hand unless you need the reset timestamp directly.
	ErrorCode         string `json:"error_code,omitempty"`
	ErrorProvider     string `json:"error_provider,omitempty"`
	QuotaResetsAt     string `json:"quota_resets_at,omitempty"`
	RetryAfterSeconds int64  `json:"retry_after_seconds,omitempty"`
}

// Citation is a reference to an expert, critic, or publication cited in the podcast.
type Citation struct {
	Critic  string `json:"critic"`
	Source  string `json:"source"`
	Context string `json:"context"`
	Quote   string `json:"quote,omitempty"`
}

// PodcastList is a paginated list of podcasts.
type PodcastList struct {
	Podcasts   []Podcast `json:"podcasts"`
	Count      int       `json:"count"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

// GetPodcast retrieves the status and details of a podcast by ID.
func (c *Client) GetPodcast(ctx context.Context, id string) (*Podcast, error) {
	var podcast Podcast
	if err := c.get(ctx, "/podcasts/"+id, nil, &podcast); err != nil {
		return nil, fmt.Errorf("get podcast: %w", err)
	}
	return &podcast, nil
}

// ListPodcasts returns a paginated list of podcasts.
func (c *Client) ListPodcasts(ctx context.Context, limit int, cursor string) (*PodcastList, error) {
	params := url.Values{}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	var list PodcastList
	if err := c.get(ctx, "/podcasts", params, &list); err != nil {
		return nil, fmt.Errorf("list podcasts: %w", err)
	}
	return &list, nil
}

// Download fetches the podcast MP3 audio data.
// The returned byte slice contains the raw MP3 file contents.
func (c *Client) Download(ctx context.Context, id string) ([]byte, error) {
	// First get the podcast to find the audio URL
	podcast, err := c.GetPodcast(ctx, id)
	if err != nil {
		return nil, err
	}
	if podcast.Status != "complete" {
		return nil, fmt.Errorf("podcast is not ready (status: %s)", podcast.Status)
	}
	if podcast.AudioURL == "" {
		return nil, fmt.Errorf("podcast has no audio URL")
	}

	// Download the audio from the CDN
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, podcast.AudioURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read audio data: %w", err)
	}
	return data, nil
}

// DownloadToFile downloads the podcast MP3 and writes it to a local file.
func (c *Client) DownloadToFile(ctx context.Context, id, path string) error {
	data, err := c.Download(ctx, id)
	if err != nil {
		return err
	}
	return writeFile(path, data)
}

// --- Polling ---

// WaitOptions configures the WaitForCompletion behavior.
type WaitOptions struct {
	// PollInterval is the duration between status polls. Default: 5s.
	PollInterval time.Duration
	// Timeout is the maximum duration to wait. Default: 10m.
	Timeout time.Duration
	// OnProgress is called after each poll with the current status.
	OnProgress func(Podcast)
}

// WaitForCompletion polls a podcast until it reaches 'complete' or 'failed' status.
// Returns the final Podcast on success. Returns an error if the podcast fails or the timeout expires.
func (c *Client) WaitForCompletion(ctx context.Context, id string, opts *WaitOptions) (*Podcast, error) {
	pollInterval := DefaultPollInterval
	timeout := DefaultTimeout
	var onProgress func(Podcast)

	if opts != nil {
		if opts.PollInterval > 0 {
			pollInterval = opts.PollInterval
		}
		if opts.Timeout > 0 {
			timeout = opts.Timeout
		}
		onProgress = opts.OnProgress
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		podcast, err := c.GetPodcast(ctx, id)
		if err != nil {
			return nil, err
		}

		if onProgress != nil {
			onProgress(*podcast)
		}

		switch podcast.Status {
		case "complete":
			return podcast, nil
		case "failed":
			errMsg := podcast.Error
			if errMsg == "" {
				errMsg = "unknown error"
			}
			// Quota failures surface as a typed *APIError so callers can
			// branch retry logic via IsQuotaError / QuotaResetsAt without
			// parsing the Error string. Other failures keep the original
			// plain-error shape for backward compatibility.
			if podcast.ErrorCode == "quota_exhausted" {
				apiErr := &APIError{
					StatusCode: 200, // GET succeeded; the podcast failed
					Message:    errMsg,
					Code:       podcast.ErrorCode,
					Provider:   podcast.ErrorProvider,
				}
				if podcast.QuotaResetsAt != "" {
					if t, err := time.Parse(time.RFC3339, podcast.QuotaResetsAt); err == nil {
						apiErr.ResetsAt = t
					}
				}
				return nil, apiErr
			}
			return nil, fmt.Errorf("podcast generation failed: %s", errMsg)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for podcast %s (last status: %s)", id, podcast.Status)
		case <-ticker.C:
			// continue polling
		}
	}
}

// --- Categories ---

// Category represents a podcast category template.
type Category struct {
	Slug        string           `json:"slug"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Defaults    CategoryDefaults `json:"defaults"`
	Tags        []string         `json:"tags"`
}

// CategoryDefaults are the generation defaults for a category.
type CategoryDefaults struct {
	Format   string   `json:"format"`
	Tone     string   `json:"tone"`
	Duration string   `json:"duration"`
	Styles   []string `json:"styles"`
	Voices   int      `json:"voices"`
}

// ListCategories returns all available podcast categories.
func (c *Client) ListCategories(ctx context.Context) ([]Category, error) {
	var resp struct {
		Categories []Category `json:"categories"`
	}
	if err := c.get(ctx, "/categories", nil, &resp); err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return resp.Categories, nil
}

// GetCategory returns details for a specific category.
func (c *Client) GetCategory(ctx context.Context, slug string) (*Category, error) {
	var cat Category
	if err := c.get(ctx, "/categories/"+slug, nil, &cat); err != nil {
		return nil, fmt.Errorf("get category: %w", err)
	}
	return &cat, nil
}

// --- Voices ---

// Voice represents a TTS voice.
type Voice struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Gender      string `json:"gender"`
	Description string `json:"description"`
	HasSample   bool   `json:"has_sample"`
	DefaultFor  string `json:"default_for,omitempty"`
}

// ListVoices returns available TTS voices for a provider.
func (c *Client) ListVoices(ctx context.Context, provider string) ([]Voice, error) {
	params := url.Values{"provider": {provider}}
	var resp struct {
		Voices []Voice `json:"voices"`
	}
	if err := c.get(ctx, "/voices", params, &resp); err != nil {
		return nil, fmt.Errorf("list voices: %w", err)
	}
	return resp.Voices, nil
}

// --- Estimate / Limits ---

// EstimateParams holds parameters for a preflight podcast estimate. Mirrors
// GenerateParams but neither input field is required — the estimate is
// independent of content. BYOK API keys, when present, set credit_cost to 0
// and bypass_credits to true on the response.
type EstimateParams struct {
	Duration     string `json:"duration,omitempty"`
	Format       string `json:"format,omitempty"`
	TTS          string `json:"tts,omitempty"`
	TTSModel     string `json:"tts_model,omitempty"`
	GeminiAPIKey string `json:"gemini_api_key,omitempty"`
}

// EstimateQuota describes the static daily quota for a TTS provider.
type EstimateQuota struct {
	Provider    string `json:"provider"`
	DailyLimit  int    `json:"daily_limit"`
	WouldExceed bool   `json:"would_exceed"`
	Note        string `json:"note,omitempty"`
}

// EstimateResponse is the result of a preflight podcast estimate.
//
// `CanAfford` is true when `BypassCredits` is set OR the user's
// `CreditBalance` is at least `CreditCost`. `RequestedTTS` echoes the
// provider the run will use end-to-end — the server pins the requested
// provider with no auto-upgrade or fallback.
type EstimateResponse struct {
	RequestedTTS       string        `json:"requested_tts"`
	Duration           string        `json:"duration"`
	Format             string        `json:"format"`
	SegmentEstimate    int           `json:"segment_estimate"`
	MinuteEstimate     int           `json:"minute_estimate"`
	CreditCost         int           `json:"credit_cost"`
	CreditBalance      int           `json:"credit_balance"`
	CreditBalanceAfter int           `json:"credit_balance_after"`
	CanAfford          bool          `json:"can_afford"`
	BypassCredits      bool          `json:"bypass_credits"`
	Quota              EstimateQuota `json:"quota"`
	Warnings           []string      `json:"warnings"`
}

// EstimatePodcast returns a preflight estimate for a podcast generation
// without starting a job or spending credits. Use this before Generate to
// preview the credit cost, segment count, expected runtime, and any
// quota warnings.
//
// Estimate and Generate share the same Go primitives on the server, so the
// estimate is structurally guaranteed to match what Generate will actually do
// for the same parameters (assuming the Lambda's environment doesn't change
// between calls).
func (c *Client) EstimatePodcast(ctx context.Context, params EstimateParams) (*EstimateResponse, error) {
	var resp EstimateResponse
	if err := c.post(ctx, "/podcasts/estimate", params, &resp); err != nil {
		return nil, fmt.Errorf("estimate: %w", err)
	}
	return &resp, nil
}

// ProviderLimits describes the static rate-limit configuration for a single
// TTS provider as the pipeline runs it.
type ProviderLimits struct {
	Name             string `json:"name"`
	Concurrency      int    `json:"concurrency"`
	InterDelayMs     int64  `json:"inter_delay_ms"`
	DailyQuota       int    `json:"daily_quota"`
	IsBatchCapable   bool   `json:"is_batch_capable"`
	CreditMultiplier int    `json:"credit_multiplier"`
}

// DurationInfo describes the credit cost and runtime estimate for a duration.
type DurationInfo struct {
	Name            string `json:"name"`
	Credits         int    `json:"credits"`
	SegmentEstimate int    `json:"segment_estimate"`
	MinuteEstimate  int    `json:"minute_estimate"`
}

// SubscriptionInfo describes the user's current Stripe subscription state.
type SubscriptionInfo struct {
	Plan            string `json:"plan,omitempty"`
	Status          string `json:"status,omitempty"`
	CreditsPerCycle int    `json:"credits_per_cycle,omitempty"`
	LastRefill      string `json:"last_refill,omitempty"`
}

// LimitsResponse is the result of GetLimits.
type LimitsResponse struct {
	CreditBalance int              `json:"credit_balance"`
	Subscription  SubscriptionInfo `json:"subscription"`
	Providers     []ProviderLimits `json:"providers"`
	Durations     []DurationInfo   `json:"durations"`
}

// GetLimits returns the authenticated user's credit balance and subscription
// state, plus the static rate-limit table for every TTS provider and the
// credit cost for every duration. Read-only, no side effects.
func (c *Client) GetLimits(ctx context.Context) (*LimitsResponse, error) {
	var resp LimitsResponse
	if err := c.get(ctx, "/limits", nil, &resp); err != nil {
		return nil, fmt.Errorf("get limits: %w", err)
	}
	return &resp, nil
}

// --- HTTP helpers ---

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return c.doRequest(req, out)
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doRequest(req, out)
}

func (c *Client) doRequest(req *http.Request, out any) error {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// Handle non-success status codes
	if resp.StatusCode >= 400 {
		// Typed error body: includes machine-readable error_code and, for
		// quota failures, the provider + reset timestamp. New in v0.4.0.
		var apiErr struct {
			Error             string `json:"error"`
			Status            int    `json:"status"`
			ErrorCode         string `json:"error_code"`
			Provider          string `json:"provider"`
			ResetsAt          string `json:"resets_at"`
			RetryAfterSeconds int64  `json:"retry_after_seconds"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
			e := &APIError{
				StatusCode: resp.StatusCode,
				Message:    apiErr.Error,
				Code:       apiErr.ErrorCode,
				Provider:   apiErr.Provider,
			}
			if apiErr.ResetsAt != "" {
				if t, err := time.Parse(time.RFC3339, apiErr.ResetsAt); err == nil {
					e.ResetsAt = t
				}
			}
			return e
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}
	}

	// Handle redirects (302 for downloads) — shouldn't normally reach here
	// because http.Client follows redirects, but just in case.

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
	}
	return nil
}

// APIError is returned when the API responds with a non-success status code.
//
// For classified server-side failures (today: quota exhaustion), Code is a
// stable machine-readable identifier and Provider / ResetsAt are populated
// so callers can make retry decisions without parsing the Message string.
// Use IsQuotaError / QuotaResetsAt instead of switching on Code by hand.
type APIError struct {
	StatusCode int
	Message    string

	// Code is a stable machine-readable error identifier. Populated for
	// classified failures (today only "quota_exhausted"); empty for
	// generic errors. New in v0.4.0.
	Code string

	// Provider carries the TTS provider that caused the failure for
	// quota errors. Zero-value otherwise.
	Provider string

	// ResetsAt is the timestamp at which the provider's daily quota
	// resets, populated for Code == "quota_exhausted" on Google-family
	// providers (gemini, google, vertex-express, gemini-vertex reset at
	// 00:00 America/Los_Angeles). Zero-value for generic errors or
	// providers without a calculable reset time.
	ResetsAt time.Time
}

func (e *APIError) Error() string {
	if e.Code == "quota_exhausted" && !e.ResetsAt.IsZero() {
		return fmt.Sprintf("podcaster API error (HTTP %d, %s): %s (resets at %s)",
			e.StatusCode, e.Code, e.Message, e.ResetsAt.Format(time.RFC3339))
	}
	if e.Code != "" {
		return fmt.Sprintf("podcaster API error (HTTP %d, %s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("podcaster API error (HTTP %d): %s", e.StatusCode, e.Message)
}

// IsQuotaError returns true when err is an *APIError with
// Code == "quota_exhausted". Use this in retry logic — quota errors
// should NEVER be retried automatically, because the quota is daily
// and retrying within the same window just burns attempts.
func IsQuotaError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == "quota_exhausted"
}

// QuotaResetsAt extracts the reset timestamp from a quota error. Returns
// the zero value when err is not a quota error or when the server could
// not determine the reset time. Call this on errors from Generate,
// GetPodcast, or WaitForCompletion to decide when to tell the user they
// can try again.
func QuotaResetsAt(err error) time.Time {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == "quota_exhausted" {
		return apiErr.ResetsAt
	}
	return time.Time{}
}
