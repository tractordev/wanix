package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/anmitsu/go-shlex"
	"github.com/mgood/go-posix"
)

// Command Preprocessor for shell input
// Handles things like glob expansion ('*' and '?'),
// environment variables, and shell variables

func preprocess(input string) ([]string, error) {
	args, err := shlex.Split(input, true)

	mapping := envMap()

	result := []string{}
	for _, arg := range args {

		if strings.ContainsAny(arg, "*?[") {
			matches, err := filepath.Glob(arg)
			if err != nil {
				return result, err
			}
			// add expanded glob into result
			for _, match := range matches {
				result = append(result, match)
			}
			continue
		}

		if arg[0] == '$' {
			val, err := posix.Expand(arg, mapping)
			if err != nil {
				return result, err
			}
			result = append(result, val)
			continue
		}

		result = append(result, arg)
	}

	return result, err
}

func envMap() posix.Map {
	env := os.Environ()
	mapping := map[string]string{}
	for _, kv := range env {
		split := strings.Split(kv, "=")
		k, v := split[0], split[1]
		mapping[k] = v
	}
	return posix.Map(mapping)
}
