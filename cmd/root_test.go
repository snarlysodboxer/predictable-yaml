package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"go.yaml.in/yaml/v3"
)

func TestGetConfigNodesByPath(t *testing.T) {
	type testCase struct {
		note                     string
		mkdirs                   []string
		mkfiles                  map[string][]byte
		configDirFlag            string
		filePaths                []string
		workDir                  string
		expectedConfigNodesPaths map[string]string
	}

	testCases := []testCase{
		{
			note: "normal",
			mkdirs: []string{
				"my-repo/kustomize/overlays/asdf/.predictable-yaml",
				"my-repo/.predictable-yaml",
				"my-repo/fdsa/.predictable-yaml",
				".predictable-yaml",
			},
			mkfiles: map[string][]byte{
				"my-repo/kustomize/overlays/asdf/.predictable-yaml/Secret.yaml": []byte(`
kind: Secret
asdf: fdsa  # required
`),
				"my-repo/.predictable-yaml/Secret.yaml": []byte(`
kind: Secret
`),
				"my-repo/fdsa/.predictable-yaml/Secret.yaml": []byte(`
kind: Secret
asdf: fdsa  # required
qwer: rewq
`),
				".predictable-yaml/Secret.yaml": []byte(`
kind: Secret
asdf: fdsa  # required
qwer: rewq
uiop: poiu
`),
			},
			configDirFlag: "",
			filePaths: []string{
				"my-repo/somefile.yaml",
				"my-repo/kustomize/overlays/asdf/someotherfile.yaml",
			},
			workDir: "my-repo",
			expectedConfigNodesPaths: map[string]string{
				"":                                 "my-repo/.predictable-yaml/Secret.yaml",
				"my-repo/kustomize/overlays/asdf/": "my-repo/kustomize/overlays/asdf/.predictable-yaml/Secret.yaml",
			},
		},
		{
			note: "config dir flag",
			mkdirs: []string{
				"my-repo/kustomize/overlays/asdf/.predictable-yaml",
				"my-repo/.predictable-yaml",
				"my-repo/fdsa/.predictable-yaml",
				".predictable-yaml",
			},
			mkfiles: map[string][]byte{
				"my-repo/kustomize/overlays/asdf/.predictable-yaml/Secret.yaml": []byte(`
kind: Secret
asdf: fdsa  # required
`),
				"my-repo/.predictable-yaml/Secret.yaml": []byte(`
kind: Secret
`),
				"my-repo/fdsa/.predictable-yaml/Secret.yaml": []byte(`
kind: Secret
asdf: fdsa  # required
qwer: rewq
`),
				".predictable-yaml/Secret.yaml": []byte(`
kind: Secret
asdf: fdsa  # required
qwer: rewq
uiop: poiu
`),
			},
			configDirFlag: ".predictable-yaml",
			filePaths: []string{
				"my-repo/somefile.yaml",
				"my-repo/kustomize/overlays/asdf/someotherfile.yaml",
			},
			workDir: "my-repo",
			expectedConfigNodesPaths: map[string]string{
				"":                                 ".predictable-yaml/Secret.yaml",
				"my-repo/kustomize/overlays/asdf/": "my-repo/kustomize/overlays/asdf/.predictable-yaml/Secret.yaml",
			},
		},
	}
TestCases:
	for _, tc := range testCases {
		// setup
		tmpDir := t.TempDir()
		err := setupFileSystem(tmpDir, tc.mkdirs, tc.mkfiles)
		if err != nil {
			t.Errorf("Description: %s: cmd.getConfigNodesByPath(...): \n-expected:\n%#v\n+got:\n%s\n", tc.note, nil, err.Error())
			continue
		}
		filePaths := fmtPaths(tmpDir, tc.filePaths)
		expectedConfigNodesByPath := []configNodesByPath{}
		for setPath, loadPath := range tc.expectedConfigNodesPaths {
			if setPath != "" {
				setPath = fmtPath(tmpDir, setPath)
			}
			loadPath = fmtPath(tmpDir, loadPath)
			cNode := &yaml.Node{}
			_, err := getYAML(cNode, loadPath)
			if err != nil {
				t.Errorf("Description: %s: cmd.getConfigNodesByPath(...): \n-expected:\n%#v\n+got:\n%s\n", tc.note, nil, err.Error())
				continue TestCases
			}
			configNode := &compare.Node{Node: cNode}
			compare.WalkConvertYamlNodeToMainNode(configNode)
			compare.WalkParseLoadConfigComments(configNode)
			fileConfigs := compare.GetFileConfigs(configNode)
			expectedConfigNodesByPath = append(expectedConfigNodesByPath, configNodesByPath{
				path: setPath,
				ConfigNodes: compare.ConfigNodes{
					fileConfigs.Kind: configNode,
				},
			})
		}
		sort.SliceStable(expectedConfigNodesByPath, func(i, j int) bool {
			return expectedConfigNodesByPath[i].path < expectedConfigNodesByPath[j].path
		})

		configDirFlag := ""
		if tc.configDirFlag != "" {
			configDirFlag = fmtPath(tmpDir, tc.configDirFlag)
		}

		// do it
		got := getConfigNodesByPath(configDirFlag, fmtPath(tmpDir, tc.workDir), tmpDir, filePaths, nil, "")
		if !reflect.DeepEqual(got, expectedConfigNodesByPath) {
			t.Errorf("Description: %s: cmd.getConfigNodesByPath(...): \n-expected:\n'%#v'\n+got:\n'%#v'\n", tc.note, expectedConfigNodesByPath, got)
		}
	}
}

func TestWalkFindParentConfigDirs(t *testing.T) {
	type testCase struct {
		note               string
		mkdirs             []string
		workDir            string
		expectedConfigDirs []string
	}

	testCases := []testCase{
		{
			note: "normal",
			mkdirs: []string{
				"my-repo/kustomize/overlays/asdf/.predictable-yaml",
				"my-repo/kustomize/overlays/asdf/fdsa/qwer/.predictable-yaml",
				"my-repo/kustomize/overlays/blah/.predictable-yaml",
				"my-repo/kustomize/overlays/fdsa",
				"my-repo/.predictable-yaml",
				".predictable-yaml",
			},
			workDir: "my-repo",
			expectedConfigDirs: []string{
				".predictable-yaml",
				"my-repo/.predictable-yaml",
			},
		},
		{
			note: "subdir",
			mkdirs: []string{
				".predictable-yaml",
				"my-repo/.predictable-yaml",
				"my-repo/kustomize/overlays/fdsa",
				"my-repo/kustomize/overlays/asdf/.predictable-yaml",
				"my-repo/kustomize/overlays/asdf/fdsa/qwer/.predictable-yaml",
				"my-repo/kustomize/overlays/blah/.predictable-yaml",
			},
			workDir: "my-repo/kustomize/overlays/asdf/fdsa/qwer",
			expectedConfigDirs: []string{
				".predictable-yaml",
				"my-repo/.predictable-yaml",
				"my-repo/kustomize/overlays/asdf/.predictable-yaml",
				"my-repo/kustomize/overlays/asdf/fdsa/qwer/.predictable-yaml",
			},
		},
	}
	for _, tc := range testCases {
		// setup
		tmpDir := t.TempDir()
		err := setupFileSystem(tmpDir, tc.mkdirs, map[string][]byte{})
		if err != nil {
			t.Errorf("Description: %s: cmd.walkFindParentConfigDirs(...): \n-expected:\n%#v\n+got:\n%s\n", tc.note, nil, err.Error())
			continue
		}
		expectedConfigDirs := fmtPaths(tmpDir, tc.expectedConfigDirs)

		// do it
		got, err := walkFindParentConfigDirs(fmtPath(tmpDir, tc.workDir), tmpDir, []string{})
		if err != nil {
			t.Errorf("Description: %s: cmd.walkFindParentConfigDirs(...): \n-expected:\n%#v\n+got:\n%s\n", tc.note, nil, err.Error())
			continue
		}
		if !reflect.DeepEqual(got, expectedConfigDirs) {
			t.Errorf("Description: %s: cmd.walkFindParentConfigDirs(...): \n-expected:\n'%#v'\n+got:\n'%#v'\n", tc.note, expectedConfigDirs, got)
		}
	}
}

func TestGetConfigDirsFromFilePaths(t *testing.T) {
	type testCase struct {
		note               string
		mkdirs             []string
		mkfiles            map[string][]byte
		workDir            string
		filePaths          []string // always full file paths, not dirs
		expectedConfigDirs []string
	}

	testCases := []testCase{
		{
			note: "normal",
			mkdirs: []string{
				"kustomize/overlays/asdf/.predictable-yaml",
				"kustomize/overlays/asdf/fdsa/qwer/.predictable-yaml",
				"kustomize/overlays/blah/.predictable-yaml",
				"kustomize/overlays/fdsa",
			},
			mkfiles: map[string][]byte{
				"kustomize/overlays/asdf/.predictable-yaml/Secret.yaml": []byte(`
asdf: fdsa  # predictable-yaml: ignore-required
`,
				),
				"kustomize/overlays/asdf/fdsa/qwer/.predictable-yaml/Secret.yaml": nil,
				"kustomize/overlays/asdf/fdsa/qwer/rewq.yaml":                     nil,
				"kustomize/overlays/asdf/asdf.yaml":                               nil,
				"kustomize/overlays/fdsa/fdsa.yaml":                               nil,
			},
			workDir: "",
			filePaths: []string{
				"kustomize/overlays/asdf",
				"kustomize/overlays/asdf/fdsa/qwer/rewq.yaml",
				"kustomize/overlays/fdsa/fdsa.yaml",
			},
			expectedConfigDirs: []string{
				"kustomize/overlays/asdf/fdsa/qwer/.predictable-yaml",
				"kustomize/overlays/asdf/.predictable-yaml",
			},
		},
	}
	for _, tc := range testCases {
		// setup
		tmpDir := t.TempDir()
		err := setupFileSystem(tmpDir, tc.mkdirs, tc.mkfiles)
		if err != nil {
			t.Errorf("Description: %s: cmd.getConfigDirsFromFilePaths(...): \n-expected:\n%#v\n+got:\n%s\n", tc.note, nil, err.Error())
			continue
		}
		filePaths := fmtPaths(tmpDir, tc.filePaths)
		expectedConfigDirs := fmtPaths(tmpDir, tc.expectedConfigDirs)

		// do it
		got := getConfigDirsFromFilePaths(fmtPath(tmpDir, tc.workDir), tmpDir, filePaths)
		sort.Strings(got)
		sort.Strings(expectedConfigDirs)
		if !reflect.DeepEqual(got, expectedConfigDirs) {
			t.Errorf("Description: %s: cmd.getConfigDirsFromFilePaths(...): \n-expected:\n'%#v'\n+got:\n'%#v'\n", tc.note, expectedConfigDirs, got)
		}
	}
}

func TestGetFilePathParentDirs(t *testing.T) {
	type testCase struct {
		note         string
		workDir      string
		homeDir      string
		filePath     string
		expectedDirs map[string]bool
	}

	testCases := []testCase{
		{
			note:     "normal",
			workDir:  "",
			homeDir:  "",
			filePath: "kustomize/overlays/asdf/fdsa/qwer/rewq.yaml",
			expectedDirs: map[string]bool{
				"kustomize":                         true,
				"kustomize/overlays":                true,
				"kustomize/overlays/asdf":           true,
				"kustomize/overlays/asdf/fdsa":      true,
				"kustomize/overlays/asdf/fdsa/qwer": true,
			},
		},
	}
	for _, tc := range testCases {
		got := getFilePathParentDirs(tc.workDir, tc.homeDir, tc.filePath, map[string]bool{})
		if !reflect.DeepEqual(got, tc.expectedDirs) {
			t.Errorf("Description: %s: cmd.getAllFilePaths(...): \n-expected:\n'%#v'\n+got:\n'%#v'\n", tc.note, tc.expectedDirs, got)
		}
	}
}

func TestGetAllFilePaths(t *testing.T) {
	type testCase struct {
		note              string
		mkdirs            []string
		mkfiles           map[string][]byte
		filePaths         []string
		expectedFilePaths []string
	}

	testCases := []testCase{
		{
			note: "normal",
			mkdirs: []string{
				"kustomize/overlays/asdf",
				"kustomize/overlays/fdsa",
				"kustomize/overlays/blah",
			},
			mkfiles: map[string][]byte{
				"kustomize/overlays/asdf/asdf.yaml": nil,
				"kustomize/overlays/fdsa/fdsa.yaml": nil,
				"kustomize/overlays/fdsa/qwer.yaml": nil,
				"kustomize/overlays/blah/blah.yaml": nil,
			},
			filePaths: []string{
				"kustomize/overlays/asdf",
				"kustomize/overlays/fdsa/fdsa.yaml",
			},
			expectedFilePaths: []string{
				"kustomize/overlays/asdf/asdf.yaml",
				"kustomize/overlays/fdsa/fdsa.yaml",
			},
		},
		{
			note: "does not find config files",
			mkdirs: []string{
				"kustomize/overlays/asdf/.predictable-yaml",
				"kustomize/overlays/fdsa",
				"kustomize/overlays/blah",
			},
			mkfiles: map[string][]byte{
				"kustomize/overlays/asdf/.predictable-yaml/Secret.yaml": nil,
				"kustomize/overlays/asdf/asdf.yaml":                     nil,
				"kustomize/overlays/fdsa/fdsa.yaml":                     nil,
				"kustomize/overlays/fdsa/qwer.yaml":                     nil,
				"kustomize/overlays/blah/blah.yaml":                     nil,
			},
			filePaths: []string{
				"kustomize/overlays/asdf",
				"kustomize/overlays/fdsa/fdsa.yaml",
			},
			expectedFilePaths: []string{
				"kustomize/overlays/asdf/asdf.yaml",
				"kustomize/overlays/fdsa/fdsa.yaml",
			},
		},
		{
			note: "excludes project config file from directory walk and direct path",
			mkdirs: []string{
				"my-project",
			},
			mkfiles: map[string][]byte{
				"my-project/.predictable-yaml.yaml": []byte("version: v1.0.0\n"),
				"my-project/deployment.yaml":        nil,
				"my-project/service.yaml":           nil,
			},
			filePaths: []string{
				"my-project",
				"my-project/.predictable-yaml.yaml",
			},
			expectedFilePaths: []string{
				"my-project/deployment.yaml",
				"my-project/service.yaml",
			},
		},
	}
	for _, tc := range testCases {
		// setup
		tmpDir := t.TempDir()
		err := setupFileSystem(tmpDir, tc.mkdirs, tc.mkfiles)
		if err != nil {
			t.Errorf("Description: %s: cmd.getAllFilePaths(...): \n-expected:\n%#v\n+got:\n%s\n", tc.note, nil, err.Error())
			continue
		}
		filePaths := fmtPaths(tmpDir, tc.filePaths)
		expectedFilePaths := fmtPaths(tmpDir, tc.expectedFilePaths)

		// do it
		got, err := getAllFilePaths(filePaths)
		if err != nil {
			t.Errorf("Description: %s: cmd.getAllFilePaths(...): \n-expected:\n%#v\n+got:\n%s\n", tc.note, nil, err.Error())
			continue
		}
		if !reflect.DeepEqual(got, expectedFilePaths) {
			t.Errorf("Description: %s: cmd.getAllFilePaths(...): \n-expected:\n'%#v'\n+got:\n'%#v'\n", tc.note, expectedFilePaths, got)
		}
	}
}

func TestFindProjectConfig(t *testing.T) {
	type testCase struct {
		note        string
		mkdirs      []string
		mkfiles     map[string][]byte
		workDir     string
		expectFound bool
		expectDir   string // relative to tmpDir
		checkFields func(*testing.T, *ProjectConfig)
	}

	testCases := []testCase{
		{
			note:   "finds config in working dir",
			mkdirs: []string{"my-repo"},
			mkfiles: map[string][]byte{
				"my-repo/.predictable-yaml.yaml": []byte("remote:\n  version: v1.0.0\n"),
			},
			workDir:     "my-repo",
			expectFound: true,
			expectDir:   "my-repo",
			checkFields: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.Remote.Version != "v1.0.0" {
					t.Errorf("expected version v1.0.0, got %s", cfg.Remote.Version)
				}
			},
		},
		{
			note:   "finds config in parent dir",
			mkdirs: []string{"my-repo/subdir"},
			mkfiles: map[string][]byte{
				"my-repo/.predictable-yaml.yaml": []byte("remote:\n  url: https://example.com/repo\n  version: v2.0.0\n"),
			},
			workDir:     "my-repo/subdir",
			expectFound: true,
			expectDir:   "my-repo",
			checkFields: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.Remote.Version != "v2.0.0" {
					t.Errorf("expected version v2.0.0, got %s", cfg.Remote.Version)
				}
				if cfg.Remote.URL != "https://example.com/repo" {
					t.Errorf("expected remote url https://example.com/repo, got %s", cfg.Remote.URL)
				}
			},
		},
		{
			note:   "closest config wins",
			mkdirs: []string{"my-repo/subdir"},
			mkfiles: map[string][]byte{
				".predictable-yaml.yaml":         []byte("remote:\n  version: v1.0.0\n"),
				"my-repo/.predictable-yaml.yaml": []byte("remote:\n  version: v2.0.0\n"),
			},
			workDir:     "my-repo/subdir",
			expectFound: true,
			expectDir:   "my-repo",
			checkFields: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.Remote.Version != "v2.0.0" {
					t.Errorf("expected version v2.0.0 (closer config), got %s", cfg.Remote.Version)
				}
			},
		},
		{
			note:        "no config found",
			mkdirs:      []string{"my-repo"},
			mkfiles:     map[string][]byte{},
			workDir:     "my-repo",
			expectFound: false,
		},
		{
			note:   "finds config in home dir",
			mkdirs: []string{"my-repo/subdir"},
			mkfiles: map[string][]byte{
				".predictable-yaml.yaml": []byte("remote:\n  version: v3.0.0\n"),
			},
			workDir:     "my-repo/subdir",
			expectFound: true,
			expectDir:   "",
			checkFields: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.Remote.Version != "v3.0.0" {
					t.Errorf("expected version v3.0.0, got %s", cfg.Remote.Version)
				}
			},
		},
		{
			note:   "config with fixer settings",
			mkdirs: []string{"my-repo"},
			mkfiles: map[string][]byte{
				"my-repo/.predictable-yaml.yaml": []byte("remote:\n  version: v1.0.0\nfixer:\n  indentation-level: 4\n  compact-lists: false\n"),
			},
			workDir:     "my-repo",
			expectFound: true,
			expectDir:   "my-repo",
			checkFields: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.Fixer.IndentationLevel == nil || *cfg.Fixer.IndentationLevel != 4 {
					t.Errorf("expected indentation-level 4, got %v", cfg.Fixer.IndentationLevel)
				}
				if cfg.Fixer.CompactLists == nil || *cfg.Fixer.CompactLists != false {
					t.Errorf("expected compact-lists false, got %v", cfg.Fixer.CompactLists)
				}
			},
		},
		{
			note:   "config without optional fields leaves them nil",
			mkdirs: []string{"my-repo"},
			mkfiles: map[string][]byte{
				"my-repo/.predictable-yaml.yaml": []byte("remote:\n  version: v1.0.0\n"),
			},
			workDir:     "my-repo",
			expectFound: true,
			expectDir:   "my-repo",
			checkFields: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.Fixer.IndentationLevel != nil {
					t.Errorf("expected indentation-level nil, got %v", cfg.Fixer.IndentationLevel)
				}
				if cfg.Fixer.CompactLists != nil {
					t.Errorf("expected compact-lists nil, got %v", cfg.Fixer.CompactLists)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.note, func(t *testing.T) {
			tmpDir := t.TempDir()
			err := setupFileSystem(tmpDir, tc.mkdirs, tc.mkfiles)
			if err != nil {
				t.Fatalf("setup error: %v", err)
			}

			cfg, foundDir := findProjectConfig(projectConfigFileName, fmtPath(tmpDir, tc.workDir), tmpDir)

			if tc.expectFound {
				if cfg == nil {
					t.Fatalf("expected to find config, got nil")
				}
				expectedDir := fmtPath(tmpDir, tc.expectDir)
				if foundDir != expectedDir {
					t.Errorf("expected config dir %s, got %s", expectedDir, foundDir)
				}
				if tc.checkFields != nil {
					tc.checkFields(t, cfg)
				}
			} else {
				if cfg != nil {
					t.Errorf("expected no config, got %+v", cfg)
				}
			}
		})
	}
}

func TestParseProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Test valid config
	validPath := filepath.Join(tmpDir, "valid.yaml")
	if err := os.WriteFile(validPath, []byte("remote:\n  url: https://example.com/repo\n  version: v1.0.0\nfixer:\n  indentation-level: 4\n  compact-lists: false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseProjectConfig(validPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Remote.URL != "https://example.com/repo" {
		t.Errorf("expected remote url https://example.com/repo, got %s", cfg.Remote.URL)
	}
	if cfg.Remote.Version != "v1.0.0" {
		t.Errorf("expected version v1.0.0, got %s", cfg.Remote.Version)
	}
	if cfg.Fixer.IndentationLevel == nil || *cfg.Fixer.IndentationLevel != 4 {
		t.Errorf("expected indentation-level 4, got %v", cfg.Fixer.IndentationLevel)
	}
	if cfg.Fixer.CompactLists == nil || *cfg.Fixer.CompactLists != false {
		t.Errorf("expected compact-lists false, got %v", cfg.Fixer.CompactLists)
	}

	// Test minimal config (version only)
	minimalPath := filepath.Join(tmpDir, "minimal.yaml")
	if err := os.WriteFile(minimalPath, []byte("remote:\n  version: v1.0.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err = parseProjectConfig(minimalPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Remote.URL != "" {
		t.Errorf("expected empty remote url, got %s", cfg.Remote.URL)
	}
	if cfg.Remote.Version != "v1.0.0" {
		t.Errorf("expected version v1.0.0, got %s", cfg.Remote.Version)
	}
	if cfg.Fixer.IndentationLevel != nil {
		t.Errorf("expected nil indentation-level, got %v", cfg.Fixer.IndentationLevel)
	}
	if cfg.Fixer.CompactLists != nil {
		t.Errorf("expected nil compact-lists, got %v", cfg.Fixer.CompactLists)
	}

	// Test nonexistent file
	_, err = parseProjectConfig(filepath.Join(tmpDir, "nonexistent.yaml"))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func setupFileSystem(tmpDir string, mkdirs []string, mkfiles map[string][]byte) error {
	// make dirs
	for _, path := range mkdirs {
		err := os.MkdirAll(fmtPath(tmpDir, path), 0755)
		if err != nil {
			return err
		}
	}

	// make files
	for path, contents := range mkfiles {
		err := os.WriteFile(fmtPath(tmpDir, path), contents, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

func fmtPaths(tmpDir string, paths []string) []string {
	fPaths := []string{}
	for _, path := range paths {
		fPaths = append(fPaths, fmtPath(tmpDir, path))
	}

	return fPaths
}

func fmtPath(tmpDir, path string) string {
	if path == "" {
		return tmpDir
	}
	return fmt.Sprintf("%s/%s", tmpDir, path)
}
