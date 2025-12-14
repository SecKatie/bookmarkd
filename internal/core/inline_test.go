package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMain sets up the test environment.
// We enable internal URLs for testing since httptest.NewServer uses localhost.
func TestMain(m *testing.M) {
	AllowInternalURLsForTesting = true
	code := m.Run()
	AllowInternalURLsForTesting = false
	os.Exit(code)
}

func TestResolveURL(t *testing.T) {
	base, _ := url.Parse("https://example.com/page/")

	tests := []struct {
		name     string
		ref      string
		expected string
	}{
		{"empty", "", ""},
		{"absolute URL", "https://other.com/file.css", "https://other.com/file.css"},
		{"relative path", "style.css", "https://example.com/page/style.css"},
		{"root relative", "/style.css", "https://example.com/style.css"},
		{"parent path", "../style.css", "https://example.com/style.css"},
		{"protocol relative", "//cdn.example.com/file.js", "https://cdn.example.com/file.js"},
		{"data URI", "data:text/css,body{}", ""},
		{"javascript", "javascript:void(0)", ""},
		{"query string", "style.css?v=1.0", "https://example.com/page/style.css?v=1.0"},
		{"fragment", "style.css#section", "https://example.com/page/style.css#section"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveURL(base, tt.ref)
			if result != tt.expected {
				t.Errorf("resolveURL(%q) = %q, want %q", tt.ref, result, tt.expected)
			}
		})
	}
}

func TestFetchURL(t *testing.T) {
	tests := []struct {
		name          string
		handler       http.HandlerFunc
		maxSize       int64
		wantErr       bool
		wantData      string
		wantType      string
		errContains   string
	}{
		{
			name: "successful fetch",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				if _, err := w.Write([]byte("hello world")); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			},
			wantData: "hello world",
			wantType: "text/plain",
		},
		{
			name: "404 error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			},
			wantErr:     true,
			errContains: "HTTP 404",
		},
		{
			name: "500 error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "server error", http.StatusInternalServerError)
			},
			wantErr:     true,
			errContains: "HTTP 500",
		},
		{
			name: "content type detection",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// PNG magic bytes
				if _, err := w.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			},
			wantType: "image/png",
		},
		{
			name: "size limit",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				if _, err := w.Write([]byte("12345678901234567890")); err != nil { // 20 bytes
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			},
			maxSize:  10,
			wantData: "1234567890", // truncated to 10 bytes
		},
		{
			name: "user agent header",
			handler: func(w http.ResponseWriter, r *http.Request) {
				ua := r.Header.Get("User-Agent")
				if !strings.Contains(ua, "bookmarkd") {
					http.Error(w, "wrong user agent", http.StatusBadRequest)
					return
				}
				if _, err := w.Write([]byte("ok")); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			},
			wantData: "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			client := &http.Client{Timeout: 5 * time.Second}
			result, err := fetchURL(context.Background(), client, ts.URL, tt.maxSize)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantData != "" && string(result.data) != tt.wantData {
				t.Errorf("data = %q, want %q", string(result.data), tt.wantData)
			}

			if tt.wantType != "" && !strings.HasPrefix(result.contentType, tt.wantType) {
				t.Errorf("contentType = %q, want prefix %q", result.contentType, tt.wantType)
			}
		})
	}
}

func TestFetchResource(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		if _, err := w.Write([]byte("body { color: red; }")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	result, err := fetchResource(context.Background(), client, ts.URL, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "body { color: red; }"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

func TestFetchAsDataURI(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		data        []byte
		wantPrefix  string
	}{
		{
			name:        "plain text",
			contentType: "text/plain",
			data:        []byte("hello"),
			wantPrefix:  "data:text/plain;base64,",
		},
		{
			name:        "CSS",
			contentType: "text/css; charset=utf-8",
			data:        []byte("body{}"),
			wantPrefix:  "data:text/css;base64,", // charset stripped
		},
		{
			name:        "PNG image",
			contentType: "image/png",
			data:        []byte{0x89, 0x50, 0x4E, 0x47},
			wantPrefix:  "data:image/png;base64,",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				if _, err := w.Write(tt.data); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}))
			defer ts.Close()

			client := &http.Client{Timeout: 5 * time.Second}
			result, err := fetchAsDataURI(context.Background(), client, ts.URL, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.HasPrefix(result, tt.wantPrefix) {
				t.Errorf("result should start with %q, got %q", tt.wantPrefix, result)
			}
		})
	}
}

func TestInlineResources(t *testing.T) {
	// Set up test server that serves different resources
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/style.css":
			w.Header().Set("Content-Type", "text/css")
			_, err = w.Write([]byte("body { color: red; }"))
		case "/script.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, err = w.Write([]byte("console.log('hello');"))
		case "/image.png":
			w.Header().Set("Content-Type", "image/png")
			// Minimal PNG
			_, err = w.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
		case "/css-with-url.css":
			w.Header().Set("Content-Type", "text/css")
			_, err = w.Write([]byte("body { background: url(/bg.png); }"))
		case "/bg.png":
			w.Header().Set("Content-Type", "image/png")
			_, err = w.Write([]byte{0x89, 0x50, 0x4E, 0x47})
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	t.Run("inline CSS", func(t *testing.T) {
		html := `<html><head><link rel="stylesheet" href="/style.css"></head><body></body></html>`
		result, err := InlineResources(context.Background(), html, DefaultInlineOptions(ts.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(result, "<style>body { color: red; }</style>") {
			t.Error("CSS should be inlined")
		}
		if strings.Contains(result, `<link rel="stylesheet"`) {
			t.Error("link tag should be replaced")
		}
	})

	t.Run("inline JS", func(t *testing.T) {
		html := `<html><head></head><body><script src="/script.js"></script></body></html>`
		result, err := InlineResources(context.Background(), html, DefaultInlineOptions(ts.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(result, "console.log") {
			t.Error("JS should be inlined")
		}
		if strings.Contains(result, `src="/script.js"`) {
			t.Error("src attribute should be removed")
		}
	})

	t.Run("inline images", func(t *testing.T) {
		html := `<html><head></head><body><img src="/image.png"></body></html>`
		result, err := InlineResources(context.Background(), html, DefaultInlineOptions(ts.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(result, "data:image/png;base64,") {
			t.Error("image should be converted to data URI")
		}
	})

	t.Run("skip existing data URIs", func(t *testing.T) {
		dataURI := "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7"
		html := `<html><head></head><body><img src="` + dataURI + `"></body></html>`
		result, err := InlineResources(context.Background(), html, DefaultInlineOptions(ts.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Original data URI should be preserved
		if !strings.Contains(result, dataURI) {
			t.Error("existing data URI should be preserved")
		}
	})

	t.Run("adds base tag", func(t *testing.T) {
		html := `<html><head></head><body></body></html>`
		result, err := InlineResources(context.Background(), html, DefaultInlineOptions(ts.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(result, `<base href="`) {
			t.Error("base tag should be added")
		}
	})

	t.Run("does not duplicate base tag", func(t *testing.T) {
		html := `<html><head><base href="https://original.com"></head><body></body></html>`
		result, err := InlineResources(context.Background(), html, DefaultInlineOptions(ts.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Count base tags
		count := strings.Count(result, "<base")
		if count != 1 {
			t.Errorf("should have exactly 1 base tag, got %d", count)
		}
	})

	t.Run("handles 404 gracefully", func(t *testing.T) {
		html := `<html><head><link rel="stylesheet" href="/nonexistent.css"></head><body></body></html>`
		result, err := InlineResources(context.Background(), html, DefaultInlineOptions(ts.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should not crash, link tag should remain
		if result == "" {
			t.Error("should return non-empty result even with 404")
		}
	})

	t.Run("respects InlineCSS option", func(t *testing.T) {
		html := `<html><head><link rel="stylesheet" href="/style.css"></head><body></body></html>`
		opts := DefaultInlineOptions(ts.URL)
		opts.InlineCSS = false

		result, err := InlineResources(context.Background(), html, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if strings.Contains(result, "<style>") {
			t.Error("CSS should not be inlined when InlineCSS is false")
		}
	})

	t.Run("respects InlineJS option", func(t *testing.T) {
		html := `<html><head></head><body><script src="/script.js"></script></body></html>`
		opts := DefaultInlineOptions(ts.URL)
		opts.InlineJS = false

		result, err := InlineResources(context.Background(), html, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if strings.Contains(result, "console.log") {
			t.Error("JS should not be inlined when InlineJS is false")
		}
	})

	t.Run("respects InlineImages option", func(t *testing.T) {
		html := `<html><head></head><body><img src="/image.png"></body></html>`
		opts := DefaultInlineOptions(ts.URL)
		opts.InlineImages = false

		result, err := InlineResources(context.Background(), html, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if strings.Contains(result, "data:image/png") {
			t.Error("images should not be inlined when InlineImages is false")
		}
	})

	t.Run("removes srcset", func(t *testing.T) {
		html := `<html><head></head><body><img src="/image.png" srcset="/image-2x.png 2x"></body></html>`
		result, err := InlineResources(context.Background(), html, DefaultInlineOptions(ts.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if strings.Contains(result, "srcset") {
			t.Error("srcset attribute should be removed")
		}
	})
}

func TestInlineCSSURLs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		if _, err := w.Write([]byte{0x89, 0x50, 0x4E, 0x47}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	tests := []struct {
		name    string
		css     string
		wantHas string
	}{
		{
			name:    "simple url",
			css:     "body { background: url(/bg.png); }",
			wantHas: "data:image/png;base64,",
		},
		{
			name:    "url with quotes",
			css:     `body { background: url("/bg.png"); }`,
			wantHas: "data:image/png;base64,",
		},
		{
			name:    "url with single quotes",
			css:     `body { background: url('/bg.png'); }`,
			wantHas: "data:image/png;base64,",
		},
		{
			name:    "preserves data URI",
			css:     "body { background: url(data:image/gif;base64,R0lGODlhAQABA); }",
			wantHas: "url(data:image/gif;base64,R0lGODlhAQABA)",
		},
		{
			name:    "multiple urls",
			css:     "body { background: url(/bg.png); } .foo { background: url(/bg.png); }",
			wantHas: "data:image/png;base64,",
		},
	}

	client := &http.Client{Timeout: 5 * time.Second}
	opts := DefaultInlineOptions(ts.URL)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inlineCSSURLs(context.Background(), client, tt.css, ts.URL, opts)
			if !strings.Contains(result, tt.wantHas) {
				t.Errorf("result should contain %q, got %q", tt.wantHas, result)
			}
		})
	}
}

func TestDefaultInlineOptions(t *testing.T) {
	opts := DefaultInlineOptions("https://example.com")

	if opts.BaseURL != "https://example.com" {
		t.Errorf("BaseURL = %q, want %q", opts.BaseURL, "https://example.com")
	}
	if opts.Timeout != DefaultResourceTimeout {
		t.Errorf("Timeout = %v, want %v", opts.Timeout, DefaultResourceTimeout)
	}
	if opts.MaxResourceSize != MaxResourceSize {
		t.Errorf("MaxResourceSize = %d, want %d", opts.MaxResourceSize, MaxResourceSize)
	}
	if !opts.InlineCSS {
		t.Error("InlineCSS should be true by default")
	}
	if !opts.InlineJS {
		t.Error("InlineJS should be true by default")
	}
	if !opts.InlineImages {
		t.Error("InlineImages should be true by default")
	}
}

func TestInvalidBaseURL(t *testing.T) {
	html := `<html><head></head><body></body></html>`
	opts := InlineOptions{BaseURL: "://invalid"}

	_, err := InlineResources(context.Background(), html, opts)
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}
	if !strings.Contains(err.Error(), "invalid base URL") {
		t.Errorf("error should mention invalid base URL, got: %v", err)
	}
}

func TestInvalidHTML(t *testing.T) {
	// goquery is quite tolerant, but let's test with empty input
	result, err := InlineResources(context.Background(), "", DefaultInlineOptions("https://example.com"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty input should still produce valid HTML structure
	if result == "" {
		t.Error("should produce non-empty result")
	}
}

func TestIsInternalURL(t *testing.T) {
	// Temporarily disable the test bypass to verify SSRF protection works
	AllowInternalURLsForTesting = false
	defer func() { AllowInternalURLsForTesting = true }()

	tests := []struct {
		name     string
		url      string
		internal bool
	}{
		// External URLs (should NOT be blocked)
		{"external https", "https://example.com/style.css", false},
		{"external http", "http://example.com/script.js", false},
		{"external with port", "https://cdn.example.com:8080/file", false},
		{"external IP", "https://93.184.216.34/path", false},

		// Localhost (should be blocked)
		{"localhost", "http://localhost/api", true},
		{"localhost with port", "http://localhost:8080/api", true},
		{"127.0.0.1", "http://127.0.0.1/api", true},
		{"127.0.0.1 with port", "http://127.0.0.1:3000/api", true},
		{"ipv6 localhost", "http://[::1]/api", true},

		// Private IP ranges (should be blocked)
		{"private 10.x", "http://10.0.0.1/internal", true},
		{"private 172.16.x", "http://172.16.0.1/internal", true},
		{"private 192.168.x", "http://192.168.1.1/internal", true},

		// Link-local (should be blocked)
		{"link local ipv4", "http://169.254.1.1/api", true},
		{"link local ipv6", "http://[fe80::1]/api", true},

		// Internal domain suffixes (should be blocked)
		{"dot local", "http://server.local/api", true},
		{"dot localhost", "http://myapp.localhost/api", true},
		{"dot internal", "http://server.internal/api", true},
		{"dot localdomain", "http://host.localdomain/api", true},

		// Unspecified (should be blocked)
		{"unspecified ipv4", "http://0.0.0.0/api", true},

		// Empty/invalid (should be blocked - fail safe)
		{"empty host", "http:///path", true},
		{"no host", "/relative/path", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInternalURL(tt.url)
			if result != tt.internal {
				t.Errorf("isInternalURL(%q) = %v, want %v", tt.url, result, tt.internal)
			}
		})
	}
}

func TestSSRFProtection(t *testing.T) {
	// Temporarily disable the test bypass to verify SSRF protection works
	AllowInternalURLsForTesting = false
	defer func() { AllowInternalURLsForTesting = true }()

	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("blocks localhost fetch", func(t *testing.T) {
		_, err := fetchURL(context.Background(), client, "http://localhost/secret", 0)
		if err == nil {
			t.Fatal("expected error for localhost URL")
		}
		if !strings.Contains(err.Error(), "blocked") {
			t.Errorf("error should mention blocked, got: %v", err)
		}
	})

	t.Run("blocks private IP fetch", func(t *testing.T) {
		_, err := fetchURL(context.Background(), client, "http://192.168.1.1/admin", 0)
		if err == nil {
			t.Fatal("expected error for private IP URL")
		}
		if !strings.Contains(err.Error(), "blocked") {
			t.Errorf("error should mention blocked, got: %v", err)
		}
	})

	t.Run("blocks internal domain fetch", func(t *testing.T) {
		_, err := fetchURL(context.Background(), client, "http://server.internal/api", 0)
		if err == nil {
			t.Fatal("expected error for internal domain URL")
		}
		if !strings.Contains(err.Error(), "blocked") {
			t.Errorf("error should mention blocked, got: %v", err)
		}
	})
}
