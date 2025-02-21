package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: wexec <command> [args...]")
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	args := os.Args[1:]
	args[0] = strings.TrimPrefix(filepath.Join(wd, args[0]), "/wanix/")

	pidRaw, err := os.ReadFile("/wanix/proc/new/wasi")
	if err != nil {
		log.Fatal(err)
	}
	pid := strings.TrimSpace(string(pidRaw))

	if err := appendFile(fmt.Sprintf("/wanix/proc/%s/cmd", pid), []byte(strings.Join(args, " "))); err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			b, err := os.ReadFile(fmt.Sprintf("/wanix/proc/%s/fd/1", pid))
			if err != nil {
				log.Fatal(err)
			}
			os.Stdout.Write(b)
			time.Sleep(200 * time.Millisecond)
		}
	}()

	if err := appendFile(fmt.Sprintf("/wanix/proc/%s/ctl", pid), []byte("start")); err != nil {
		log.Fatal(err)
	}

	for {
		b, err := os.ReadFile(fmt.Sprintf("/wanix/proc/%s/exit", pid))
		if err != nil {
			log.Fatal(err)
		}
		out := strings.TrimSpace(string(b))
		if out != "" {
			code, err := strconv.Atoi(out)
			if err != nil {
				log.Fatal(err)
			}
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
