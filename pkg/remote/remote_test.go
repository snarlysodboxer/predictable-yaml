package remote

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestParseRemoteConfig(t *testing.T) {
	type testCase struct {
		note        string
		content     string
		expected    *RemoteConfig
		expectError bool
	}

	testCases := []testCase{
		{
			note: "tag version, default remote",
			content: `
version: v2.3.0
`,
			expected: &RemoteConfig{
				Remote:  DefaultRemote,
				Version: "v2.3.0",
			},
		},
		{
			note: "tag version with custom remote",
			content: `
remote: https://github.com/my-org/my-configs
version: v1.0.0
`,
			expected: &RemoteConfig{
				Remote:  "https://github.com/my-org/my-configs",
				Version: "v1.0.0",
			},
		},
		{
			note: "commit SHA as version",
			content: `
version: abc123def456
`,
			expected: &RemoteConfig{
				Remote:  DefaultRemote,
				Version: "abc123def456",
			},
		},
		{
			note: "commit SHA with custom remote",
			content: `
remote: https://gitlab.com/my-org/my-configs
version: abc123def456
`,
			expected: &RemoteConfig{
				Remote:  "https://gitlab.com/my-org/my-configs",
				Version: "abc123def456",
			},
		},
		{
			note: "missing version is error",
			content: `
remote: https://github.com/my-org/my-configs
`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tmpDir := t.TempDir()
		remotePath := filepath.Join(tmpDir, RemoteFileName)
		if err := os.WriteFile(remotePath, []byte(tc.content), 0644); err != nil {
			t.Fatalf("Description: %s: error writing test file: %v", tc.note, err)
		}

		got, err := ParseRemoteConfig(remotePath)
		if tc.expectError {
			if err == nil {
				t.Errorf("Description: %s: ParseRemoteConfig: expected error but got nil", tc.note)
			}
			continue
		}
		if err != nil {
			t.Errorf("Description: %s: ParseRemoteConfig: unexpected error: %v", tc.note, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.expected) {
			t.Errorf("Description: %s: ParseRemoteConfig:\n-expected:\n%#v\n+got:\n%#v", tc.note, tc.expected, got)
		}
	}
}

func TestCachePath(t *testing.T) {
	type testCase struct {
		note      string
		rc        *RemoteConfig
		configDir string
		expected  string
	}

	testCases := []testCase{
		{
			note: "github with version",
			rc: &RemoteConfig{
				Remote:  "https://github.com/snarlysodboxer/predictable-yaml-configs",
				Version: "v2.3.0",
			},
			configDir: "/repo/.predictable-yaml",
			expected:  "/repo/.predictable-yaml/.cache/github.com/snarlysodboxer/predictable-yaml-configs/v2.3.0",
		},
		{
			note: "github with commit SHA as version",
			rc: &RemoteConfig{
				Remote:  "https://github.com/my-org/configs",
				Version: "abc123def456",
			},
			configDir: "/repo/.predictable-yaml",
			expected:  "/repo/.predictable-yaml/.cache/github.com/my-org/configs/abc123def456",
		},
		{
			note: "gitlab",
			rc: &RemoteConfig{
				Remote:  "https://gitlab.com/my-org/configs",
				Version: "v1.0.0",
			},
			configDir: "/repo/.predictable-yaml",
			expected:  "/repo/.predictable-yaml/.cache/gitlab.com/my-org/configs/v1.0.0",
		},
		{
			note: "remote with .git suffix",
			rc: &RemoteConfig{
				Remote:  "https://github.com/my-org/configs.git",
				Version: "v1.0.0",
			},
			configDir: "/repo/.predictable-yaml",
			expected:  "/repo/.predictable-yaml/.cache/github.com/my-org/configs/v1.0.0",
		},
	}

	for _, tc := range testCases {
		got, err := tc.rc.CachePath(tc.configDir)
		if err != nil {
			t.Errorf("Description: %s: CachePath: unexpected error: %v", tc.note, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("Description: %s: CachePath:\n-expected: %s\n+got:      %s", tc.note, tc.expected, got)
		}
	}
}

func TestTarballURL(t *testing.T) {
	type testCase struct {
		note      string
		remoteURL string
		ref       string
		expected  string
	}

	testCases := []testCase{
		{
			note:      "github",
			remoteURL: "https://github.com/snarlysodboxer/predictable-yaml-configs",
			ref:       "v2.3.0",
			expected:  "https://github.com/snarlysodboxer/predictable-yaml-configs/archive/v2.3.0.tar.gz",
		},
		{
			note:      "github with .git suffix",
			remoteURL: "https://github.com/my-org/configs.git",
			ref:       "v1.0.0",
			expected:  "https://github.com/my-org/configs/archive/v1.0.0.tar.gz",
		},
		{
			note:      "gitlab",
			remoteURL: "https://gitlab.com/my-org/configs",
			ref:       "v1.0.0",
			expected:  "https://gitlab.com/my-org/configs/-/archive/v1.0.0/configs-v1.0.0.tar.gz",
		},
		{
			note:      "bitbucket",
			remoteURL: "https://bitbucket.org/my-org/configs",
			ref:       "v1.0.0",
			expected:  "https://bitbucket.org/my-org/configs/get/v1.0.0.tar.gz",
		},
		{
			note:      "unknown host falls back to github style",
			remoteURL: "https://gitea.example.com/my-org/configs",
			ref:       "v1.0.0",
			expected:  "https://gitea.example.com/my-org/configs/archive/v1.0.0.tar.gz",
		},
		{
			note:      "commit hash ref",
			remoteURL: "https://github.com/my-org/configs",
			ref:       "abc123def456",
			expected:  "https://github.com/my-org/configs/archive/abc123def456.tar.gz",
		},
	}

	for _, tc := range testCases {
		got, err := tarballURL(tc.remoteURL, tc.ref)
		if err != nil {
			t.Errorf("Description: %s: tarballURL: unexpected error: %v", tc.note, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("Description: %s: tarballURL:\n-expected: %s\n+got:      %s", tc.note, tc.expected, got)
		}
	}
}

func TestExtractTarball(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache")

	tarballData := createTestTarball(t, map[string]string{
		"repo-v1.0.0/Deployment.yaml": "kind: Deployment\nmetadata:\n  name: test\n",
		"repo-v1.0.0/Service.yaml":    "kind: Service\nmetadata:\n  name: test\n",
		"repo-v1.0.0/README.md":       "# Not a YAML file",
		"repo-v1.0.0/sub/nested.yaml": "kind: Nested\n",
	})

	err := extractTarball(bytes.NewReader(tarballData), cachePath)
	if err != nil {
		t.Fatalf("extractTarball: unexpected error: %v", err)
	}

	entries, err := os.ReadDir(cachePath)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	fileNames := []string{}
	for _, e := range entries {
		fileNames = append(fileNames, e.Name())
	}
	sort.Strings(fileNames)

	expected := []string{"Deployment.yaml", "Service.yaml"}
	if !reflect.DeepEqual(fileNames, expected) {
		t.Errorf("extracted files:\n-expected: %v\n+got:      %v", expected, fileNames)
	}

	data, err := os.ReadFile(filepath.Join(cachePath, "Deployment.yaml"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	expectedContent := "kind: Deployment\nmetadata:\n  name: test\n"
	if string(data) != expectedContent {
		t.Errorf("file content:\n-expected: %q\n+got:      %q", expectedContent, string(data))
	}
}

func TestFetchIfNeeded(t *testing.T) {
	requestCount := 0
	tarballData := createTestTarball(t, map[string]string{
		"configs-v1.0.0/Deployment.yaml": "kind: Deployment\nmetadata:\n  name: test\n",
		"configs-v1.0.0/Service.yaml":    "kind: Service\nmetadata:\n  name: test\n",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(tarballData)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".predictable-yaml")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	rc := &RemoteConfig{
		Remote:  fmt.Sprintf("%s/my-org/configs", server.URL),
		Version: "v1.0.0",
	}

	// First fetch — should download
	cachePath, err := FetchIfNeeded(rc, configDir)
	if err != nil {
		t.Fatalf("FetchIfNeeded: unexpected error: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected 1 request, got %d", requestCount)
	}

	entries, err := os.ReadDir(cachePath)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 cached files, got %d", len(entries))
	}

	// Second fetch — should use cache, no new request
	cachePath2, err := FetchIfNeeded(rc, configDir)
	if err != nil {
		t.Fatalf("FetchIfNeeded (cached): unexpected error: %v", err)
	}
	if cachePath2 != cachePath {
		t.Errorf("expected same cache path on second fetch")
	}
	if requestCount != 1 {
		t.Errorf("expected still 1 request after cache hit, got %d", requestCount)
	}
}

func TestCleanOldCacheEntries(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".predictable-yaml")

	rc := &RemoteConfig{
		Remote:  "https://github.com/my-org/configs",
		Version: "v2.0.0",
	}

	// Create old cache entry
	oldCachePath := filepath.Join(configDir, CacheDirName, "github.com", "my-org", "configs", "v1.0.0")
	if err := os.MkdirAll(oldCachePath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldCachePath, "test.yaml"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create current cache entry
	currentCachePath := filepath.Join(configDir, CacheDirName, "github.com", "my-org", "configs", "v2.0.0")
	if err := os.MkdirAll(currentCachePath, 0755); err != nil {
		t.Fatal(err)
	}

	err := cleanOldCacheEntries(rc, configDir)
	if err != nil {
		t.Fatalf("cleanOldCacheEntries: unexpected error: %v", err)
	}

	if _, err := os.Stat(oldCachePath); !os.IsNotExist(err) {
		t.Errorf("expected old cache entry to be removed")
	}

	if _, err := os.Stat(currentCachePath); err != nil {
		t.Errorf("expected current cache entry to still exist: %v", err)
	}
}

func TestGetAuthToken(t *testing.T) {
	origGH := os.Getenv("GITHUB_TOKEN")
	origGL := os.Getenv("GITLAB_TOKEN")
	origBB := os.Getenv("BITBUCKET_TOKEN")
	defer func() {
		os.Setenv("GITHUB_TOKEN", origGH)
		os.Setenv("GITLAB_TOKEN", origGL)
		os.Setenv("BITBUCKET_TOKEN", origBB)
	}()

	os.Setenv("GITHUB_TOKEN", "gh-test-token")
	os.Setenv("GITLAB_TOKEN", "gl-test-token")
	os.Setenv("BITBUCKET_TOKEN", "bb-test-token")

	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/org/repo", "gh-test-token"},
		{"https://gitlab.com/org/repo", "gl-test-token"},
		{"https://bitbucket.org/org/repo", "bb-test-token"},
	}

	for _, tt := range tests {
		got := getAuthToken(tt.url)
		if got != tt.expected {
			t.Errorf("getAuthToken(%s): expected %q, got %q", tt.url, tt.expected, got)
		}
	}
}

func TestFetchIfNeededNetworkError(t *testing.T) {
	// Server that always returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".predictable-yaml")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	rc := &RemoteConfig{
		Remote:  fmt.Sprintf("%s/my-org/configs", server.URL),
		Version: "v1.0.0",
	}

	_, err := FetchIfNeeded(rc, configDir)
	if err == nil {
		t.Fatalf("FetchIfNeeded: expected error for failed fetch, got nil")
	}
}

// createTestTarball creates a gzipped tarball in memory with the given files.
func createTestTarball(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	// Sort keys for deterministic output
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		content := files[name]
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar WriteHeader: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar Write: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}

	return buf.Bytes()
}
