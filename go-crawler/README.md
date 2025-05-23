# Go Concurrent Web Crawler

A concurrent web crawler written in Go that demonstrates the use of goroutines, channels, and worker pools, with a web-based user interface.

## Features

- Web-based UI for easy interaction
- Real-time crawling progress updates
- Concurrent URL fetching with configurable number of workers
- Respects `robots.txt` (basic support)
- Configurable crawl depth and delay between requests
- Graceful shutdown on interrupt signals
- Avoids duplicate URL visits
- Extracts and follows links from HTML pages

## Installation

1. Make sure you have Go installed (version 1.21 or later)
2. Clone this repository
3. Build and run the web server:
   ```bash
   cd go-crawler
   go run ./cmd/api
   ```
   The web interface will be available at http://localhost:8080

Or build the binary:
   ```bash
   go build -o gocrawler ./cmd/api
   ./gocrawler
   ```

## Usage

## Web Interface

The web interface provides an easy way to configure and monitor crawls:

1. Enter the starting URL
2. Configure crawl parameters:
   - **Depth**: How many levels deep to crawl (default: 2)
   - **Workers**: Number of concurrent workers (default: 5)
   - **Delay (ms)**: Delay between requests in milliseconds (default: 100)
3. Click "Start Crawl" to begin
4. View real-time results in the output panel

## Command Line Usage

You can also use the crawler from the command line:

```bash
go run ./cmd/api -port 8080 -workers 5 -depth 2 -delay 100ms
```

Then open http://localhost:8080 in your browser.

### Command Line Options for API Server

- `-port`: Port to run the server on (default: 8080)
- `-workers`: Default number of concurrent workers (default: 5)
- `-depth`: Default maximum crawl depth (default: 2)
- `-delay`: Default delay between requests (default: 100ms)
- `-timeout`: Maximum crawl time (default: 30s)

## Example Output

```
$ ./crawler -depth 1 -workers 3 https://example.com
Crawled: https://example.com/
  Found 5 links
Crawled: https://www.iana.org/domains/example
  Found 10 links
Crawled: https://www.iana.org/domains/reserved
  Found 15 links
...
Crawling completed!
```

## How It Works

1. The crawler starts with a seed URL and creates a pool of worker goroutines.
2. Each worker picks up URLs from a shared queue and processes them.
3. For each URL, the worker:
   - Fetches the page content
   - Extracts all links
   - Sends the results to the output channel
   - Queues new links for crawling (if within depth limit)
4. The main goroutine prints the results as they come in.
5. The crawler respects the specified delay between requests to be polite to servers.

## License

MIT
