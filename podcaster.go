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
	TTSStability   float64 `json:"tts_stability,omitempty"`
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
		var apiErr struct {
			Error  string `json:"error"`
			Status int    `json:"status"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
			return &APIError{
				StatusCode: resp.StatusCode,
				Message:    apiErr.Error,
			}
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
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("podcaster API error (HTTP %d): %s", e.StatusCode, e.Message)
}
