package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			return
		}
		switch strings.TrimSpace(scanner.Text()) {
		case "hello":
			fmt.Println("hi there!")
		case "ping":
			fmt.Println("pong")
		case "exit", "quit":
			fmt.Println("bye")
			return
		case "":
			// ignore
		default:
			fmt.Println("unknown command. try 'hello', 'ping', or 'exit'")
		}
	}
}
