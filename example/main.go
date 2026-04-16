// Example: Generate a podcast from a wine blog URL using the Podcaster Go SDK.
//
// Usage:
//
//	export PODCASTER_API_KEY=pk_your_key_here
//	go run main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	podcaster "github.com/apresai/podcaster-sdk"
)

func main() {
	apiKey := os.Getenv("PODCASTER_API_KEY")
	if apiKey == "" {
		log.Fatal("PODCASTER_API_KEY environment variable is required")
	}

	client := podcaster.NewClient(apiKey)
	ctx := context.Background()

	// 1. List available categories
	cats, err := client.ListCategories(ctx)
	if err != nil {
		log.Fatalf("List categories: %v", err)
	}
	fmt.Printf("Available categories (%d):\n", len(cats))
	for _, c := range cats {
		fmt.Printf("  • %s — %s\n", c.Slug, c.Name)
	}
	fmt.Println()

	// 2. Start a wine & food blog podcast. AllowProviderSwap is left false
	// (default) so the server pins the requested TTS provider end-to-end.
	// If you'd rather have the server silently fall back to a sibling
	// provider on quota / empty-audio errors, set AllowProviderSwap: true
	// — but voices may drift mid-episode because sibling providers use
	// different synthesis engines. Leave it false for brand voices.
	job, err := client.Generate(ctx, podcaster.GenerateParams{
		InputURL: "https://en.wikipedia.org/wiki/Wine",
		Category: "wine-food-blog",
		Duration: podcaster.DurationShort,
	})
	if err != nil {
		// Quota errors (pre-flight 429 or mid-job failure) come back as a
		// typed *APIError with Code == "quota_exhausted" and a ResetsAt
		// timestamp. Don't retry — the quota is daily.
		if podcaster.IsQuotaError(err) {
			resetsAt := podcaster.QuotaResetsAt(err)
			log.Fatalf("TTS quota exhausted; try again after %s (or pick vertex-express, gemini-vertex, google for higher quotas)",
				resetsAt.Local().Format("3:04 PM MST"))
		}
		log.Fatalf("Generate: %v", err)
	}
	fmt.Printf("Started podcast %s (estimated %d minutes)\n", job.ID, job.EstimatedMinutes)

	// 3. Wait for completion with progress updates
	podcast, err := client.WaitForCompletion(ctx, job.ID, &podcaster.WaitOptions{
		OnProgress: func(p podcaster.Podcast) {
			fmt.Printf("  [%s] %d%% — %s\n", p.Status, p.ProgressPercent, p.StageMessage)
		},
	})
	if err != nil {
		// Mid-job quota failures also surface as *APIError from
		// WaitForCompletion. Same branch, same non-retry rule.
		if podcaster.IsQuotaError(err) {
			resetsAt := podcaster.QuotaResetsAt(err)
			log.Fatalf("TTS quota hit mid-job; try again after %s", resetsAt.Local().Format("3:04 PM MST"))
		}
		log.Fatalf("Wait: %v", err)
	}

	fmt.Printf("\nPodcast complete!\n")
	fmt.Printf("  Title:    %s\n", podcast.Title)
	fmt.Printf("  Duration: %s\n", podcast.Duration)
	fmt.Printf("  Audio:    %s\n", podcast.AudioURL)

	// 4. Show citations if present
	if len(podcast.Citations) > 0 {
		fmt.Printf("\nCitations:\n")
		for _, c := range podcast.Citations {
			fmt.Printf("  • %s (%s) — %s\n", c.Critic, c.Source, c.Context)
		}
	}

	// 5. Download the MP3
	outputFile := podcast.ID + ".mp3"
	if err := client.DownloadToFile(ctx, podcast.ID, outputFile); err != nil {
		log.Fatalf("Download: %v", err)
	}
	fmt.Printf("\nSaved to %s\n", outputFile)
}
