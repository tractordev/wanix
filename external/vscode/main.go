package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"strings"

	"tractor.dev/wanix/kernel/fsys"
)

//go:embed assets
var assets embed.FS

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	go func() {
		log.Println("launching vscode...")
		b, err := os.ReadFile("/web/dom/new/iframe")
		fatal(err)
		id := strings.TrimSpace(string(b))
		fatal(os.WriteFile("/web/dom/body/ctl", []byte(fmt.Sprintf("append-child %s", id)), 0))
		fatal(os.WriteFile("/web/dom/style", []byte("iframe { width: 100%; height: 100%; position: absolute; top: 0; left: 0; }"), 0))
		// for the moment, "go9p" is hardcoded mount point for exported fs.
		// similarly, for now "/sw" is hardcoded path for wanix root fs.
		fatal(os.WriteFile(fmt.Sprintf("/web/dom/%s/attrs", id), []byte("src=/sw/go9p/assets/"), 0))
	}()

	log.Println("exporting fs...")
	fatal(fsys.Export(assets))
}
