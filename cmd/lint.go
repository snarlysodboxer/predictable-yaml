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
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// lintCmd represents the lint command
var lintCmd = &cobra.Command{
	Use:   "lint [flags] <file-path> ...",
	Short: "Lint YAML key order",
	Long: `Compare YAML files to config files, checking for matching
    key order, missing required keys, and first key in sequence or map.`,
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
		configMap := getConfigMap()

		success := true
		for _, filePath := range filePaths {
			// setup
			fNode := &yaml.Node{}
			_, err := getYAML(fNode, filePath)
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
				log.Fatalf("error: unable to determine a schema for target file: %s", filePath)
			}
			configNode, ok := configMap[fileConfigs.Kind]
			if !ok {
				log.Printf("WARNING: no config found for schema '%s' in file: %s", fileConfigs.Kind, filePath)
				continue
			}

			// do it
			sortConfigs := compare.SortConfigs{
				ConfigMap:   configMap,
				FileConfigs: fileConfigs,
			}
			errs := compare.WalkAndCompare(configNode, fileNode, sortConfigs, compare.ValidationErrors{})
			if len(errs) != 0 {
				success = false
				log.Printf("File '%s' has validation errors:\n%v", filePath, compare.GetValidationErrorStrings(errs))
				continue
			}
			if !quiet {
				log.Printf("File '%s' is valid!", filePath)
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
	rootCmd.AddCommand(lintCmd)
	lintCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "shush success messages")
}
