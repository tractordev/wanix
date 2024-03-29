//go:build !bundle

package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"tractor.dev/toolkit-go/engine/fs/watchfs"
	"tractor.dev/wanix/internal/httpfs"
	"tractor.dev/wanix/kernel/web/gwutil"
)

func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Call the next handler in the chain
		next.ServeHTTP(w, r)
		// Log the request details
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func main() {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	basePath := ""
	log.Printf("Serving WANIX dev server at http://localhost:7777%s ...\n", basePath)

	mux := http.NewServeMux()
	mux.Handle("/auth/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/" {
			domain := os.Getenv("AUTH0_DOMAIN")
			clientID := os.Getenv("AUTH0_CLIENTID")
			if domain == "" || clientID == "" {
				log.Fatal("Auth was used with Auth0 env vars set")
			}
			d, err := os.ReadFile("./dev/auth/index.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			d = bytes.ReplaceAll(d, []byte("AUTH0_DOMAIN"), []byte(domain))
			d = bytes.ReplaceAll(d, []byte("AUTH0_CLIENTID"), []byte(clientID))
			if _, err := w.Write(d); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		http.StripPrefix("/auth/", http.FileServer(http.Dir(dir+"/dev/auth"))).ServeHTTP(w, r)
	}))

	mux.Handle(fmt.Sprintf("%s/sys/dev/", basePath), http.StripPrefix(fmt.Sprintf("%s/sys/dev/", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gwutil.FileTransformer(watchfs.New(os.DirFS(dir)), httpfs.FileServer).ServeHTTP(w, r)
	})))
	mux.Handle(fmt.Sprintf("%s/wanix-kernel.gz", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, err := os.Open(filepath.Join(dir, "local/bin/kernel"))
		if err != nil {
			fmt.Println(err)
			http.Error(w, "File not found.", http.StatusNotFound)
			return
		}
		defer file.Close()

		var b bytes.Buffer
		gzipWriter := gzip.NewWriter(&b)
		defer gzipWriter.Close()

		if _, err := io.Copy(gzipWriter, file); err != nil {
			log.Println("copy:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := gzipWriter.Close(); err != nil {
			fmt.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reader := bytes.NewReader(b.Bytes())
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", `attachment; filename="wanix-kernel.gz"`)
		http.ServeContent(w, r, "wanix-kernel.gz", time.Now(), reader)
	}))
	mux.Handle(fmt.Sprintf("%s/wanix-bootloader.js", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/javascript")

		var packMode PackMode
		if os.Getenv("PROD") == "1" {
			log.Printf("Packing self-contained bootloader...\n")
			packMode = PackFileData
		} else {
			packMode = PackFilePaths
		}

		// TODO: Does this need to pack on every request for the bootloader?
		// I don't think you want to be changing PROD at runtime, so we can
		// probably cache this.
		PackFilesTo(w, packMode)
		f, err := os.ReadFile("./dev/bootloader.js")
		fatal(err)
		w.Write(f)
	}))
	mux.Handle(fmt.Sprintf("%s/~init/", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	}))
	mux.Handle(fmt.Sprintf("%s/", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix(fmt.Sprintf("%s/", basePath), http.FileServer(http.Dir(dir+"/dev"))).ServeHTTP(w, r)
	}))
	if err := http.ListenAndServe(":7777", loggerMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}
