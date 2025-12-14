package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// AllowInternalURLsForTesting disables SSRF protection for testing purposes.
// This should only be set to true in test code, never in production.
var AllowInternalURLsForTesting = false

// isInternalURL checks if a URL points to a private/internal network address.
// This helps prevent SSRF attacks by blocking requests to localhost, private IPs, etc.
func isInternalURL(urlStr string) bool {
	if AllowInternalURLsForTesting {
		return false
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return true // Fail safe - block if we can't parse
	}

	host := u.Hostname()
	if host == "" {
		return true // No host means it's not a valid external URL
	}

	// Check for localhost variants
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || lowerHost == "127.0.0.1" || lowerHost == "::1" {
		return true
	}

	// Check for internal domain suffixes
	if strings.HasSuffix(lowerHost, ".local") ||
		strings.HasSuffix(lowerHost, ".localhost") ||
		strings.HasSuffix(lowerHost, ".internal") ||
		strings.HasSuffix(lowerHost, ".localdomain") {
		return true
	}

	// Parse as IP and check for private ranges
	ip := net.ParseIP(host)
	if ip != nil {
		// Check for loopback, private, and link-local addresses
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}
		// Block unspecified addresses (0.0.0.0, ::)
		if ip.IsUnspecified() {
			return true
		}
	}

	return false
}

// InlineOptions controls how resources are inlined into archived HTML.
type InlineOptions struct {
	// BaseURL is used to resolve relative URLs in the HTML.
	BaseURL string
	// Timeout is the per-resource fetch timeout.
	Timeout time.Duration
	// MaxResourceSize is the maximum size of a single resource to inline (bytes).
	// Resources larger than this are skipped. 0 means no limit.
	MaxResourceSize int64
	// InlineImages controls whether images are converted to data URIs.
	InlineImages bool
	// InlineCSS controls whether external stylesheets are inlined.
	InlineCSS bool
	// InlineJS controls whether external scripts are inlined.
	InlineJS bool
}

// DefaultInlineOptions returns sensible defaults for inlining.
func DefaultInlineOptions(baseURL string) InlineOptions {
	return InlineOptions{
		BaseURL:         baseURL,
		Timeout:         DefaultResourceTimeout,
		MaxResourceSize: MaxResourceSize,
		InlineImages:    true,
		InlineCSS:       true,
		InlineJS:        true,
	}
}

// resourceInliner handles the inlining of external resources into HTML.
// It encapsulates the context, HTTP client, base URL, and options needed
// for fetching and processing resources.
type resourceInliner struct {
	ctx     context.Context
	client  *http.Client
	baseURL *url.URL
	opts    InlineOptions
}

// newResourceInliner creates a new resourceInliner with the given configuration.
func newResourceInliner(ctx context.Context, opts InlineOptions) (*resourceInliner, error) {
	baseURL, err := url.Parse(opts.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	return &resourceInliner{
		ctx:     ctx,
		client:  &http.Client{Timeout: opts.Timeout},
		baseURL: baseURL,
		opts:    opts,
	}, nil
}

// logFetchError logs fetch errors, filtering out common 404 errors.
func (ri *resourceInliner) logFetchError(resourceType, url string, err error) {
	if !strings.Contains(err.Error(), "HTTP 404") {
		log.Printf("Failed to fetch %s %s: %v", resourceType, url, err)
	}
}

// inlineStylesheets converts external <link rel="stylesheet"> tags to inline <style> tags.
func (ri *resourceInliner) inlineStylesheets(doc *goquery.Document) {
	doc.Find("link[rel='stylesheet']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		cssURL := resolveURL(ri.baseURL, href)
		if cssURL == "" {
			return
		}

		css, err := fetchResource(ri.ctx, ri.client, cssURL, ri.opts.MaxResourceSize)
		if err != nil {
			ri.logFetchError("CSS", cssURL, err)
			return
		}

		// Process CSS to inline any url() references
		css = inlineCSSURLs(ri.ctx, ri.client, css, cssURL, ri.opts)

		// Replace <link> with <style>
		s.ReplaceWithHtml(fmt.Sprintf("<style>%s</style>", css))
	})
}

// inlineScripts converts external <script src> tags to inline scripts.
func (ri *resourceInliner) inlineScripts(doc *goquery.Document) {
	doc.Find("script[src]").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists || src == "" {
			return
		}

		jsURL := resolveURL(ri.baseURL, src)
		if jsURL == "" {
			return
		}

		js, err := fetchResource(ri.ctx, ri.client, jsURL, ri.opts.MaxResourceSize)
		if err != nil {
			ri.logFetchError("JS", jsURL, err)
			return
		}

		// Replace script with inline version
		s.RemoveAttr("src")
		s.SetText(js)
	})
}

// inlineImages converts image src attributes to data URIs.
func (ri *resourceInliner) inlineImages(doc *goquery.Document) {
	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists || src == "" {
			return
		}

		// Skip data URIs
		if strings.HasPrefix(src, "data:") {
			return
		}

		imgURL := resolveURL(ri.baseURL, src)
		if imgURL == "" {
			return
		}

		dataURI, err := fetchAsDataURI(ri.ctx, ri.client, imgURL, ri.opts.MaxResourceSize)
		if err != nil {
			ri.logFetchError("image", imgURL, err)
			return
		}

		s.SetAttr("src", dataURI)
	})

	// Remove srcset attributes since they're complex and we've inlined src
	doc.Find("img[srcset], source[srcset]").Each(func(i int, s *goquery.Selection) {
		s.RemoveAttr("srcset")
	})
}

// inlineBackgroundImages processes style attributes to inline CSS url() references.
func (ri *resourceInliner) inlineBackgroundImages(doc *goquery.Document) {
	doc.Find("[style]").Each(func(i int, s *goquery.Selection) {
		style, _ := s.Attr("style")
		if strings.Contains(style, "url(") {
			newStyle := inlineCSSURLs(ri.ctx, ri.client, style, ri.opts.BaseURL, ri.opts)
			s.SetAttr("style", newStyle)
		}
	})
}

// addBaseTag adds a <base> tag for any remaining relative URLs that couldn't be inlined.
func (ri *resourceInliner) addBaseTag(doc *goquery.Document) {
	head := doc.Find("head")
	if head.Length() > 0 && doc.Find("base").Length() == 0 {
		head.PrependHtml(fmt.Sprintf(`<base href="%s">`, ri.baseURL.String()))
	}
}

// InlineResources processes HTML and inlines external resources.
// This makes the archived HTML self-contained and viewable offline.
func InlineResources(ctx context.Context, html string, opts InlineOptions) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	inliner, err := newResourceInliner(ctx, opts)
	if err != nil {
		return "", err
	}

	if opts.InlineCSS {
		inliner.inlineStylesheets(doc)
	}
	if opts.InlineJS {
		inliner.inlineScripts(doc)
	}
	if opts.InlineImages {
		inliner.inlineImages(doc)
	}
	inliner.inlineBackgroundImages(doc)
	inliner.addBaseTag(doc)

	result, err := doc.Html()
	if err != nil {
		return "", fmt.Errorf("failed to serialize HTML: %w", err)
	}

	return result, nil
}

// resolveURL resolves a potentially relative URL against a base URL.
func resolveURL(base *url.URL, ref string) string {
	if ref == "" {
		return ""
	}

	// Skip data URIs and javascript:
	if strings.HasPrefix(ref, "data:") || strings.HasPrefix(ref, "javascript:") {
		return ""
	}

	refURL, err := url.Parse(ref)
	if err != nil {
		return ""
	}

	resolved := base.ResolveReference(refURL)
	return resolved.String()
}

// fetchResult holds the result of fetching a URL.
type fetchResult struct {
	data        []byte
	contentType string
}

// fetchURL is the shared HTTP fetch implementation.
// It handles request creation, size limits, and response reading.
// It blocks requests to internal/private network addresses to prevent SSRF attacks.
func fetchURL(ctx context.Context, client *http.Client, urlStr string, maxSize int64) (*fetchResult, error) {
	// SSRF protection: block requests to internal network addresses
	if isInternalURL(urlStr) {
		return nil, fmt.Errorf("blocked request to internal URL: %s", urlStr)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if maxSize > 0 {
		reader = io.LimitReader(resp.Body, maxSize)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	return &fetchResult{data: data, contentType: contentType}, nil
}

// fetchResource fetches a URL and returns its content as a string.
func fetchResource(ctx context.Context, client *http.Client, urlStr string, maxSize int64) (string, error) {
	result, err := fetchURL(ctx, client, urlStr, maxSize)
	if err != nil {
		return "", err
	}
	return string(result.data), nil
}

// fetchAsDataURI fetches a URL and returns it as a data URI.
func fetchAsDataURI(ctx context.Context, client *http.Client, urlStr string, maxSize int64) (string, error) {
	result, err := fetchURL(ctx, client, urlStr, maxSize)
	if err != nil {
		return "", err
	}

	contentType := result.contentType
	// Strip charset suffix for data URI
	if idx := strings.Index(contentType, ";"); idx > 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	encoded := base64.StdEncoding.EncodeToString(result.data)
	return fmt.Sprintf("data:%s;base64,%s", contentType, encoded), nil
}

// inlineCSSURLs processes CSS and inlines any url() references.
func inlineCSSURLs(ctx context.Context, client *http.Client, css string, baseURLStr string, opts InlineOptions) string {
	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return css
	}

	// Process url() patterns by building result incrementally
	var result strings.Builder
	remaining := css

	for {
		startIdx := strings.Index(remaining, "url(")
		if startIdx == -1 {
			result.WriteString(remaining)
			break
		}

		// Write everything before url(
		result.WriteString(remaining[:startIdx])

		// Find the closing parenthesis
		afterURL := remaining[startIdx+4:]
		endIdx := strings.Index(afterURL, ")")
		if endIdx == -1 {
			result.WriteString(remaining[startIdx:])
			break
		}

		urlContent := afterURL[:endIdx]

		// Strip quotes
		urlContent = strings.TrimSpace(urlContent)
		urlContent = strings.Trim(urlContent, `"'`)

		// Skip data URIs - keep them as-is
		if strings.HasPrefix(urlContent, "data:") {
			result.WriteString(remaining[startIdx : startIdx+4+endIdx+1])
			remaining = remaining[startIdx+4+endIdx+1:]
			continue
		}

		// Resolve and fetch
		resolved := resolveURL(baseURL, urlContent)
		if resolved == "" {
			// Keep original
			result.WriteString(remaining[startIdx : startIdx+4+endIdx+1])
			remaining = remaining[startIdx+4+endIdx+1:]
			continue
		}

		dataURI, err := fetchAsDataURI(ctx, client, resolved, opts.MaxResourceSize)
		if err != nil {
			// Only log non-404 errors (404s are common for deleted/moved resources)
			if !strings.Contains(err.Error(), "HTTP 404") {
				log.Printf("Failed to fetch CSS resource %s: %v", resolved, err)
			}
			// Keep original URL
			result.WriteString(remaining[startIdx : startIdx+4+endIdx+1])
			remaining = remaining[startIdx+4+endIdx+1:]
			continue
		}

		// Write the inlined url()
		result.WriteString(fmt.Sprintf("url(%s)", dataURI))
		remaining = remaining[startIdx+4+endIdx+1:]
	}

	return result.String()
}
