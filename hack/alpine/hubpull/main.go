package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	// Docker Hub API endpoints
	dockerRegistry = "https://registry.hub.docker.com"
	dockerAuth     = "https://auth.docker.io/token"
	registryV2     = "https://registry-1.docker.io/v2"
)

// ImageIndex represents a Docker image index (fat manifest)
type ImageIndex struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Manifests     []struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int    `json:"size"`
		Platform  struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
			Variant      string `json:"variant,omitempty"`
		} `json:"platform"`
		Annotations map[string]string `json:"annotations,omitempty"`
	} `json:"manifests"`
}

// ImageManifest represents the Docker image manifest
type ImageManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Size      int    `json:"size"`
		Digest    string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		MediaType string `json:"mediaType"`
		Size      int    `json:"size"`
		Digest    string `json:"digest"`
	} `json:"layers"`
}

func httpClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// getToken obtains an authentication token for Docker Hub
func getToken(image string) (string, error) {
	repo := image
	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}

	url := fmt.Sprintf("%s?service=registry.docker.io&scope=repository:%s:pull", dockerAuth, repo)

	resp, err := httpClient().Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get token: %s", resp.Status)
	}

	var result struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Token, nil
}

// getImageIndex gets the image index (fat manifest) for the Docker image
func getImageIndex(image, tag, token string) (*ImageIndex, error) {
	repo := image
	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}

	url := fmt.Sprintf("%s/%s/manifests/%s", registryV2, repo, tag)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	// Accept OCI and Docker manifest formats
	req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json,application/vnd.docker.distribution.manifest.list.v2+json")

	client := httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get image index: %s", resp.Status)
	}

	var index ImageIndex
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, err
	}

	return &index, nil
}

// getManifest gets the specific manifest for an architecture from the image index
func getManifest(image, digestOrTag, token string, architecture string) (*ImageManifest, error) {
	repo := image
	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}

	// First check if we were given a digest directly
	if !strings.HasPrefix(digestOrTag, "sha256:") {
		// We have a tag, get the image index first
		index, err := getImageIndex(image, digestOrTag, token)
		if err != nil {
			return nil, err
		}

		// Find the manifest for the requested architecture
		var manifestDigest string
		for _, manifest := range index.Manifests {
			// Skip attestation manifests
			if manifest.Platform.Architecture == "unknown" {
				continue
			}

			// If no architecture specified, default to amd64
			if architecture == "" && manifest.Platform.Architecture == "amd64" {
				manifestDigest = manifest.Digest
				break
			}

			if architecture != "" && manifest.Platform.Architecture == architecture {
				manifestDigest = manifest.Digest
				break
			}
		}

		if manifestDigest == "" {
			if architecture == "" {
				architecture = "amd64"
			}
			return nil, fmt.Errorf("no manifest found for architecture: %s", architecture)
		}

		digestOrTag = manifestDigest
	}

	// Now get the specific manifest by digest
	url := fmt.Sprintf("%s/%s/manifests/%s", registryV2, repo, digestOrTag)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json,application/vnd.oci.image.manifest.v1+json")

	client := httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get manifest: %s", resp.Status)
	}

	var manifest ImageManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// downloadLayer downloads a single layer from Docker Hub
func downloadLayer(image, digest, token, outputDir string) (string, error) {
	repo := image
	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}

	url := fmt.Sprintf("%s/%s/blobs/%s", registryV2, repo, digest)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download layer: %s", resp.Status)
	}

	// Create output file
	layerFile := filepath.Join(outputDir, strings.Replace(digest, "sha256:", "", 1)+".tar.gz")
	out, err := os.Create(layerFile)
	if err != nil {
		return "", err
	}
	defer out.Close()

	// Copy the layer content to the file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return layerFile, nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: docker-hub-downloader <image> <tag> [architecture]")
		fmt.Println("Example: docker-hub-downloader nginx latest")
		fmt.Println("Example with architecture: docker-hub-downloader alpine latest arm64")
		os.Exit(1)
	}

	image := os.Args[1]
	tag := os.Args[2]

	// Optional architecture parameter
	architecture := ""
	if len(os.Args) >= 4 {
		architecture = os.Args[3]
	}

	outputDir := fmt.Sprintf("%s-%s", image, tag)
	if architecture != "" {
		outputDir = fmt.Sprintf("%s-%s-%s", image, tag, architecture)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Getting Docker Hub token...")
	token, err := getToken(image)
	if err != nil {
		fmt.Printf("Error getting token: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Getting image manifest...")
	manifest, err := getManifest(image, tag, token, architecture)
	if err != nil {
		fmt.Printf("Error getting manifest: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Image has %d layers\n", len(manifest.Layers))

	var layerFiles []string
	for i, layer := range manifest.Layers {
		fmt.Printf("Downloading layer %d/%d: %s\n", i+1, len(manifest.Layers), layer.Digest)
		layerFile, err := downloadLayer(image, layer.Digest, token, outputDir)
		if err != nil {
			fmt.Printf("Error downloading layer: %v\n", err)
			os.Exit(1)
		}
		layerFiles = append(layerFiles, layerFile)
	}

	fmt.Printf("Layers downloaded to %s directory\n", outputDir)
	fmt.Println("The layers are individual .tar.gz files that make up the Docker image.")

	// Save manifest and config for informational purposes
	manifestFile := filepath.Join(outputDir, "manifest.json")
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(manifestFile, manifestJSON, 0644); err != nil {
		fmt.Printf("Warning: Failed to write manifest file: %v\n", err)
	}

	fmt.Println("\nTo use these layers, you would typically need to process them further into a Docker image tarball format.")
}
