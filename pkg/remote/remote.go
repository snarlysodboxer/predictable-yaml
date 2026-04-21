/*
Copyright © 2022 david amick git@davidamick.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package remote

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultRemote  = "https://github.com/snarlysodboxer/predictable-yaml-configs"
	RemoteFileName = ".remote"
	CacheDirName   = ".cache"
)

// RemoteConfig represents the contents of a .predictable-yaml/.remote file.
type RemoteConfig struct {
	Remote  string `yaml:"remote"`
	Version string `yaml:"version"`
}

// ParseRemoteConfig reads and parses a .remote file.
func ParseRemoteConfig(path string) (*RemoteConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading remote config '%s': %w", path, err)
	}

	rc := &RemoteConfig{}
	if err := yaml.Unmarshal(data, rc); err != nil {
		return nil, fmt.Errorf("error parsing remote config '%s': %w", path, err)
	}

	if rc.Version == "" {
		return nil, fmt.Errorf("remote config '%s': 'version' is required (git tag, commit SHA, or branch name)", path)
	}

	if rc.Remote == "" {
		rc.Remote = DefaultRemote
	}

	return rc, nil
}

// CachePath returns the local cache directory path for this remote config.
// Format: {configDir}/.cache/{host}/{owner}/{repo}/{ref}/
func (rc *RemoteConfig) CachePath(configDir string) (string, error) {
	parsed, err := url.Parse(rc.Remote)
	if err != nil {
		return "", fmt.Errorf("error parsing remote URL '%s': %w", rc.Remote, err)
	}

	// e.g. github.com/snarlysodboxer/predictable-yaml-configs/v2.3.0
	repoPath := strings.TrimPrefix(parsed.Path, "/")
	repoPath = strings.TrimSuffix(repoPath, ".git")

	return filepath.Join(configDir, CacheDirName, parsed.Host, repoPath, rc.Version), nil
}

// FetchIfNeeded checks the cache and fetches remote configs if not already cached.
// Returns the path to the cached config directory.
func FetchIfNeeded(rc *RemoteConfig, configDir string) (string, error) {
	cachePath, err := rc.CachePath(configDir)
	if err != nil {
		return "", err
	}

	// Check if cache already exists and has files
	entries, err := os.ReadDir(cachePath)
	if err == nil && len(entries) > 0 {
		return cachePath, nil
	}

	// Clean up any stale cache entries for other versions of the same remote
	if err := cleanOldCacheEntries(rc, configDir); err != nil {
		return "", fmt.Errorf("error cleaning old cache entries: %w", err)
	}

	// Fetch configs
	if err := fetchConfigs(rc, cachePath); err != nil {
		return "", err
	}

	return cachePath, nil
}

// cleanOldCacheEntries removes cached versions of the same remote repo that don't match the current ref.
func cleanOldCacheEntries(rc *RemoteConfig, configDir string) error {
	parsed, err := url.Parse(rc.Remote)
	if err != nil {
		return err
	}

	repoPath := strings.TrimPrefix(parsed.Path, "/")
	repoPath = strings.TrimSuffix(repoPath, ".git")
	repoDir := filepath.Join(configDir, CacheDirName, parsed.Host, repoPath)

	entries, err := os.ReadDir(repoDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != rc.Version {
			if err := os.RemoveAll(filepath.Join(repoDir, entry.Name())); err != nil {
				return err
			}
		}
	}

	return nil
}

// fetchConfigs tries multiple strategies to download the remote configs.
func fetchConfigs(rc *RemoteConfig, cachePath string) error {
	// Strategy 1: unauthenticated tarball download
	err := fetchTarball(rc, cachePath, "")
	if err == nil {
		return nil
	}

	// Strategy 2: authenticated tarball download using environment token
	token := getAuthToken(rc.Remote)
	if token != "" {
		err = fetchTarball(rc, cachePath, token)
		if err == nil {
			return nil
		}
	}

	// Strategy 3: fall back to git commands
	err = fetchViaGit(rc, cachePath)
	if err == nil {
		return nil
	}

	return fmt.Errorf("failed to fetch remote configs from '%s' at ref '%s': all fetch strategies failed", rc.Remote, rc.Version)
}

// tarballURL constructs the tarball download URL based on the hosting provider.
func tarballURL(remoteURL, ref string) (string, error) {
	parsed, err := url.Parse(remoteURL)
	if err != nil {
		return "", fmt.Errorf("error parsing remote URL '%s': %w", remoteURL, err)
	}

	repoPath := strings.TrimPrefix(parsed.Path, "/")
	repoPath = strings.TrimSuffix(repoPath, ".git")

	host := strings.ToLower(parsed.Host)

	switch {
	case strings.Contains(host, "github"):
		return fmt.Sprintf("%s://%s/%s/archive/%s.tar.gz", parsed.Scheme, parsed.Host, repoPath, ref), nil
	case strings.Contains(host, "gitlab"):
		// Get just the last segment for the filename
		segments := strings.Split(repoPath, "/")
		lastSegment := segments[len(segments)-1]
		return fmt.Sprintf("%s://%s/%s/-/archive/%s/%s-%s.tar.gz", parsed.Scheme, parsed.Host, repoPath, ref, lastSegment, ref), nil
	case strings.Contains(host, "bitbucket"):
		return fmt.Sprintf("%s://%s/%s/get/%s.tar.gz", parsed.Scheme, parsed.Host, repoPath, ref), nil
	default:
		// Try GitHub-style URL as default for unknown hosts
		return fmt.Sprintf("%s://%s/%s/archive/%s.tar.gz", parsed.Scheme, parsed.Host, repoPath, ref), nil
	}
}

// fetchTarball downloads and extracts a tarball of the remote configs.
func fetchTarball(rc *RemoteConfig, cachePath, token string) error {
	tbURL, err := tarballURL(rc.Remote, rc.Version)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", tbURL, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error downloading tarball from '%s': %w", tbURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tarball download from '%s' returned status %d", tbURL, resp.StatusCode)
	}

	return extractTarball(resp.Body, cachePath)
}

// extractTarball extracts YAML files from a gzipped tarball into the cache directory.
// Tarballs from GitHub/GitLab/Bitbucket have a top-level directory; we strip it.
func extractTarball(reader io.Reader, cachePath string) error {
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		return fmt.Errorf("error creating cache directory '%s': %w", cachePath, err)
	}

	gz, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("error creating gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tarball: %w", err)
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Only extract YAML files
		name := header.Name
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		// Strip the top-level directory from the tarball path
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		relativePath := parts[1]

		// Skip files in subdirectories — only extract top-level YAML files
		if strings.Contains(relativePath, "/") {
			continue
		}

		destPath := filepath.Join(cachePath, relativePath)
		destFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("error creating file '%s': %w", destPath, err)
		}

		if _, err := io.Copy(destFile, tr); err != nil {
			destFile.Close()
			return fmt.Errorf("error writing file '%s': %w", destPath, err)
		}
		destFile.Close()
	}

	return nil
}

// getAuthToken returns an auth token from environment variables based on the remote URL host.
func getAuthToken(remoteURL string) string {
	parsed, err := url.Parse(remoteURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(parsed.Host)

	switch {
	case strings.Contains(host, "github"):
		return os.Getenv("GITHUB_TOKEN")
	case strings.Contains(host, "gitlab"):
		return os.Getenv("GITLAB_TOKEN")
	case strings.Contains(host, "bitbucket"):
		return os.Getenv("BITBUCKET_TOKEN")
	default:
		// Try common token env vars for unknown hosts
		for _, env := range []string{"GITHUB_TOKEN", "GITLAB_TOKEN", "GIT_TOKEN"} {
			if token := os.Getenv(env); token != "" {
				return token
			}
		}
		return ""
	}
}

// fetchViaGit uses git commands to fetch the remote configs as a fallback.
func fetchViaGit(rc *RemoteConfig, cachePath string) error {
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		return fmt.Errorf("error creating cache directory '%s': %w", cachePath, err)
	}

	// Create a temporary directory for the clone
	tmpDir, err := os.MkdirTemp("", "predictable-yaml-git-*")
	if err != nil {
		return fmt.Errorf("error creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Try shallow clone with --branch first (works for tags and branches)
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", rc.Version, rc.Remote, tmpDir)
	if _, err := cmd.CombinedOutput(); err != nil {
		// Fall back to full clone + checkout (needed for commit SHAs)
		os.RemoveAll(tmpDir)
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("error recreating temp directory: %w", err)
		}
		cmd = exec.Command("git", "clone", rc.Remote, tmpDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git clone failed: %s: %w", string(output), err)
		}
		cmd = exec.Command("git", "-C", tmpDir, "checkout", rc.Version)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git checkout failed: %s: %w", string(output), err)
		}
	}

	// Copy YAML files from the clone to the cache
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return fmt.Errorf("error reading cloned directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		srcPath := filepath.Join(tmpDir, name)
		dstPath := filepath.Join(cachePath, name)

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("error reading '%s': %w", srcPath, err)
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("error writing '%s': %w", dstPath, err)
		}
	}

	return nil
}
