package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

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
		Timeout:         10 * time.Second,
		MaxResourceSize: 5 * 1024 * 1024, // 5MB
		InlineImages:    true,
		InlineCSS:       true,
		InlineJS:        true,
	}
}

// InlineResources processes HTML and inlines external resources.
// This makes the archived HTML self-contained and viewable offline.
func InlineResources(ctx context.Context, html string, opts InlineOptions) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	baseURL, err := url.Parse(opts.BaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	client := &http.Client{
		Timeout: opts.Timeout,
	}

	// Inline stylesheets
	if opts.InlineCSS {
		doc.Find("link[rel='stylesheet']").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists || href == "" {
				return
			}

			cssURL := resolveURL(baseURL, href)
			if cssURL == "" {
				return
			}

			css, err := fetchResource(ctx, client, cssURL, opts.MaxResourceSize)
			if err != nil {
				// Only log non-404 errors (404s are common for deleted/moved resources)
				if !strings.Contains(err.Error(), "HTTP 404") {
					log.Printf("Failed to fetch CSS %s: %v", cssURL, err)
				}
				return
			}

			// Process CSS to inline any url() references
			css = inlineCSSURLs(ctx, client, css, cssURL, opts)

			// Replace <link> with <style>
			s.ReplaceWithHtml(fmt.Sprintf("<style>%s</style>", css))
		})
	}

	// Inline scripts
	if opts.InlineJS {
		doc.Find("script[src]").Each(func(i int, s *goquery.Selection) {
			src, exists := s.Attr("src")
			if !exists || src == "" {
				return
			}

			jsURL := resolveURL(baseURL, src)
			if jsURL == "" {
				return
			}

			js, err := fetchResource(ctx, client, jsURL, opts.MaxResourceSize)
			if err != nil {
				// Only log non-404 errors (404s are common for deleted/moved resources)
				if !strings.Contains(err.Error(), "HTTP 404") {
					log.Printf("Failed to fetch JS %s: %v", jsURL, err)
				}
				return
			}

			// Replace script with inline version
			s.RemoveAttr("src")
			s.SetText(js)
		})
	}

	// Inline images
	if opts.InlineImages {
		doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
			src, exists := s.Attr("src")
			if !exists || src == "" {
				return
			}

			// Skip data URIs
			if strings.HasPrefix(src, "data:") {
				return
			}

			imgURL := resolveURL(baseURL, src)
			if imgURL == "" {
				return
			}

			dataURI, err := fetchAsDataURI(ctx, client, imgURL, opts.MaxResourceSize)
			if err != nil {
				// Only log non-404 errors (404s are common for deleted/moved resources)
				if !strings.Contains(err.Error(), "HTTP 404") {
					log.Printf("Failed to fetch image %s: %v", imgURL, err)
				}
				return
			}

			s.SetAttr("src", dataURI)
		})

		// Also handle srcset
		doc.Find("img[srcset], source[srcset]").Each(func(i int, s *goquery.Selection) {
			srcset, exists := s.Attr("srcset")
			if !exists || srcset == "" {
				return
			}

			// srcset is complex (multiple URLs with descriptors), just remove it
			// to fall back to src which we've already inlined
			s.RemoveAttr("srcset")
		})
	}

	// Handle background images in style attributes
	doc.Find("[style]").Each(func(i int, s *goquery.Selection) {
		style, _ := s.Attr("style")
		if strings.Contains(style, "url(") {
			newStyle := inlineCSSURLs(ctx, client, style, opts.BaseURL, opts)
			s.SetAttr("style", newStyle)
		}
	})

	// Add base tag for any remaining relative URLs we couldn't inline
	head := doc.Find("head")
	if head.Length() > 0 {
		// Check if base tag already exists
		if doc.Find("base").Length() == 0 {
			head.PrependHtml(fmt.Sprintf(`<base href="%s">`, baseURL.String()))
		}
	}

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

// fetchResource fetches a URL and returns its content as a string.
func fetchResource(ctx context.Context, client *http.Client, urlStr string, maxSize int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; bookmarkd/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if maxSize > 0 {
		reader = io.LimitReader(resp.Body, maxSize)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// fetchAsDataURI fetches a URL and returns it as a data URI.
func fetchAsDataURI(ctx context.Context, client *http.Client, urlStr string, maxSize int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; bookmarkd/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if maxSize > 0 {
		reader = io.LimitReader(resp.Body, maxSize)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	// Strip charset suffix for data URI
	if idx := strings.Index(contentType, ";"); idx > 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	encoded := base64.StdEncoding.EncodeToString(data)
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
