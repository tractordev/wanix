package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Parse command line flags
	inputFile := flag.String("input", "", "Input gzipped tar file")
	outputFile := flag.String("output", "", "Output tar file")
	flag.Parse()

	if *inputFile == "" || *outputFile == "" {
		fmt.Println("Usage: program -input input.tar.gz -output output.tar")
		os.Exit(1)
	}

	// Open the input file
	in, err := os.Open(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening input file: %v\n", err)
		os.Exit(1)
	}
	defer in.Close()

	// Create a gzip reader
	gzReader, err := gzip.NewReader(in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating gzip reader: %v\n", err)
		os.Exit(1)
	}
	defer gzReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzReader)

	// Create the output file
	out, err := os.Create(*outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	// Create a tar writer directly on the output file (removed gzip writer)
	tarWriter := tar.NewWriter(out)
	defer tarWriter.Close()

	// Process each file in the tar
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading tar: %v\n", err)
			os.Exit(1)
		}

		// Check if it's a symlink with an absolute path
		if header.Typeflag == tar.TypeSymlink && strings.HasPrefix(header.Linkname, "/") {
			// Get the directory of the symlink
			symlinkDir := filepath.Dir(header.Name)

			// Convert absolute linkname to relative
			relLinkname, err := makeRelative(header.Linkname, symlinkDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error converting symlink %s -> %s: %v\n",
					header.Name, header.Linkname, err)
				continue
			}

			fmt.Printf("Converting symlink %s: %s -> %s\n", header.Name, header.Linkname, relLinkname)
			header.Linkname = relLinkname
		}

		// Write the header
		if err := tarWriter.WriteHeader(header); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing header: %v\n", err)
			os.Exit(1)
		}

		// If it's a regular file, copy the contents
		if header.Typeflag == tar.TypeReg {
			if _, err := io.Copy(tarWriter, tarReader); err != nil {
				fmt.Fprintf(os.Stderr, "Error copying file contents: %v\n", err)
				os.Exit(1)
			}
		}
	}

	fmt.Println("Processing complete")
}

// makeRelative converts an absolute path to a relative path
// from the perspective of the source directory
func makeRelative(target, sourceDir string) (string, error) {
	if !strings.HasPrefix(target, "/") {
		return target, nil // Already relative
	}

	// Normalize paths
	targetPath := filepath.Clean(target)
	sourcePath := filepath.Clean("/" + sourceDir)

	// Split paths into components
	targetComponents := strings.Split(targetPath, string(filepath.Separator))
	sourceComponents := strings.Split(sourcePath, string(filepath.Separator))

	// Remove empty components
	targetComponents = removeEmpty(targetComponents)
	sourceComponents = removeEmpty(sourceComponents)

	// Find common prefix
	commonLen := 0
	minLen := min(len(targetComponents), len(sourceComponents))
	for i := 0; i < minLen; i++ {
		if targetComponents[i] != sourceComponents[i] {
			break
		}
		commonLen++
	}

	// Build relative path
	var relPath []string

	// Add "../" for each level we need to go up
	for i := 0; i < len(sourceComponents)-commonLen; i++ {
		relPath = append(relPath, "..")
	}

	// Add the remaining target path components
	relPath = append(relPath, targetComponents[commonLen:]...)

	return strings.Join(relPath, "/"), nil
}

func removeEmpty(s []string) []string {
	var result []string
	for _, str := range s {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
