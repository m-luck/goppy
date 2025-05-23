package crawler

import (
	"bufio"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type RobotRules struct {
	disallowedPaths []*regexp.Regexp
	crawlDelay     time.Duration
	lastAccess     time.Time
	userAgent      string
}

func NewRobotRules(userAgent string) *RobotRules {
	return &RobotRules{
		disallowedPaths: make([]*regexp.Regexp, 0),
		crawlDelay:     time.Second, // Default delay
		userAgent:      userAgent,
	}
}

func (r *RobotRules) Parse(robotsURL string, content string) error {
	// Reset existing rules
	r.disallowedPaths = make([]*regexp.Regexp, 0)
	r.crawlDelay = time.Second // Reset to default

	scanner := bufio.NewScanner(strings.NewReader(content))
	userAgentMatch := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split into field and value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		field := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])


		// Check if this is a User-agent line
		if field == "user-agent" {
			// Check if it matches our user agent or is the wildcard
			userAgentMatch = value == "*" || strings.Contains(strings.ToLower(r.userAgent), strings.ToLower(value))
			continue
		}

		// Only process rules that apply to our user agent
		if !userAgentMatch {
			continue
		}

		switch field {
		case "disallow":
			if value == "" {
				continue // Empty disallow means allow all
			}
			// Convert the path to a regex pattern
			pattern := "^" + regexp.QuoteMeta(value)
			pattern = strings.ReplaceAll(pattern, "\\*", ".*") // Handle wildcards
			re, err := regexp.Compile(pattern)
			if err == nil {
				r.disallowedPaths = append(r.disallowedPaths, re)
			}

		case "crawl-delay":
			var seconds int
			_, err := fmt.Sscanf(value, "%d", &seconds)
			if err == nil && seconds > 0 {
				r.crawlDelay = time.Duration(seconds) * time.Second
			}
		}
	}

	return scanner.Err()
}

// IsAllowed checks if a URL is allowed to be crawled based on robots.txt rules
func (r *RobotRules) IsAllowed(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	path := parsedURL.Path
	if !strings.HasSuffix(path, "/") && !strings.Contains(parsedURL.Path, ".") {
		path += "/"
	}

	for _, re := range r.disallowedPaths {
		if re.MatchString(path) {
			return false
		}
	}
	return true
}

// GetCrawlDelay returns the required delay between requests
func (r *RobotRules) GetCrawlDelay() time.Duration {
	return r.crawlDelay
}

// Wait enforces the crawl delay between requests
func (r *RobotRules) Wait() {
	now := time.Now()
	elapsed := now.Sub(r.lastAccess)

	if elapsed < r.crawlDelay {
		sleepTime := r.crawlDelay - elapsed
		time.Sleep(sleepTime)
	}

	r.lastAccess = time.Now()
}
