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

	// 2. Start a wine & food blog podcast
	job, err := client.Generate(ctx, podcaster.GenerateParams{
		InputURL: "https://example.com/wine-blog-post",
		Category: "wine-food-blog",
		Duration: podcaster.DurationStandard,
	})
	if err != nil {
		log.Fatalf("Generate: %v", err)
	}
	fmt.Printf("Started podcast %s (estimated %d minutes)\n", job.ID, job.EstimatedMinutes)

	// 3. Wait for completion with progress updates
	podcast, err := client.WaitForCompletion(ctx, job.ID, &podcaster.WaitOptions{
		OnProgress: func(p podcaster.Podcast) {
			fmt.Printf("  [%s] %.0f%% — %s\n", p.Status, p.ProgressPercent*100, p.StageMessage)
		},
	})
	if err != nil {
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
