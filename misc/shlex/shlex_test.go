package shlex

import (
	"fmt"
	"log"
	"slices"
	"testing"
)

func TestSplitQuotedEmpty(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"a '' b", []string{"a", "", "b"}},
		{"''", []string{""}},
		{"a \"\" b", []string{"a", "", "b"}},
		{"  ", nil},
		{"a b", []string{"a", "b"}},
	} {
		got, err := Split(tc.in, true)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func ExampleSplit() {
	cmd := `cp -Rdp "file name" 'file name2' dir\ name`

	// Split of cmd with POSIX mode.
	words1, err := Split(cmd, true)
	if err != nil {
		log.Fatal(err)
	}
	// Split of cmd with Non-POSIX mode.
	words2, err := Split(cmd, false)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Source command:")
	fmt.Println(`cp -Rdp "file name" 'file name2' dir\ name`)
	fmt.Println()

	fmt.Println("POSIX mode:")
	for _, word := range words1 {
		fmt.Println(word)
	}
	fmt.Println()
	fmt.Println("Non-POSIX mode:")
	for _, word := range words2 {
		fmt.Println(word)
	}

	// Output:
	// Source command:
	// cp -Rdp "file name" 'file name2' dir\ name
	//
	// POSIX mode:
	// cp
	// -Rdp
	// file name
	// file name2
	// dir name
	//
	// Non-POSIX mode:
	// cp
	// -Rdp
	// "file name"
	// 'file name2'
	// dir\
	// name
}
