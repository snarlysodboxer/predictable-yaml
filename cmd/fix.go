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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"github.com/snarlysodboxer/predictable-yaml/pkg/indentation"
	"github.com/snarlysodboxer/predictable-yaml/pkg/moves"
	"github.com/snarlysodboxer/predictable-yaml/pkg/whitespace"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// flags
var (
	prompt                  bool
	promptIfLineCountChange bool
	reduceIndentationBy     int
	indentationLevel        int
	unmatchedToBeginning    bool
	addPreferreds           bool
	validate                bool
	preserveEmptyLines      bool
	preserveComments        bool
	disablePostProcessing   bool
)

// fixCmd represents the fix command
var fixCmd = &cobra.Command{
	Use:   "fix [flags] <file-or-dir-path> ...",
	Short: "Lint YAML key order",
	Long:  `Compare YAML files to config files, reordering keys.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("requires file path argument(s)")
		}
		for _, arg := range args {
			if _, err := os.Stat(arg); errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("file '%s' doesn't exist: %v", arg, err)
			}
		}

		return nil
	},
	Run: func(cmd *cobra.Command, filePaths []string) {
		// setup
		configDirFlag := ""
		if cfgDir != "" {
			configDirFlag = cfgDir
		}
		workDir, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		allFilePaths, err := getAllFilePaths(filePaths)
		if err != nil {
			log.Fatal(err)
		}
		cfgNodesByPaths := getConfigNodesByPath(configDirFlag, workDir, homeDir, allFilePaths)

		success := true

		for _, filePath := range allFilePaths {
			fNode := &yaml.Node{}
			existingFileContents, err := getYAML(fNode, filePath)
			if err != nil {
				log.Fatalf("error parsing yaml for target file: %s: %v", filePath, err)
			}
			fileNode := &compare.Node{Node: fNode}
			compare.WalkConvertYamlNodeToMainNode(fileNode)
			fileConfigs := compare.GetFileConfigs(fileNode)
			if fileConfigs.Ignore {
				continue
			}
			if fileConfigs.Kind == "" {
				log.Printf("WARNING: unable to determine a schema for target file: %s", filePath)
				continue
			}

			configNodes := configNodesForPath(cfgNodesByPaths, filePath)
			configNode, ok := configNodes[fileConfigs.Kind]
			if !ok {
				log.Printf("WARNING: no config found for schema '%s' in file: %s", fileConfigs.Kind, filePath)
				continue
			}

			// do it
			addedFields := []compare.AddedField{}
			sortConfigs := compare.SortConfigs{
				ConfigNodes:          configNodes,
				FileConfigs:          fileConfigs,
				UnmatchedToBeginning: unmatchedToBeginning,
				AddPreferreds:        addPreferreds,
				AddedFields:          &addedFields,
			}
			// check for null values before sorting
			nullErrs := compare.WalkFindNullValues(configNode, fileNode, sortConfigs, compare.ValidationErrors{})
			if len(nullErrs) != 0 {
				success = false
				log.Printf("File '%s' has fix errors:\n%v", filePath, compare.GetValidationErrorStrings(nullErrs))
				continue
			}

			// only sort if validation fails
			if validate {
				errs := compare.WalkAndCompare(configNode, fileNode, sortConfigs, compare.ValidationErrors{})
				if len(errs) == 0 {
					continue
				}
			}

			// parse a separate copy for move summary comparison
			oldFNode := &yaml.Node{}
			_, err = getYAML(oldFNode, filePath)
			if err != nil {
				log.Fatalf("error parsing yaml for target file: %s: %v", filePath, err)
			}
			oldFileNode := &compare.Node{Node: oldFNode}
			compare.WalkConvertYamlNodeToMainNode(oldFileNode)
			commentCount := moves.CountComments(oldFileNode)

			errs := compare.WalkAndSort(configNode, fileNode, sortConfigs, compare.ValidationErrors{})
			if len(errs) != 0 {
				success = false
				log.Printf("File '%s' has fix errors:\n%v", filePath, compare.GetValidationErrorStrings(errs))
				continue
			}

			// setup to check if contents changed, re-add `---` if it was there before
			var buf bytes.Buffer
			firstLine := bytes.Split(existingFileContents, []byte(`\n`))[0]
			hasTripleDash := regexp.MustCompile(`^---\s`).Match(firstLine)
			if hasTripleDash {
				_, err := buf.Write([]byte("---\n"))
				if err != nil {
					log.Printf("Buffer for '%s' has write error:\n%v", filePath, err)
					continue
				}
			}
			encoder := yaml.NewEncoder(&buf)
			encoder.SetIndent(indentationLevel)
			err = encoder.Encode(fileNode.Node)
			if err != nil {
				log.Printf("File '%s' has encode errors:\n%v\n", filePath, err)
				continue
			}
			fileContents := buf.Bytes()

			if !disablePostProcessing {
				if reduceIndentationBy != 0 {
					var err error
					fileContents, err = indentation.FixLists(fileContents, reduceIndentationBy)
					if err != nil {
						log.Println(err)
						continue
					}
				}
				if preserveComments {
					fileContents, err = whitespace.PreserveComments(existingFileContents, fileContents)
					if err != nil {
						log.Println(err)
						continue
					}
				}
				if preserveEmptyLines {
					fileContents, err = whitespace.PreserveEmptyLines(existingFileContents, fileContents)
					if err != nil {
						log.Println(err)
						continue
					}
				}
			}

			// check if contents changed
			fileContentsStr := string(fileContents)
			existingFileContentsStr := string(existingFileContents)
			if fileContentsStr != existingFileContentsStr {
				shouldPrompt := false
				switch {
				case promptIfLineCountChange:
					if countLines(strings.TrimSpace(fileContentsStr), '\n') != countLines(strings.TrimSpace(existingFileContentsStr), '\n') {
						shouldPrompt = true
					}
				case prompt:
					shouldPrompt = true
				}
				doFix := true
				if shouldPrompt {
					// show structural summary
					descriptions := moves.ComputeDescriptions(oldFileNode, fileNode)
					summary := moves.FormatSummary(filePath, descriptions, addedFields, commentCount)
					if summary != "" {
						fmt.Printf("\n%s\n", summary)
					}
					doFix = promptForConfirmation(filePath, existingFileContentsStr, fileContentsStr)
				}

				if !doFix {
					log.Printf("File '%s' has been skipped!", filePath)
					continue
				}
				fileStat, err := os.Stat(filePath)
				if err != nil {
					log.Println(err)
					continue
				}
				err = os.WriteFile(filePath, fileContents, fileStat.Mode())
				if err != nil {
					log.Printf("File '%s' has write errors:\n%v\n", filePath, err)
					continue
				}
				log.Printf("File '%s' has been fixed!", filePath)
			}
		}

		if !success {
			log.Fatal("FAIL")
		}

		if !quiet {
			log.Println("SUCCESS")
		}
	},
}

func init() {
	rootCmd.AddCommand(fixCmd)
	fixCmd.PersistentFlags().BoolVar(&prompt, "prompt", true, "show diff and prompt before making changes")
	fixCmd.PersistentFlags().BoolVar(&promptIfLineCountChange, "prompt-if-line-count-change", false, "show diff and prompt only if the number of lines changed. overrides '--prompt'.")
	fixCmd.PersistentFlags().IntVar(&indentationLevel, "indentation-level", 2, "set yaml.v3 indentation spaces")
	fixCmd.PersistentFlags().IntVar(&reduceIndentationBy, "reduce-list-indentation-by", 2, "reduce indentation level for lists by number")
	fixCmd.PersistentFlags().BoolVar(&unmatchedToBeginning, "unmatched-to-beginning", false, "move keys not in the config to the beginning of their map instead of the end")
	fixCmd.PersistentFlags().BoolVar(&addPreferreds, "add-preferred", false, "add lines marked as preferred when adding missing keys")
	fixCmd.PersistentFlags().BoolVar(&validate, "validate", true, "use validation to determine if sorting should happen. (only sort if validation fails. this can prevent whitespace changes when unnecessary.)")
	fixCmd.PersistentFlags().BoolVar(&preserveEmptyLines, "preserve-empty-lines", true, "preserve empty lines from the original file")
	fixCmd.PersistentFlags().BoolVar(&preserveComments, "preserve-comments", true, "preserve spaces before comments from the original file")
	fixCmd.PersistentFlags().BoolVarP(&disablePostProcessing, "disable-post-processing", "d", false, "disable preserve-empty-lines, preserve-comments, and reduce-list-indentation-by")
	fixCmd.PersistentFlags().BoolVar(&disablePostProcessing, "disable-all-experiments", false, "deprecated: use --disable-post-processing")
	_ = fixCmd.PersistentFlags().MarkDeprecated("disable-all-experiments", "use --disable-post-processing instead")
}

func generateDiff(filePath, oldContent, newContent string) string {
	edits := myers.ComputeEdits(span.URIFromPath(filePath), oldContent, newContent)
	unified := gotextdiff.ToUnified("a/"+filepath.Base(filePath), "b/"+filepath.Base(filePath), oldContent, edits)
	text := fmt.Sprint(unified)

	if !moves.IsTerminal() {
		return text
	}

	return colorizeDiff(text)
}

func colorizeDiff(text string) string {
	const (
		reset = "\033[0m"
		red   = "\033[31m"
		green = "\033[32m"
		cyan  = "\033[36m"
		bold  = "\033[1m"
	)

	var stringBuilder strings.Builder
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			stringBuilder.WriteString(bold + line + reset)
		case strings.HasPrefix(line, "@@"):
			stringBuilder.WriteString(cyan + line + reset)
		case strings.HasPrefix(line, "-"):
			stringBuilder.WriteString(red + line + reset)
		case strings.HasPrefix(line, "+"):
			stringBuilder.WriteString(green + line + reset)
		default:
			stringBuilder.WriteString(line)
		}
		stringBuilder.WriteString("\n")
	}

	return stringBuilder.String()
}

// getExternalDiffTool returns the configured external diff tool command,
// checking environment variables first, then falling back to auto-detection
// of common diff tools on the system.
func getExternalDiffTool() string {
	for _, envVar := range []string{"PREDICTABLE_YAML_DIFF", "KUBECTL_EXTERNAL_DIFF", "DIFFTOOL"} {
		if val := os.Getenv(envVar); val != "" {
			return val
		}
	}

	return detectDiffTool()
}

// detectDiffTool checks for common diff tools in order of preference.
func detectDiffTool() string {
	candidates := []struct {
		bin  string   // binary to look for in PATH
		args []string // full command to use if found
	}{
		{"nvim", []string{"nvim", "-d"}},
		{"vimdiff", []string{"vimdiff"}},
		{"difft", []string{"difft"}},
		{"delta", []string{"delta"}},
		{"code", []string{"code", "--diff", "--wait"}},
		{"meld", []string{"meld"}},
		{"colordiff", []string{"colordiff", "-u"}},
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c.bin); err == nil {
			return strings.Join(c.args, " ")
		}
	}

	return ""
}

// showExternalDiff writes old/new content to temp files and invokes the
// configured external diff tool.
func showExternalDiff(filePath, oldContent, newContent string) error {
	tool := getExternalDiffTool()
	if tool == "" {
		return fmt.Errorf("no external diff tool configured")
	}

	baseName := filepath.Base(filePath)

	oldFile, err := os.CreateTemp("", "predictable-yaml-before-*-"+baseName)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(oldFile.Name())

	newFile, err := os.CreateTemp("", "predictable-yaml-after-*-"+baseName)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(newFile.Name())

	if _, err := oldFile.WriteString(oldContent); err != nil {
		return fmt.Errorf("writing old content: %w", err)
	}
	oldFile.Close()
	if err := os.Chmod(oldFile.Name(), 0444); err != nil {
		return fmt.Errorf("setting temp file read-only: %w", err)
	}

	if _, err := newFile.WriteString(newContent); err != nil {
		return fmt.Errorf("writing new content: %w", err)
	}
	newFile.Close()
	if err := os.Chmod(newFile.Name(), 0444); err != nil {
		return fmt.Errorf("setting temp file read-only: %w", err)
	}

	// Split command, respecting that the tool may have arguments
	parts := strings.Fields(tool)
	args := append(parts[1:], oldFile.Name(), newFile.Name())

	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		// Exit code 1 is normal for diff tools (means differences found)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("running external diff tool: %w", err)
	}

	return nil
}

func countLines(str string, separator rune) int {
	count := 0
	for _, character := range str {
		if character == separator {
			count++
		}
	}

	return count
}

func promptForConfirmation(filePath, oldContent, newContent string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Do you want to write these changes to '%s'?\n", filePath)
		fmt.Println("  y = yes, apply changes")
		fmt.Println("  n = no, skip this file")
		fmt.Println("  d = show built-in diff")
		fmt.Println("  e = show diff in external tool")
		fmt.Print("[y/n/d/e]: ")

		response, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		response = strings.ToLower(strings.TrimSpace(response))

		switch response {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		case "d":
			fmt.Printf("\n%s\n", generateDiff(filePath, oldContent, newContent))
		case "e":
			tool := getExternalDiffTool()
			if tool == "" {
				fmt.Println("No external diff tool configured. Set one of these environment variables:")
				fmt.Println("  PREDICTABLE_YAML_DIFF=\"vimdiff\"")
				fmt.Println("  KUBECTL_EXTERNAL_DIFF=\"code --diff --wait\"")
				fmt.Println("  DIFFTOOL=\"difftastic\"")
			} else {
				if err := showExternalDiff(filePath, oldContent, newContent); err != nil {
					log.Printf("external diff error: %v", err)
				}
			}
		}
	}
}
