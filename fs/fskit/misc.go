package fskit

import (
	"sort"
	"strings"
)

func MatchPaths(paths []string, name string) (matches []string) {
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i]) > len(paths[j])
	})
	for _, path := range paths {
		if strings.HasPrefix(name, path) || path == "." {
			matches = append(matches, path)
		}
	}
	return
}
