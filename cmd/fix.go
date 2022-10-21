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
	"regexp"
	"strings"

	"github.com/kylelemons/godebug/diff"
	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"github.com/snarlysodboxer/predictable-yaml/pkg/indentation"
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
	disableAllExperiments   bool
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
			sortConfigs := compare.SortConfigs{
				ConfigNodes:          configNodes,
				FileConfigs:          fileConfigs,
				UnmatchedToBeginning: unmatchedToBeginning,
				AddPreferreds:        addPreferreds,
			}
			// only sort if validation fails
			if validate {
				errs := compare.WalkAndCompare(configNode, fileNode, sortConfigs, compare.ValidationErrors{})
				if len(errs) == 0 {
					continue
				}
			}

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

			if !disableAllExperiments {
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
					fmt.Printf("\n%s\n", diff.Diff(existingFileContentsStr, fileContentsStr))
					doFix = promptForConfirmation(fmt.Sprintf("Do you want to write these changes to '%s'?", filePath))
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
	fixCmd.PersistentFlags().BoolVar(&unmatchedToBeginning, "unmatched-to-beginning", false, "show diff and prompt only if the number of lines changed. overrides '--prompt'.")
	fixCmd.PersistentFlags().BoolVar(&addPreferreds, "add-preferred", false, "add lines marked as preferred when adding missing keys")
	fixCmd.PersistentFlags().BoolVar(&validate, "validate", true, "use validation to determine if sorting should happen. (only sort if validation fails. this can prevent whitespace changes when unnecessary.)")
	fixCmd.PersistentFlags().BoolVar(&preserveEmptyLines, "preserve-empty-lines", true, "attempt to preserve empty lines that are in the original")
	fixCmd.PersistentFlags().BoolVar(&preserveComments, "preserve-comments", true, "attempt to preserve spaces before comments")
	fixCmd.PersistentFlags().BoolVarP(&disableAllExperiments, "disable-all-experiments", "d", false, "disable all experimental features. preserve-empty-lines, preserve-comments, and reduce-list-indentation-by.")
}

func countLines(str string, r rune) int {
	count := 0
	for _, c := range str {
		if c == r {
			count++
		}
	}
	return count
}

func promptForConfirmation(str string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/n]: ", str)

		response, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		response = strings.ToLower(strings.TrimSpace(response))
		fmt.Println(response)

		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" {
			return false
		}
	}
}
