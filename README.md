# Podcaster Go SDK

Go client library for the [Podcaster REST API](https://podcasts.apresai.dev/api/v1).

Generate podcasts from text or URLs, poll for completion, and download audio — all from your Go application.

## Installation

```bash
go get github.com/apresai/podcaster-sdk
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    podcaster "github.com/apresai/podcaster-sdk"
)

func main() {
    client := podcaster.NewClient("pk_your_api_key")
    ctx := context.Background()

    // Generate a podcast from a URL
    job, err := client.Generate(ctx, podcaster.GenerateParams{
        InputURL: "https://example.com/article",
        Duration: podcaster.DurationStandard,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Wait for completion (auto-polls every 5 seconds)
    podcast, err := client.WaitForCompletion(ctx, job.ID, nil)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Ready: %s — %s\n", podcast.Title, podcast.AudioURL)

    // Download the MP3
    err = client.DownloadToFile(ctx, podcast.ID, "episode.mp3")
    if err != nil {
        log.Fatal(err)
    }
}
```

## Categories

Categories are pre-configured templates that bundle domain-specific personas, prompts, and research instructions.

```go
// List available categories
cats, _ := client.ListCategories(ctx)

// Use a category (applies custom personas + research instructions)
job, _ := client.Generate(ctx, podcaster.GenerateParams{
    InputURL: "https://wineblog.example.com/bordeaux-2025",
    Category: "wine-food-blog", // sommelier + food writer hosts, food critic citations
    Duration: podcaster.DurationStandard,
})
```

### Available Categories

| Slug | Name | Description |
|------|------|-------------|
| `wine-food-blog` | Wine & Food Blog | Sommelier + food writer hosts, food critic citations |

## API Reference

### Client

```go
// Create a client
client := podcaster.NewClient("pk_...",
    podcaster.WithBaseURL("https://custom-endpoint.com/api/v1"),
    podcaster.WithHTTPClient(&http.Client{Timeout: 60*time.Second}),
)
```

### Generate

```go
job, err := client.Generate(ctx, podcaster.GenerateParams{
    InputURL:   "https://...",      // or InputText: "raw content..."
    Category:   "wine-food-blog",   // optional
    Model:      "sonnet",           // haiku, sonnet, gemini-flash, gemini-pro
    TTS:        "gemini",           // gemini, elevenlabs, google
    Tone:       podcaster.ToneCasual,
    Duration:   podcaster.DurationStandard,
    Format:     podcaster.FormatConversation,
    Voices:     2,
    Topic:      "focus on pairings",
    Style:      "humor,storytelling",
    Visibility: podcaster.VisibilityPublic, // defaults to private if omitted
})
```

> **Note:** SDK callers get **private** podcasts by default. Pass `Visibility: podcaster.VisibilityPublic` to list on the public feed at `podcasts.apresai.dev`.

### Poll & Wait

```go
// Get status once
podcast, err := client.GetPodcast(ctx, "podcast-id")

// Auto-poll until complete
podcast, err := client.WaitForCompletion(ctx, "podcast-id", &podcaster.WaitOptions{
    PollInterval: 3 * time.Second,
    Timeout:      15 * time.Minute,
    OnProgress: func(p podcaster.Podcast) {
        fmt.Printf("[%s] %d%%\n", p.Status, p.ProgressPercent)
    },
})
```

### Download

```go
// Download to memory
audioData, err := client.Download(ctx, "podcast-id")

// Download to file
err := client.DownloadToFile(ctx, "podcast-id", "episode.mp3")
```

### List Podcasts

```go
list, err := client.ListPodcasts(ctx, 10, "") // limit, cursor
for _, p := range list.Podcasts {
    fmt.Printf("%s — %s (%s)\n", p.ID, p.Title, p.Status)
}
// Paginate
if list.NextCursor != "" {
    nextPage, _ := client.ListPodcasts(ctx, 10, list.NextCursor)
}
```

### Voices

```go
voices, err := client.ListVoices(ctx, "gemini")
for _, v := range voices {
    fmt.Printf("%s — %s (%s)\n", v.ID, v.Name, v.Gender)
}
```

## Error Handling

All HTTP errors surface as `*podcaster.APIError`. For `POST /podcasts`:

| Status | Meaning |
|--------|---------|
| `401` | Invalid or missing API key |
| `402` | Out of credits (upgrade your plan) |
| `422` | Invalid input — bad URL, extraction failed, validation error |
| `502` | Infrastructure error — transient, safe to retry |

```go
job, err := client.Generate(ctx, params)
if err != nil {
    var apiErr *podcaster.APIError
    if errors.As(err, &apiErr) {
        switch apiErr.StatusCode {
        case 422:
            fmt.Printf("bad input: %s\n", apiErr.Message)
        case 502:
            fmt.Printf("transient, retrying: %s\n", apiErr.Message)
        default:
            fmt.Printf("API error %d: %s\n", apiErr.StatusCode, apiErr.Message)
        }
    }
}
```

## License

MIT
