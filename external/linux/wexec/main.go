package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func debug(format string, a ...any) {
	// log.Printf(format+"\n", a...)
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: wexec <wasm> [args...]")
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	args := os.Args[1:]
	args[0] = strings.TrimPrefix(filepath.Join(wd, args[0]), "/wanix/")

	debug("allocating pid")
	pidRaw, err := os.ReadFile("/wanix/task/new/wasi")
	if err != nil {
		log.Fatal(err)
	}
	pid := strings.TrimSpace(string(pidRaw))

	debug("writing cmd")
	if err := appendFile(fmt.Sprintf("/wanix/task/%s/cmd", pid), []byte(strings.Join(args, " "))); err != nil {
		log.Fatal(err)
	}

	debug("writing env")
	env := strings.Join(append(os.Environ(), ""), "\n")
	if err := appendFile(fmt.Sprintf("/wanix/task/%s/env", pid), []byte(env)); err != nil {
		log.Fatal(err)
	}

	var done atomic.Int32
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		debug("polling fd/1 => stdout")
		for {
			b, err := os.ReadFile(fmt.Sprintf("/wanix/task/%s/fd/1", pid))
			if err != nil {
				log.Fatal(err)
			}
			if done.Load() == 1 && len(b) == 0 {
				return
			}
			os.Stdout.Write(b)
			time.Sleep(200 * time.Millisecond)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		debug("polling fd/2 => stderr")
		for {
			b, err := os.ReadFile(fmt.Sprintf("/wanix/task/%s/fd/2", pid))
			if err != nil {
				log.Fatal(err)
			}
			if done.Load() == 1 && len(b) == 0 {
				return
			}
			os.Stderr.Write(b)
			time.Sleep(200 * time.Millisecond)
		}
	}()

	debug("starting")
	if err := appendFile(fmt.Sprintf("/wanix/task/%s/ctl", pid), []byte("start")); err != nil {
		log.Fatal(err)
	}

	debug("waiting for exit")
	for {
		b, err := os.ReadFile(fmt.Sprintf("/wanix/task/%s/exit", pid))
		if err != nil {
			log.Fatal(err)
		}
		out := strings.TrimSpace(string(b))
		if out != "" {
			code, err := strconv.Atoi(out)
			if err != nil {
				log.Fatal(err)
			}
			done.Store(1)
			wg.Wait()
			os.Exit(code)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func appendFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
