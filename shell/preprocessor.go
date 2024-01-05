package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/anmitsu/go-shlex"
)

// Command Preprocessor for shell input
// Handles things like glob expansion ('*' and '?'),
// environment variables, and shell variables

func preprocess(input string) ([]string, error) {
	args, err := shlex.Split(input, true)

	result := []string{}
	for _, arg := range args {

		if strings.Contains(arg, "*") {
			matches, err := filepath.Glob(arg)
			if err != nil {
				return result, err
			}

			for _, match := range matches {
				result = append(result, match)
			}
			continue
		}

		if arg[0] == '$' {
			val := getEnv(arg[1:])
			result = append(result, val)
			continue
		}

		result = append(result, arg)
	}

	return result, err
}

func getEnv(key string) string {
	env := os.Environ()
	for _, kv := range env {
		split := strings.Split(kv, "=")
		if split[0] == key {
			return split[1]
		}
	}
	return ""
}
