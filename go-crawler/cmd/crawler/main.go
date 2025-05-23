package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-crawler/internal/crawler"
)

func main() {
	// Parse command line flags
	workers := flag.Int("workers", 5, "Number of concurrent workers")
	maxDepth := flag.Int("depth", 2, "Maximum crawl depth")
	delay := flag.Duration("delay", 100*time.Millisecond, "Delay between requests")
	timeout := flag.Duration("timeout", 30*time.Second, "Maximum crawl time")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("Please provide a starting URL")
	}
	startURL := args[0]

	// Set up context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Handle interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	// Create and start the crawler
	c := crawler.NewCrawler(*workers, *maxDepth, *delay)
	log.Printf("Starting crawler with %d workers, max depth %d, delay %v", *workers, *maxDepth, *delay)
	log.Printf("User-Agent: %s", c.UserAgent()) // Add this line to log the user agent
	results := c.Start(ctx, startURL)

	// Process results
	for result := range results {
		if result.Error != nil {
			log.Printf("Error crawling %s: %v", result.URL, result.Error)
			continue
		}

		fmt.Printf("Crawled: %s\n", result.URL)
		if len(result.Links) > 0 {
			fmt.Printf("  Found %d links\n", len(result.Links))
		}
	}

	fmt.Println("\nCrawling completed!")
}
