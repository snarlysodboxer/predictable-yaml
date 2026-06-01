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
package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// findRepoRoot walks up from the current directory to find the repo root
// by looking for go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}

// buildBinary builds the predictable-yaml binary into a temp directory
// and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "predictable-yaml")
	repoRoot := findRepoRoot(t)
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}
	return binary
}

func TestIntegrationLint(t *testing.T) {
	binary := buildBinary(t)
	repoRoot := findRepoRoot(t)
	configDir := filepath.Join(repoRoot, "example-configs")

	type testCase struct {
		note           string
		files          []string
		expectFail     bool
		expectInOutput string
	}

	testCases := []testCase{
		{
			note:       "valid deployment passes",
			files:      []string{filepath.Join(repoRoot, "test-data", "deployment.valid.yaml")},
			expectFail: false,
		},
		{
			note:           "invalid deployment fails",
			files:          []string{filepath.Join(repoRoot, "test-data", "deployment.invalid.yaml")},
			expectFail:     true,
			expectInOutput: "Changes:",
		},
		{
			note:       "valid service passes",
			files:      []string{filepath.Join(repoRoot, "test-data", "service.valid.yaml")},
			expectFail: false,
		},
		{
			note:           "invalid service fails",
			files:          []string{filepath.Join(repoRoot, "test-data", "service.invalid.yaml")},
			expectFail:     true,
			expectInOutput: "Changes:",
		},
		{
			note: "mixed valid and invalid fails",
			files: []string{
				filepath.Join(repoRoot, "test-data", "deployment.valid.yaml"),
				filepath.Join(repoRoot, "test-data", "service.invalid.yaml"),
			},
			expectFail:     true,
			expectInOutput: "Changes:",
		},
		{
			note:       "directory with invalid files fails",
			files:      []string{filepath.Join(repoRoot, "test-data")},
			expectFail: true, // directory contains invalid files too
		},
	}

	for _, tc := range testCases {
		t.Run(tc.note, func(t *testing.T) {
			args := []string{"lint", "--config-dir", configDir}
			args = append(args, tc.files...)
			cmd := exec.Command(binary, args...)
			out, err := cmd.CombinedOutput()
			output := string(out)

			if tc.expectFail {
				if err == nil {
					t.Errorf("expected failure but got success\noutput: %s", output)
				}
			} else {
				if err != nil {
					t.Errorf("expected success but got failure\noutput: %s", output)
				}
			}

			if tc.expectInOutput != "" && !strings.Contains(output, tc.expectInOutput) {
				t.Errorf("expected output to contain %q\noutput: %s", tc.expectInOutput, output)
			}
		})
	}
}

func TestIntegrationFix(t *testing.T) {
	binary := buildBinary(t)
	repoRoot := findRepoRoot(t)
	configDir := filepath.Join(repoRoot, "example-configs")

	type testCase struct {
		note         string
		sourceFile   string
		expectedFile string // if set, compare fixed output against this file
	}

	testCases := []testCase{
		{
			note:       "valid deployment unchanged",
			sourceFile: filepath.Join(repoRoot, "test-data", "deployment.valid.yaml"),
		},
		{
			note:         "invalid deployment gets fixed",
			sourceFile:   filepath.Join(repoRoot, "test-data", "deployment.invalid.yaml"),
			expectedFile: filepath.Join(repoRoot, "test-data", "deployment.invalid-fixed.yaml"),
		},
		{
			note:       "valid service unchanged",
			sourceFile: filepath.Join(repoRoot, "test-data", "service.valid.yaml"),
		},
		{
			note:         "invalid service gets fixed",
			sourceFile:   filepath.Join(repoRoot, "test-data", "service.invalid.yaml"),
			expectedFile: filepath.Join(repoRoot, "test-data", "service.invalid-fixed.yaml"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.note, func(t *testing.T) {
			// Copy the file to a temp location so we don't modify test data
			original, err := os.ReadFile(tc.sourceFile)
			if err != nil {
				t.Fatal(err)
			}
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, filepath.Base(tc.sourceFile))
			if err := os.WriteFile(tmpFile, original, 0644); err != nil {
				t.Fatal(err)
			}

			// Run fix without prompting
			args := []string{"fix", "--config-dir", configDir, "--prompt=false", tmpFile}
			cmd := exec.Command(binary, args...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("fix command failed: %v\noutput: %s", err, out)
			}

			fixed, err := os.ReadFile(tmpFile)
			if err != nil {
				t.Fatal(err)
			}

			if tc.expectedFile != "" {
				// Compare against expected fixed output
				expected, err := os.ReadFile(tc.expectedFile)
				if err != nil {
					t.Fatal(err)
				}
				if string(fixed) != string(expected) {
					t.Errorf("fixed output does not match expected file %s\ngot:\n%s\nexpected:\n%s", tc.expectedFile, fixed, expected)
				}
			} else {
				// Expect no changes
				if string(fixed) != string(original) {
					t.Errorf("expected file to be unchanged but it was modified\noriginal:\n%s\nfixed:\n%s", original, fixed)
				}
			}

			// Fixed files should always pass lint
			if tc.expectedFile != "" {
				lintArgs := []string{"lint", "--config-dir", configDir, tmpFile}
				lintCmd := exec.Command(binary, lintArgs...)
				lintOut, lintErr := lintCmd.CombinedOutput()
				if lintErr != nil {
					t.Errorf("fixed file fails lint: %v\noutput: %s", lintErr, lintOut)
				}
			}
		})
	}
}

func TestIntegrationFixIdempotent(t *testing.T) {
	binary := buildBinary(t)
	repoRoot := findRepoRoot(t)
	configDir := filepath.Join(repoRoot, "example-configs")

	// Fix an invalid file, then fix again - result should be identical
	original, err := os.ReadFile(filepath.Join(repoRoot, "test-data", "deployment.invalid.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "deployment.invalid.yaml")
	if err := os.WriteFile(tmpFile, original, 0644); err != nil {
		t.Fatal(err)
	}

	// First fix
	args := []string{"fix", "--config-dir", configDir, "--prompt=false", tmpFile}
	cmd := exec.Command(binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("first fix failed: %v\noutput: %s", err, out)
	}
	firstFix, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	// Second fix
	cmd = exec.Command(binary, args...)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("second fix failed: %v\noutput: %s", err, out)
	}
	secondFix, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	if string(firstFix) != string(secondFix) {
		t.Errorf("fix is not idempotent\nfirst fix:\n%s\nsecond fix:\n%s", firstFix, secondFix)
	}
}
