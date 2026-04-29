package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/evanw/esbuild/pkg/api"
)

func main() {
	production := false
	watch := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--production":
			production = true
		case "--watch":
			watch = true
		}
	}

	sourcemap := api.SourceMapLinked
	if production {
		sourcemap = api.SourceMapNone
	}

	opts := api.BuildOptions{
		EntryPoints:       []string{"src/web/extension.ts"},
		Bundle:            true,
		Write:             true,
		Format:            api.FormatCommonJS,
		MinifyWhitespace:  production,
		MinifyIdentifiers: production,
		MinifySyntax:      production,
		Sourcemap:         sourcemap,
		SourcesContent:    api.SourcesContentExclude,
		Platform:          api.PlatformBrowser,
		Outdir:            "dist/web",
		External:          []string{"vscode", "util", "worker_threads"},
		LogLevel:          api.LogLevelInfo,
		Loader: map[string]api.Loader{
			".html": api.LoaderText,
		},
		Define: map[string]string{
			"global": "globalThis",
		},
	}

	ctx, ctxErr := api.Context(opts)
	if ctxErr != nil {
		fmt.Fprintf(os.Stderr, "%v\n", ctxErr)
		os.Exit(1)
	}

	if watch {
		if err := ctx.Watch(api.WatchOptions{}); err != nil {
			ctx.Dispose()
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		fmt.Println("watching for changes...")

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		fmt.Println("shutting down")

		ctx.Dispose()
		return
	}

	result := ctx.Rebuild()
	ctx.Dispose()
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			fmt.Fprintln(os.Stderr, e.Text)
		}
		os.Exit(1)
	}
	fmt.Println("build complete")
}
