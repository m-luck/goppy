package crawler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type Crawler struct {
	maxWorkers   int
	maxDepth     int
	crawlDelay   time.Duration
	userAgent    string
	httpClient   *http.Client
	visitedURLs  *sync.Map
	urlsToCrawl  chan crawlTask
	results      chan CrawlResult
	wg           sync.WaitGroup
	robotsMap    *sync.Map // Maps domain to *RobotRules
}

type CrawlResult struct {
	URL   string
	Links []string
	Error error
}

type crawlTask struct {
	URL   string
	Depth int
}

func NewCrawler(maxWorkers, maxDepth int, crawlDelay time.Duration) *Crawler {
	return &Crawler{
		maxWorkers:  maxWorkers,
		maxDepth:    maxDepth,
		crawlDelay:  crawlDelay,
		userAgent:   "GoCrawler/1.0",
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		visitedURLs: &sync.Map{},
		urlsToCrawl: make(chan crawlTask, 1000),
		results:     make(chan CrawlResult, 1000),
		robotsMap:   &sync.Map{},
	}
}

func (c *Crawler) Start(ctx context.Context, startURL string) <-chan CrawlResult {
	// Start worker goroutines
	for i := 0; i < c.maxWorkers; i++ {
		c.wg.Add(1)
		go c.worker(ctx)
	}

	// Start the crawling process
	go func() {
		c.urlsToCrawl <- crawlTask{URL: startURL, Depth: 0}
		c.wg.Wait()
		close(c.results)
	}()

	return c.results
}

func (c *Crawler) worker(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-c.urlsToCrawl:
			if !ok {
				return
			}

			// Respect crawl delay
			time.Sleep(c.crawlDelay)

			// Process the URL
			links, err := c.processURL(task.URL)


			// Send result
			c.results <- CrawlResult{
				URL:   task.URL,
				Links: links,
				Error: err,
			}

			// Queue up new URLs if we haven't reached max depth
			if task.Depth < c.maxDepth && err == nil {
				c.queueLinks(task.URL, links, task.Depth+1)
			}
		}
	}
}

func (c *Crawler) processURL(urlStr string) ([]string, error) {
	// Check if we've already visited this URL
	if _, loaded := c.visitedURLs.LoadOrStore(urlStr, struct{}{}); loaded {
		return nil, nil
	}

	// Parse the URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %s: %v", urlStr, err)
	}

	// Check robots.txt rules
	robotsRules, err := c.getRobotsRules(parsedURL)
	if err != nil {
		return nil, fmt.Errorf("error getting robots.txt rules: %v", err)
	}

	// Check if this URL is allowed by robots.txt
	if !robotsRules.IsAllowed(urlStr) {
		return nil, fmt.Errorf("disallowed by robots.txt: %s", urlStr)
	}

	// Respect crawl delay
	robotsRules.Wait()

	// Set User-Agent header
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	// Fetch the URL
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching %s: %v", urlStr, err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d for %s", resp.StatusCode, urlStr)
	}

	// Only process HTML content
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return nil, nil
	}

	// Parse the HTML to extract links
	return extractLinks(resp.Body, urlStr)
}

// UserAgent returns the User-Agent string used by the crawler
func (c *Crawler) UserAgent() string {
	return c.userAgent
}

// VisitedCount returns the number of unique URLs visited by the crawler
func (c *Crawler) VisitedCount() int {
	count := 0
	c.visitedURLs.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func (c *Crawler) queueLinks(baseURL string, links []string, depth int) {
	for _, link := range links {
		// Convert relative URLs to absolute
		absURL, err := resolveURL(baseURL, link)
		if err != nil {
			continue
		}

		// Skip non-http(s) URLs
		if absURL.Scheme != "http" && absURL.Scheme != "https" {
			continue
		}

		// Queue the URL for crawling
		select {
		case c.urlsToCrawl <- crawlTask{URL: absURL.String(), Depth: depth}:
		default:
			log.Printf("Warning: URL queue full, dropping %s", absURL)
		}
	}
}

func extractLinks(body io.Reader, baseURL string) ([]string, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	var links []string
	var f func(*html.Node)

	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					links = append(links, a.Val)
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)
	return links, nil
}

// getRobotsRules fetches and caches robots.txt rules for a domain
func (c *Crawler) getRobotsRules(parsedURL *url.URL) (*RobotRules, error) {
	// Use the host as the cache key
	host := parsedURL.Hostname()
	if host == "" {
		return nil, fmt.Errorf("invalid host in URL: %s", parsedURL.String())
	}

	// Check if we already have rules for this domain
	if rules, ok := c.robotsMap.Load(host); ok {
		return rules.(*RobotRules), nil
	}

	// Create new rules with default values
	rules := NewRobotRules(c.userAgent)

	// Try to fetch robots.txt
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", parsedURL.Scheme, host)
	req, err := http.NewRequest("GET", robotsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating robots.txt request: %v", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// If we can't fetch robots.txt, allow crawling but with default settings
		c.robotsMap.Store(host, rules)
		return rules, nil
	}
	defer resp.Body.Close()

	// Only parse if we got a successful response
	if resp.StatusCode == http.StatusOK {
		content, err := io.ReadAll(resp.Body)
		if err == nil {
			rules.Parse(robotsURL, string(content))
		}
	}

	// Cache the rules (even if empty or failed to parse)
	c.robotsMap.Store(host, rules)
	return rules, nil
}

// resolveURL converts a relative URL to an absolute URL
func resolveURL(base, rel string) (*url.URL, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %v", err)
	}

	relURL, err := url.Parse(rel)
	if err != nil {
		return nil, fmt.Errorf("invalid relative URL: %v", err)
	}

	return baseURL.ResolveReference(relURL), nil
}
