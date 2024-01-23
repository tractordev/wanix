package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Parzival-3141/go-posix"
	"github.com/anmitsu/go-shlex"
)

// Command Preprocessor for shell input.
// Handles things like glob expansion ('*' and '?'),
// environment variables, and shell variables.
func (m *Shell) preprocess(input string) ([]string, error) {
	args, err := shlex.Split(input, true)

	env := os.Environ()
	mapping := posix.Map{}
	for _, kv := range env {
		split := strings.Split(kv, "=")
		mapping[split[0]] = split[1]
	}

	if m.script != nil {
		mapping["0"] = m.script.Name()
		for i, scriptArg := range os.Args[2:] { // skip shell.wasm and script.sh
			mapping[strconv.Itoa(i+1)] = scriptArg
		}
	} else {
		mapping["0"] = "shell"
	}

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

		if strings.ContainsRune(arg, '$') {
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
