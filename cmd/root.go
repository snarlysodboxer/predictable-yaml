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
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var cfgDir string
var quiet bool

var rootCmd = &cobra.Command{
	Use:   "predictable-yaml",
	Short: "Lint YAML key order",
	Long: `Compare YAML files to config files, checking for matching
    key order, missing required keys, and first key in sequence or map.`,
	Run: func(cmd *cobra.Command, args []string) {
		// read files in configDir, populate configMap
		configMap := compare.ConfigMap{}
		configFiles, err := ioutil.ReadDir(cfgDir)
		if err != nil {
			log.Fatal(err)
		}
		for _, file := range configFiles {
			if file.IsDir() {
				continue
			}
			re := regexp.MustCompile(`.*\.yaml$`)
			if !re.MatchString(file.Name()) {
				continue
			}
			cNode := &yaml.Node{}
			path := fmt.Sprintf("%s/%s", cfgDir, file.Name())
			err := getYAML(cNode, path)
			if err != nil {
				log.Fatalf("error getting config: %v", err)
			}
			configNode := &compare.Node{Node: cNode}
			compare.WalkConvertYamlNodeToMainNode(configNode)
			compare.WalkParseLoadConfigComments(configNode)
			name, err := compare.GetSchemaType(configNode)
			if err != nil {
				log.Fatalf("error getting file schema: %s: %v", path, err)
			}
			configMap[name] = configNode
		}

		// read files to check, loop and check
		filePaths, err := getFilePathsFromStdin()
		if err != nil {
			log.Fatal(err)
		}
		success := true
		for _, filePath := range filePaths {
			// setup
			fNode := &yaml.Node{}
			err := getYAML(fNode, filePath)
			if err != nil {
				log.Fatalf("error getting file: %s: %v", filePath, err)
			}
			fileNode := &compare.Node{Node: fNode}
			compare.WalkConvertYamlNodeToMainNode(fileNode)
			name, err := compare.GetSchemaType(fileNode)
			if err != nil {
				log.Fatalf("error getting file schema: %s: %v", filePath, err)
			}
			configNode, ok := configMap[name]
			if !ok {
				log.Printf("WARNING: no config found for schema '%s'\n", name)
				continue
			}
			ignoreRequireds := compare.GetIgnoreRequireds(fileNode)

			// do it
			errs := compare.WalkAndCompare(configNode, fileNode, ignoreRequireds, compare.ValidationErrors{})
			if len(errs) != 0 {
				success = false
				log.Printf("File '%s' has validation errors:\n%v", filePath, compare.GetValidationErrorStrings(errs))
			} else {
				if !quiet {
					log.Printf("File '%s' is valid!", filePath)
				}
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

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgDir, "config-dir", "~/.predictable-yaml", "directory containing config file(s), (default is $HOME/.predictable-yaml)")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "output nothing, unless there are failures")
	if strings.HasPrefix(cfgDir, "~/") {
		dirname, _ := os.UserHomeDir()
		cfgDir = filepath.Join(dirname, cfgDir[2:])
	}
}

func getFilePathsFromStdin() ([]string, error) {
	filePaths := make([]string, 0)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if len(text) != 0 {
			filePaths = append(filePaths, text)
		} else {
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return filePaths, err
	}
	if len(filePaths) < 1 {
		return filePaths, errors.New("error: no file paths passed to stdin")
	}

	return filePaths, nil
}

func getYAML(node *yaml.Node, file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, node)
	if err != nil {
		return err
	}

	return nil
}
