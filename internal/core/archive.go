package core

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/seckatie/bookmarkd/internal/core/db"
)

// ArchiveOptions controls how a bookmark page is fetched and captured.
//
// This uses a real Chrome/Chromium browser (via the DevTools protocol) so that
// JS-heavy pages have a chance to fully render before we snapshot the final HTML.
type ArchiveOptions struct {
	// ChromePath optionally overrides the Chrome/Chromium executable path.
	// If empty, chromedp will try to find a browser on PATH / default locations.
	ChromePath string
	// Headless controls whether Chrome runs without a visible window.
	// Set to false to debug scraping in a real window ("headful").
	Headless bool
	// Timeout is the per-page deadline for navigation + rendering + capture.
	// If <= 0, a sensible default is used.
	Timeout time.Duration
	// WaitSelector optionally waits for a CSS selector to become visible before
	// capturing the page. This is useful for SPAs or sites that render late.
	WaitSelector string
}

// ArchiveResult is the captured output of archiving a single bookmark page.
type ArchiveResult struct {
	// FinalURL is the browser's final URL after redirects.
	FinalURL string
	// Title is the document title if available (may be empty).
	Title string
	// HTML is the final rendered document HTML (outerHTML of <html>).
	HTML string
}

// ArchiveRunOptions describes a higher-level archive run: either archive a single
// bookmark by ID, or archive a batch of unarchived bookmarks.
type ArchiveRunOptions struct {
	// ID, if > 0, archives only the bookmark with this ID.
	ID int64
	// Limit bounds the number of bookmarks archived when archiving in batch mode.
	// If <= 0, archives all unarchived bookmarks.
	Limit int
	// Options are passed through to the underlying browser capture.
	Options ArchiveOptions
}

// ArchiveRunResult reports the outcome of an archive run.
type ArchiveRunResult struct {
	Attempted int
	Succeeded int
	Failed    int
}

// ArchiveBookmark loads a URL in Chrome and returns the final rendered HTML.
//
// The function:
// - navigates to the provided URL
// - waits for <body> to be ready (and optionally opts.WaitSelector to be visible)
// - captures final URL, document.title, and <html> outerHTML
//
// Notes:
//   - This does not attempt to bypass paywalls/CAPTCHAs/login walls; failures are
//     returned as errors.
//   - For pages that set a blank title, we fall back to parsing <title> from HTML.
func ArchiveBookmark(ctx context.Context, url string, opts ArchiveOptions) (ArchiveResult, error) {
	log.Printf("Archiving bookmark %s", url)
	log.Printf("Opts: %+v", opts)
	if opts.Timeout <= 0 {
		opts.Timeout = 35 * time.Second
	}

	allocatorOpts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	allocatorOpts = append(allocatorOpts,
		chromedp.NoDefaultBrowserCheck,
		chromedp.NoFirstRun,
	)
	if opts.ChromePath != "" {
		allocatorOpts = append(allocatorOpts, chromedp.ExecPath(opts.ChromePath))
	}
	if opts.Headless {
		allocatorOpts = append(allocatorOpts, chromedp.Headless)
	} else {
		allocatorOpts = append(allocatorOpts, chromedp.Flag("headless", false))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocatorOpts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	runCtx, cancelRun := context.WithTimeout(browserCtx, opts.Timeout)
	defer cancelRun()

	var html string
	var title string
	var finalURL string

	// Wait for network idle to ensure all resources are loaded
	waitForNetworkIdle := func(ctx context.Context) error {
		// Enable lifecycle events
		if err := page.SetLifecycleEventsEnabled(true).Do(ctx); err != nil {
			return err
		}

		// Create a channel to receive lifecycle events
		ch := make(chan struct{})
		chromedp.ListenTarget(ctx, func(ev interface{}) {
			if e, ok := ev.(*page.EventLifecycleEvent); ok {
				if e.Name == "networkIdle" {
					select {
					case ch <- struct{}{}:
					default:
					}
				}
			}
		})

		// Navigate and wait for network idle
		if err := chromedp.Navigate(url).Do(ctx); err != nil {
			return err
		}

		// Wait for networkIdle event or timeout
		select {
		case <-ch:
			log.Printf("Network idle reached for %s", url)
		case <-ctx.Done():
			return ctx.Err()
		}

		return nil
	}

	actions := []chromedp.Action{
		chromedp.ActionFunc(waitForNetworkIdle),
		chromedp.WaitReady("body", chromedp.ByQuery),
	}
	if strings.TrimSpace(opts.WaitSelector) != "" {
		actions = append(actions, chromedp.WaitVisible(opts.WaitSelector, chromedp.ByQuery))
	}
	// Small delay to allow any final JS execution after network idle
	actions = append(actions,
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)

	if err := chromedp.Run(runCtx, actions...); err != nil {
		return ArchiveResult{}, err
	}

	// Some pages leave document.title blank; fall back to parsing HTML if needed.
	if strings.TrimSpace(title) == "" && strings.TrimSpace(html) != "" {
		if doc, err := goquery.NewDocumentFromReader(strings.NewReader(html)); err == nil {
			title = strings.TrimSpace(doc.Find("title").First().Text())
		}
	}

	return ArchiveResult{
		FinalURL: finalURL,
		Title:    title,
		HTML:     html,
	}, nil
}

// ArchiveAndPersist archives a bookmark URL and stores the result in the database.
//
// On success, it writes:
// - archive_attempted_at
// - archived_at
// - archive_status = "ok"
// - archived_url, archived_html
//
// On failure, it still records:
// - archive_attempted_at
// - archive_status = "error"
// - archive_error
func ArchiveAndPersist(ctx context.Context, database *db.DB, b db.Bookmark, opts ArchiveOptions) error {
	attemptedAt := time.Now()

	res, err := ArchiveBookmark(ctx, b.URL, opts)
	if err != nil {
		saveErr := database.SaveArchiveResult(b.ID, attemptedAt, nil, "error", err.Error(), "", "")
		if saveErr != nil {
			return fmt.Errorf("archive failed (%v) and saving failure failed (%v)", err, saveErr)
		}
		return err
	}

	// Inline external resources to make HTML self-contained
	log.Printf("Inlining resources for bookmark id=%d", b.ID)
	inlineOpts := DefaultInlineOptions(res.FinalURL)
	inlinedHTML, err := InlineResources(ctx, res.HTML, inlineOpts)
	if err != nil {
		log.Printf("Warning: failed to inline resources for id=%d: %v (using original HTML)", b.ID, err)
		inlinedHTML = res.HTML
	}

	archivedAt := time.Now()
	if err := database.SaveArchiveResult(b.ID, attemptedAt, &archivedAt, "ok", "", res.FinalURL, inlinedHTML); err != nil {
		return err
	}

	// Optional: if the stored title is empty, you could update it here in the future.
	_ = res.Title
	log.Printf("Archived bookmark id=%d url=%s", b.ID, b.URL)
	return nil
}

// RunArchive is the top-level archiving workflow.
//
// It supports:
// - single-bookmark mode (opts.ID > 0)
// - batch mode (archives bookmarks where archived_at IS NULL, optionally limited)
//
// It returns an ArchiveRunResult plus an error if any bookmarks failed to archive.
func RunArchive(ctx context.Context, database *db.DB, opts ArchiveRunOptions) (ArchiveRunResult, error) {
	if opts.ID > 0 {
		b, err := database.GetBookmark(opts.ID)
		if err != nil {
			return ArchiveRunResult{}, err
		}
		if err := ArchiveAndPersist(ctx, database, b, opts.Options); err != nil {
			return ArchiveRunResult{Attempted: 1, Failed: 1}, err
		}
		return ArchiveRunResult{Attempted: 1, Succeeded: 1}, nil
	}

	bookmarks, err := database.ListBookmarksToArchive(opts.Limit)
	if err != nil {
		return ArchiveRunResult{}, err
	}
	if len(bookmarks) == 0 {
		log.Println("No bookmarks to archive.")
		return ArchiveRunResult{}, nil
	}

	log.Printf("Archiving %d bookmark(s)...", len(bookmarks))
	var res ArchiveRunResult
	for _, b := range bookmarks {
		res.Attempted++
		if err := ArchiveAndPersist(ctx, database, b, opts.Options); err != nil {
			res.Failed++
			log.Printf("Archive failed for id=%d url=%s: %v", b.ID, b.URL, err)
			continue
		}
		res.Succeeded++
	}

	if res.Failed > 0 {
		return res, fmt.Errorf("archiving finished with %d failure(s)", res.Failed)
	}

	log.Println("Archiving finished successfully.")
	return res, nil
}
