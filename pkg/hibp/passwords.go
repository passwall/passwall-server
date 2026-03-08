package hibp

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const pwnedPasswordsEndpoint = "https://api.pwnedpasswords.com/range"

// PwnedResult holds the count for a single SHA-1 hash check.
type PwnedResult struct {
	SHA1Hash string `json:"sha1_hash"`
	Count    int    `json:"count"` // 0 = not found in any breach
}

// rangeEntry stores suffix → count for a cached range prefix.
type rangeEntry struct {
	suffixes map[string]int
	fetched  time.Time
}

// PwnedPasswordsClient checks passwords against HIBP Pwned Passwords (k-anonymity).
// It caches range responses in memory with a configurable TTL.
type PwnedPasswordsClient struct {
	httpClient *http.Client

	mu    sync.RWMutex
	cache map[string]*rangeEntry // prefix (5 hex chars) → range data

	cacheTTL  time.Duration
	maxCache  int
	rateDelay time.Duration

	rateMu   sync.Mutex
	lastCall time.Time
}

// NewPwnedPasswordsClient creates a client for HIBP Pwned Passwords.
// cacheTTLMinutes controls how long range responses are cached (0 = 60 min default).
// maxCacheEntries limits the number of cached prefixes (0 = 10000 default).
func NewPwnedPasswordsClient(cacheTTLMinutes, maxCacheEntries int) *PwnedPasswordsClient {
	if cacheTTLMinutes <= 0 {
		cacheTTLMinutes = 60
	}
	if maxCacheEntries <= 0 {
		maxCacheEntries = 10000
	}
	return &PwnedPasswordsClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache:      make(map[string]*rangeEntry, 256),
		cacheTTL:   time.Duration(cacheTTLMinutes) * time.Minute,
		maxCache:   maxCacheEntries,
		rateDelay:  50 * time.Millisecond, // light rate limit; Pwned Passwords is more generous than breached-account
	}
}

// CheckBatch checks a batch of SHA-1 hashes against HIBP Pwned Passwords.
// Each hash must be uppercase hex (40 chars). Returns results in the same order.
func (c *PwnedPasswordsClient) CheckBatch(hashes []string) ([]PwnedResult, error) {
	// Group hashes by their 5-char prefix to minimize API calls.
	type indexedHash struct {
		idx    int
		suffix string
	}
	prefixGroups := make(map[string][]indexedHash)
	for i, h := range hashes {
		h = strings.ToUpper(strings.TrimSpace(h))
		if len(h) != 40 {
			continue
		}
		prefix := h[:5]
		suffix := h[5:]
		prefixGroups[prefix] = append(prefixGroups[prefix], indexedHash{idx: i, suffix: suffix})
	}

	results := make([]PwnedResult, len(hashes))
	for i, h := range hashes {
		results[i] = PwnedResult{SHA1Hash: strings.ToUpper(strings.TrimSpace(h)), Count: 0}
	}

	for prefix, group := range prefixGroups {
		suffixMap, err := c.getRange(prefix)
		if err != nil {
			return nil, fmt.Errorf("hibp pwned passwords check failed for prefix %s: %w", prefix, err)
		}
		for _, ih := range group {
			if count, ok := suffixMap[ih.suffix]; ok {
				results[ih.idx].Count = count
			}
		}
	}

	return results, nil
}

// getRange fetches (or returns cached) range data for a 5-char hex prefix.
func (c *PwnedPasswordsClient) getRange(prefix string) (map[string]int, error) {
	prefix = strings.ToUpper(prefix)

	// Check cache
	c.mu.RLock()
	if entry, ok := c.cache[prefix]; ok && time.Since(entry.fetched) < c.cacheTTL {
		c.mu.RUnlock()
		return entry.suffixes, nil
	}
	c.mu.RUnlock()

	// Rate limit
	c.rateMu.Lock()
	elapsed := time.Since(c.lastCall)
	if elapsed < c.rateDelay {
		time.Sleep(c.rateDelay - elapsed)
	}
	c.lastCall = time.Now()
	c.rateMu.Unlock()

	// Fetch from HIBP
	req, err := http.NewRequest(http.MethodGet, pwnedPasswordsEndpoint+"/"+prefix, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Add-Padding", "true")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("hibp range %s returned %d: %s", prefix, resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	suffixes := parseRangeResponse(string(bodyBytes))

	// Store in cache (evict oldest if at capacity)
	c.mu.Lock()
	if len(c.cache) >= c.maxCache {
		c.evictOldest()
	}
	c.cache[prefix] = &rangeEntry{suffixes: suffixes, fetched: time.Now()}
	c.mu.Unlock()

	return suffixes, nil
}

// parseRangeResponse parses HIBP range response lines: "SUFFIX:COUNT"
func parseRangeResponse(body string) map[string]int {
	lines := strings.Split(body, "\n")
	suffixes := make(map[string]int, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		suffix := strings.ToUpper(line[:idx])
		countStr := strings.TrimSpace(line[idx+1:])
		count, _ := strconv.Atoi(countStr)
		if count > 0 {
			suffixes[suffix] = count
		}
	}
	return suffixes
}

// evictOldest removes the oldest cache entry. Caller must hold c.mu write lock.
func (c *PwnedPasswordsClient) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, v := range c.cache {
		if first || v.fetched.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.fetched
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.cache, oldestKey)
	}
}

// CacheStats returns cache size for monitoring.
func (c *PwnedPasswordsClient) CacheStats() (size int, capacity int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache), c.maxCache
}
