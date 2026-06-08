// wtest: Headless Chrome test runner for JS/HTML integration tests.
//
// Usage: wtest [flags] <url>
//
// Flags:
//   -timeout   Set the max time to load the page (default 30s; accepts "15s", "1m", etc).
//   -v         Print JavaScript exceptions and page errors to stderr (default false).
//   -wait-load Wait for networkIdle event before exiting (default true).
//
// Pass the URL of the web page to test (e.g., some local development server or test server).
// wtest launches headless Chrome, navigates to the page, and fails (exit code 1) if any JS exception is thrown.
//
// Intended to be run as part of automated Go tests, to validate example HTML apps.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func main() {
	timeout := flag.Duration("timeout", 30*time.Second, "page load timeout (e.g. 15s, 1m)")
	verbose := flag.Bool("v", false, "print exceptions and page errors to stderr")
	waitLoad := flag.Bool("wait-load", true, "wait for the networkIdle event before exiting")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: wtest [flags] <url>\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExits 0 if no JS exceptions are thrown, 1 otherwise.\n")
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	url := flag.Arg(0)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	// ── launch headless Chrome ────────────────────────────────────────────────
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("no-sandbox", ""),
		chromedp.Flag("disable-gpu", ""),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, *timeout)
	defer cancel()

	// ── collect exceptions ────────────────────────────────────────────────────
	type jsError struct {
		msg    string
		source string
		line   int
	}
	var (
		mu         sync.Mutex
		exceptions []jsError
	)

	chromedp.ListenTarget(ctx, func(ev any) {
		switch ev := ev.(type) {
		case *runtime.EventExceptionThrown:
			ex := ev.ExceptionDetails
			msg := ex.Error()
			if msg == "" {
				msg = ex.Text
			}
			src := ex.URL
			if src == "" {
				src = "<unknown>"
			}
			line := int(ex.LineNumber)
			mu.Lock()
			exceptions = append(exceptions, jsError{msg: msg, source: src, line: line})
			mu.Unlock()
			if *verbose {
				fmt.Fprintf(os.Stderr, "[exception] %s  (%s:%d)\n", msg, src, line)
			}
		case *runtime.EventConsoleAPICalled:
			if *verbose && ev.Type == runtime.APITypeError {
				for _, arg := range ev.Args {
					fmt.Fprintf(os.Stderr, "[console.error] %s\n", arg.Value)
				}
			}
		}
	})

	// ── navigate ──────────────────────────────────────────────────────────────
	if *waitLoad {
		navErr, waitErr := navigateAndWaitNetworkIdle(ctx, url)
		if navErr != nil {
			fmt.Fprintf(os.Stderr, "navigation error: %v\n", navErr)
			os.Exit(1)
		}
		if waitErr != nil {
			fmt.Fprintf(os.Stderr, "wait-load error: %v\n", waitErr)
			// Don't exit — still check collected exceptions below.
		}
	} else if err := chromedp.Run(ctx, chromedp.Navigate(url)); err != nil {
		fmt.Fprintf(os.Stderr, "navigation error: %v\n", err)
		os.Exit(1)
	}

	// Give the event loop a moment to flush any final exceptions.
	time.Sleep(300 * time.Millisecond)

	// ── report ────────────────────────────────────────────────────────────────
	mu.Lock()
	n := len(exceptions)
	exs := append([]jsError(nil), exceptions...)
	mu.Unlock()

	if n == 0 {
		if *verbose {
			fmt.Fprintf(os.Stderr, "✓ no JS exceptions detected on %s\n", url)
		}
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "✗ %d JS exception(s) detected on %s:\n", n, url)
	for i, ex := range exs {
		fmt.Fprintf(os.Stderr, "  %d) %s  (%s:%d)\n", i+1, ex.msg, ex.source, ex.line)
	}
	os.Exit(1)
}

func navigateAndWaitNetworkIdle(ctx context.Context, url string) (navErr, waitErr error) {
	ch := make(chan struct{})
	var once sync.Once
	listenerCtx, listenerCancel := context.WithCancel(ctx)
	defer listenerCancel()

	chromedp.ListenTarget(listenerCtx, func(ev any) {
		if e, ok := ev.(*page.EventLifecycleEvent); ok && e.Name == "networkIdle" {
			once.Do(func() { close(ch) })
		}
	})

	if navErr = chromedp.Run(ctx, chromedp.Navigate(url)); navErr != nil {
		return navErr, nil
	}

	select {
	case <-ch:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
