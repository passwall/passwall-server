package http

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/service"
)

const (
	// Cache durations
	iconCacheDuration    = 7 * 24 * time.Hour // 1 week for successful icons
	iconCacheFailure     = 1 * time.Hour      // 1 hour for failed lookups
	iconRequestTimeout   = 5 * time.Second    // Timeout for upstream requests
	iconMaxSize          = 100 * 1024         // 100KB max icon size
	iconMemoryCacheLimit = 500                // Max icons in memory cache

	// Default storage path
	defaultIconStoragePath = "data/icons"

	// Custom icons subfolder (for manually added icons)
	customIconsSubfolder = "custom"
)

// Allowed origins for icon API (prevent unauthorized usage)
var allowedIconOrigins = []string{
	"passwall.io",
	"www.passwall.io",
	"vault.passwall.io",
	"app.passwall.io",
	"localhost",
	"127.0.0.1",
}

// Icon sources (in priority order)
var iconSources = []string{
	"https://www.google.com/s2/favicons?sz=128&domain_url=%s", // Google (most reliable)
	"https://icons.duckduckgo.com/ip3/%s.ico",                 // DuckDuckGo
}

// IconsHandler serves website icons/favicons
type IconsHandler struct {
	logger      service.Logger
	httpClient  *http.Client
	cache       *iconCache
	storagePath string
}

// iconCache is a simple in-memory cache for icons
type iconCache struct {
	mu    sync.RWMutex
	items map[string]*cachedIcon
	order []string // For LRU eviction
}

type cachedIcon struct {
	data        []byte
	contentType string
	expiresAt   time.Time
	notFound    bool // True if icon wasn't found (negative cache)
}

func newIconCache() *iconCache {
	return &iconCache{
		items: make(map[string]*cachedIcon),
		order: make([]string, 0, iconMemoryCacheLimit),
	}
}

func (c *iconCache) get(domain string) (*cachedIcon, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	icon, ok := c.items[domain]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Now().After(icon.expiresAt) {
		return nil, false
	}

	return icon, true
}

func (c *iconCache) set(domain string, icon *cachedIcon) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if at capacity
	if len(c.items) >= iconMemoryCacheLimit {
		if len(c.order) > 0 {
			oldest := c.order[0]
			delete(c.items, oldest)
			c.order = c.order[1:]
		}
	}

	c.items[domain] = icon
	c.order = append(c.order, domain)
}

// NewIconsHandler creates a new icons handler
func NewIconsHandler(logger service.Logger) *IconsHandler {
	return NewIconsHandlerWithStorage(logger, defaultIconStoragePath)
}

// NewIconsHandlerWithStorage creates a new icons handler with custom storage path
func NewIconsHandlerWithStorage(logger service.Logger, storagePath string) *IconsHandler {
	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		logger.Error("failed to create icon storage directory", "path", storagePath, "error", err)
	}

	// Create custom icons directory
	customPath := filepath.Join(storagePath, customIconsSubfolder)
	if err := os.MkdirAll(customPath, 0755); err != nil {
		logger.Error("failed to create custom icon storage directory", "path", customPath, "error", err)
	}

	return &IconsHandler{
		logger: logger,
		httpClient: &http.Client{
			Timeout: iconRequestTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		cache:       newIconCache(),
		storagePath: storagePath,
	}
}

// domainRegex validates domain format
var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*\.[a-zA-Z]{2,}$`)

// isBrowserExtensionOrigin checks if the origin is from a browser extension
func isBrowserExtensionOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	originLower := strings.ToLower(origin)

	// Browser extension origin patterns
	extensionPatterns := []string{
		"chrome-extension://",
		"moz-extension://",
		"safari-extension://",
		"safari-web-extension://",
		"extension://",
		"ms-browser-extension://", // Edge
	}

	for _, pattern := range extensionPatterns {
		if strings.HasPrefix(originLower, pattern) {
			return true
		}
	}

	return false
}

// isAllowedOrigin checks if the request origin is allowed
func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	// Allow all browser extensions
	if isBrowserExtensionOrigin(origin) {
		return true
	}

	// Parse origin URL to extract host
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	host := strings.ToLower(parsed.Hostname())

	for _, allowed := range allowedIconOrigins {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}

	return false
}

// isAllowedReferer checks if the request referer is from a browser extension or allowed domain
func isAllowedReferer(referer string) bool {
	if referer == "" {
		return false
	}

	// Check for browser extension patterns
	if isBrowserExtensionOrigin(referer) {
		return true
	}

	// Also check if referer is from allowed domains
	return isAllowedOrigin(referer)
}

// IconProtectionMiddleware validates that requests come from Passwall clients
func IconProtectionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		referer := c.GetHeader("Referer")

		// 1. Allow browser extensions (Chrome, Firefox, Safari, Edge)
		// Extensions send Origin like "chrome-extension://abc123..."
		if isBrowserExtensionOrigin(origin) {
			c.Next()
			return
		}

		// 2. Allow Passwall web apps (vault.passwall.io, etc.)
		if isAllowedOrigin(origin) {
			c.Next()
			return
		}

		// 3. Allow if Referer is from extension or allowed domain
		// Some browsers send Referer instead of Origin
		if isAllowedReferer(referer) {
			c.Next()
			return
		}

		// 4. Allow mobile apps (they don't send Origin/Referer)
		// Sec-Fetch-Site: none = direct request (not from a website)
		secFetchSite := c.GetHeader("Sec-Fetch-Site")
		if secFetchSite == "none" || secFetchSite == "same-origin" {
			c.Next()
			return
		}

		// 5. Allow CORS preflight requests
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// 6. Allow if no Origin AND no Referer (server-to-server, curl, mobile apps)
		// These requests typically don't have browser security headers
		if origin == "" && referer == "" {
			c.Next()
			return
		}

		// Block: Request has Origin/Referer but not from allowed source
		// This catches websites trying to use our API
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Access denied",
			"message": "Icon API is only available for Passwall clients",
		})
		c.Abort()
	}
}

// GetIcon fetches and serves a website icon
// GET /icons/:domain
func (h *IconsHandler) GetIcon(c *gin.Context) {
	domain := c.Param("domain")

	// Validate domain
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required"})
		return
	}

	// Basic domain validation (prevent malicious input)
	if !domainRegex.MatchString(domain) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain format"})
		return
	}

	// 1. Check memory cache first (fastest)
	if cached, ok := h.cache.get(domain); ok {
		if cached.notFound {
			h.servePlaceholder(c, domain)
			return
		}
		h.serveIcon(c, cached.data, cached.contentType, iconCacheDuration)
		return
	}

	// 2. Check custom icons folder first (manually added icons have priority)
	if iconData, contentType, ok := h.loadCustomIcon(domain); ok {
		// Add to memory cache for faster subsequent access
		h.cache.set(domain, &cachedIcon{
			data:        iconData,
			contentType: contentType,
			expiresAt:   time.Now().Add(iconCacheDuration),
			notFound:    false,
		})
		h.serveIcon(c, iconData, contentType, iconCacheDuration)
		return
	}

	// 3. Check filesystem cache (persistent)
	if iconData, contentType, ok := h.loadFromDisk(domain); ok {
		// Add to memory cache for faster subsequent access
		h.cache.set(domain, &cachedIcon{
			data:        iconData,
			contentType: contentType,
			expiresAt:   time.Now().Add(iconCacheDuration),
			notFound:    false,
		})
		h.serveIcon(c, iconData, contentType, iconCacheDuration)
		return
	}

	// 4. Fetch from upstream sources
	iconData, contentType, err := h.fetchIcon(domain)
	if err != nil {
		h.logger.Debug("failed to fetch icon", "domain", domain, "error", err)
		// Cache the failure in memory (don't persist failures to disk)
		h.cache.set(domain, &cachedIcon{
			notFound:  true,
			expiresAt: time.Now().Add(iconCacheFailure),
		})
		h.servePlaceholder(c, domain)
		return
	}

	// 5. Save to disk (persistent) and memory cache
	if err := h.saveToDisk(domain, iconData, contentType); err != nil {
		h.logger.Error("failed to save icon to disk", "domain", domain, "error", err)
	}

	h.cache.set(domain, &cachedIcon{
		data:        iconData,
		contentType: contentType,
		expiresAt:   time.Now().Add(iconCacheDuration),
		notFound:    false,
	})

	h.serveIcon(c, iconData, contentType, iconCacheDuration)
}

// getIconFilename generates a safe filename for a domain
func (h *IconsHandler) getIconFilename(domain string) string {
	// Use MD5 hash of domain to prevent directory traversal attacks
	hash := md5.Sum([]byte(domain))
	hashStr := hex.EncodeToString(hash[:])
	return hashStr + ".icon"
}

// getMetadataFilename generates the metadata filename
func (h *IconsHandler) getMetadataFilename(domain string) string {
	hash := md5.Sum([]byte(domain))
	hashStr := hex.EncodeToString(hash[:])
	return hashStr + ".meta"
}

// saveToDisk saves icon data to the filesystem
func (h *IconsHandler) saveToDisk(domain string, data []byte, contentType string) error {
	iconPath := filepath.Join(h.storagePath, h.getIconFilename(domain))
	metaPath := filepath.Join(h.storagePath, h.getMetadataFilename(domain))

	// Write icon data
	if err := os.WriteFile(iconPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write icon file: %w", err)
	}

	// Write metadata (content-type and timestamp)
	metadata := fmt.Sprintf("%s\n%d\n%s", contentType, time.Now().Unix(), domain)
	if err := os.WriteFile(metaPath, []byte(metadata), 0644); err != nil {
		// Clean up icon file if metadata write fails
		os.Remove(iconPath)
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	h.logger.Debug("saved icon to disk", "domain", domain, "size", len(data))
	return nil
}

// loadFromDisk loads icon data from the filesystem
func (h *IconsHandler) loadFromDisk(domain string) ([]byte, string, bool) {
	iconPath := filepath.Join(h.storagePath, h.getIconFilename(domain))
	metaPath := filepath.Join(h.storagePath, h.getMetadataFilename(domain))

	// Check if files exist
	iconInfo, err := os.Stat(iconPath)
	if err != nil {
		return nil, "", false
	}

	// Check if icon is too old (older than cache duration)
	if time.Since(iconInfo.ModTime()) > iconCacheDuration {
		// Icon expired, remove it
		os.Remove(iconPath)
		os.Remove(metaPath)
		return nil, "", false
	}

	// Read metadata
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		// Metadata missing, assume PNG
		iconData, err := os.ReadFile(iconPath)
		if err != nil {
			return nil, "", false
		}
		return iconData, "image/png", true
	}

	// Parse metadata (content-type is first line)
	lines := strings.SplitN(string(metaData), "\n", 2)
	contentType := "image/png"
	if len(lines) > 0 && lines[0] != "" {
		contentType = lines[0]
	}

	// Read icon data
	iconData, err := os.ReadFile(iconPath)
	if err != nil {
		return nil, "", false
	}

	return iconData, contentType, true
}

// fetchIcon tries to fetch icon from multiple sources
func (h *IconsHandler) fetchIcon(domain string) ([]byte, string, error) {
	var lastErr error

	for _, sourceTemplate := range iconSources {
		url := fmt.Sprintf(sourceTemplate, domain)

		data, contentType, err := h.fetchFromURL(url)
		if err != nil {
			lastErr = err
			continue
		}

		// Verify it's actually an image
		if !strings.HasPrefix(contentType, "image/") {
			lastErr = fmt.Errorf("not an image: %s", contentType)
			continue
		}

		return data, contentType, nil
	}

	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", fmt.Errorf("no icon found for domain: %s", domain)
}

// fetchFromURL fetches icon data from a URL
func (h *IconsHandler) fetchFromURL(url string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}

	// Set user agent to avoid being blocked
	req.Header.Set("User-Agent", "PasswallServer/1.0 (Icon Fetcher)")
	req.Header.Set("Accept", "image/*")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	// Limit response size
	limitedReader := io.LimitReader(resp.Body, iconMaxSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png" // Default to PNG
	}

	return data, contentType, nil
}

// serveIcon serves the icon with proper cache headers
func (h *IconsHandler) serveIcon(c *gin.Context, data []byte, contentType string, cacheDuration time.Duration) {
	// Set cache headers for browser caching
	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", int(cacheDuration.Seconds())))
	c.Header("Expires", time.Now().Add(cacheDuration).UTC().Format(http.TimeFormat))
	c.Header("ETag", fmt.Sprintf(`"%x"`, len(data))) // Simple ETag based on size

	// Check If-None-Match for 304 response
	if match := c.GetHeader("If-None-Match"); match != "" {
		etag := fmt.Sprintf(`"%x"`, len(data))
		if match == etag {
			c.Status(http.StatusNotModified)
			return
		}
	}

	c.Data(http.StatusOK, contentType, data)
}

// servePlaceholder returns a simple placeholder response
// The client should handle this and show a fallback letter
func (h *IconsHandler) servePlaceholder(c *gin.Context, domain string) {
	// Set shorter cache for failures
	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", int(iconCacheFailure.Seconds())))
	c.Header("Expires", time.Now().Add(iconCacheFailure).UTC().Format(http.TimeFormat))

	// Return 204 No Content - client will show fallback
	c.Status(http.StatusNoContent)
}

// loadCustomIcon loads a manually added custom icon from the custom folder
// Custom icons are stored with domain name: example.com.png, example.com.ico, etc.
func (h *IconsHandler) loadCustomIcon(domain string) ([]byte, string, bool) {
	customPath := filepath.Join(h.storagePath, customIconsSubfolder)

	// Supported extensions in priority order
	extensions := []struct {
		ext         string
		contentType string
	}{
		{".png", "image/png"},
		{".ico", "image/x-icon"},
		{".svg", "image/svg+xml"},
		{".jpg", "image/jpeg"},
		{".jpeg", "image/jpeg"},
		{".webp", "image/webp"},
		{".gif", "image/gif"},
	}

	for _, ext := range extensions {
		iconPath := filepath.Join(customPath, domain+ext.ext)
		if data, err := os.ReadFile(iconPath); err == nil {
			h.logger.Debug("loaded custom icon", "domain", domain, "path", iconPath)
			return data, ext.contentType, true
		}
	}

	return nil, "", false
}

// UploadCustomIcon handles uploading a custom icon for a domain
// POST /icons/:domain
func (h *IconsHandler) UploadCustomIcon(c *gin.Context) {
	domain := c.Param("domain")

	// Validate domain
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required"})
		return
	}

	if !domainRegex.MatchString(domain) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain format"})
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("icon")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "icon file is required"})
		return
	}
	defer file.Close()

	// Check file size
	if header.Size > iconMaxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "file too large",
			"max_size": fmt.Sprintf("%dKB", iconMaxSize/1024),
		})
		return
	}

	// Read file content
	data, err := io.ReadAll(io.LimitReader(file, iconMaxSize))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	// Detect content type and validate it's an image
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	if !strings.HasPrefix(contentType, "image/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file must be an image"})
		return
	}

	// Determine file extension from content type
	ext := getExtensionFromContentType(contentType)
	if ext == "" {
		ext = ".png" // Default to .png
	}

	// Save to custom icons folder
	customPath := filepath.Join(h.storagePath, customIconsSubfolder)
	iconPath := filepath.Join(customPath, domain+ext)

	// Remove any existing custom icons for this domain (different extensions)
	h.removeExistingCustomIcons(domain)

	if err := os.WriteFile(iconPath, data, 0644); err != nil {
		h.logger.Error("failed to save custom icon", "domain", domain, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save icon"})
		return
	}

	// Invalidate cache for this domain
	h.invalidateCache(domain)

	h.logger.Info("custom icon uploaded", "domain", domain, "size", len(data), "type", contentType)

	c.JSON(http.StatusOK, gin.H{
		"message": "icon uploaded successfully",
		"domain":  domain,
		"size":    len(data),
		"type":    contentType,
	})
}

// DeleteCustomIcon removes a custom icon for a domain
// DELETE /icons/:domain
func (h *IconsHandler) DeleteCustomIcon(c *gin.Context) {
	domain := c.Param("domain")

	// Validate domain
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required"})
		return
	}

	if !domainRegex.MatchString(domain) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain format"})
		return
	}

	// Remove custom icons
	removed := h.removeExistingCustomIcons(domain)

	// Invalidate cache
	h.invalidateCache(domain)

	if removed {
		h.logger.Info("custom icon deleted", "domain", domain)
		c.JSON(http.StatusOK, gin.H{
			"message": "custom icon deleted",
			"domain":  domain,
		})
	} else {
		c.JSON(http.StatusNotFound, gin.H{
			"error":  "custom icon not found",
			"domain": domain,
		})
	}
}

// ListCustomIcons returns a list of all custom icons
// GET /icons/custom/list
func (h *IconsHandler) ListCustomIcons(c *gin.Context) {
	customPath := filepath.Join(h.storagePath, customIconsSubfolder)

	entries, err := os.ReadDir(customPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"icons": []string{}})
		return
	}

	var icons []gin.H
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Extract domain from filename (remove extension)
		ext := filepath.Ext(name)
		domain := strings.TrimSuffix(name, ext)

		info, err := entry.Info()
		if err != nil {
			continue
		}

		icons = append(icons, gin.H{
			"domain":      domain,
			"filename":    name,
			"size":        info.Size(),
			"uploaded_at": info.ModTime().Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"icons": icons,
		"count": len(icons),
	})
}

// removeExistingCustomIcons removes all custom icons for a domain (any extension)
func (h *IconsHandler) removeExistingCustomIcons(domain string) bool {
	customPath := filepath.Join(h.storagePath, customIconsSubfolder)
	extensions := []string{".png", ".ico", ".svg", ".jpg", ".jpeg", ".webp", ".gif"}

	removed := false
	for _, ext := range extensions {
		iconPath := filepath.Join(customPath, domain+ext)
		if err := os.Remove(iconPath); err == nil {
			removed = true
		}
	}

	return removed
}

// invalidateCache removes a domain from memory cache
func (h *IconsHandler) invalidateCache(domain string) {
	h.cache.mu.Lock()
	defer h.cache.mu.Unlock()

	delete(h.cache.items, domain)

	// Remove from order slice
	for i, d := range h.cache.order {
		if d == domain {
			h.cache.order = append(h.cache.order[:i], h.cache.order[i+1:]...)
			break
		}
	}
}

// getExtensionFromContentType returns file extension for a content type
func getExtensionFromContentType(contentType string) string {
	switch contentType {
	case "image/png":
		return ".png"
	case "image/x-icon", "image/vnd.microsoft.icon":
		return ".ico"
	case "image/svg+xml":
		return ".svg"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ""
	}
}
