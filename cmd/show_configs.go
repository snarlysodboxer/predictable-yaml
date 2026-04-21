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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"github.com/snarlysodboxer/predictable-yaml/pkg/remote"
	"github.com/spf13/cobra"
)

var showConfigsCmd = &cobra.Command{
	Use:   "show-configs [file-or-dir-path ...]",
	Short: "Show which configs will be used",
	Long:  `Show the config sources and schemas that would be used for the current directory or given paths.`,
	Run: func(cmd *cobra.Command, filePaths []string) {
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

		// Find config dirs
		var configDirs []string
		if configDirFlag != "" {
			configDirs = []string{configDirFlag}
			fmt.Printf("Using --config-dir flag: %s\n", configDirFlag)
		} else {
			configDirs, err = walkFindParentConfigDirs(workDir, homeDir, []string{})
			if err != nil {
				log.Fatal(err)
			}
		}

		if len(configDirs) == 0 && configDirFlag == "" {
			fmt.Println("No .predictable-yaml directories found.")
		}

		// Check each config dir for remote and local configs
		hasRemoteConfig := false
		for _, dir := range configDirs {
			fmt.Printf("\nConfig directory: %s\n", dir)

			// Check for remote config
			remotePath := filepath.Join(dir, remote.RemoteFileName)
			if _, statErr := os.Stat(remotePath); statErr == nil {
				rc, parseErr := remote.ParseRemoteConfig(remotePath)
				if parseErr != nil {
					fmt.Printf("  Remote config: ERROR: %v\n", parseErr)
				} else {
					hasRemoteConfig = true
					fmt.Printf("  Remote: %s @ %s\n", rc.Remote, rc.Version)
					cachePath, cacheErr := rc.CachePath(dir)
					if cacheErr == nil {
						if _, statErr := os.Stat(cachePath); statErr == nil {
							fmt.Printf("  Cache: %s (cached)\n", cachePath)
						} else {
							fmt.Printf("  Cache: not yet fetched\n")
						}
					}
				}
			}

			// Check for local config files
			localNodes := loadLocalConfigNodes(dir)
			if len(localNodes) > 0 {
				fmt.Printf("  Local overrides: %s\n", strings.Join(configNodeKinds(localNodes), ", "))
			}
		}

		// Check for override dirs from file paths
		if len(filePaths) > 0 {
			allFilePaths, err := getAllFilePaths(filePaths)
			if err != nil {
				log.Fatal(err)
			}
			overrideDirs := getConfigDirsFromFilePaths(workDir, homeDir, allFilePaths)
			for _, dir := range overrideDirs {
				fmt.Printf("\nOverride config directory: %s\n", dir)

				remotePath := filepath.Join(dir, remote.RemoteFileName)
				if _, statErr := os.Stat(remotePath); statErr == nil {
					rc, parseErr := remote.ParseRemoteConfig(remotePath)
					if parseErr == nil {
						fmt.Printf("  Remote: %s @ %s\n", rc.Remote, rc.Version)
					}
				}

				localNodes := loadLocalConfigNodes(dir)
				if len(localNodes) > 0 {
					fmt.Printf("  Local overrides: %s\n", strings.Join(configNodeKinds(localNodes), ", "))
				}
			}
		}

		// Embedded defaults
		if !hasRemoteConfig && len(configDirs) == 0 {
			embeddedNodes := loadEmbeddedConfigNodes()
			if len(embeddedNodes) > 0 {
				kinds := configNodeKinds(embeddedNodes)
				fmt.Printf("\nUsing built-in embedded configs (%d schemas): %s\n", len(kinds), strings.Join(kinds, ", "))
			} else {
				fmt.Println("\nNo embedded configs available. Build with 'go generate ./internal/embedded/' to embed configs.")
			}
		}
	},
}

func configNodeKinds(nodes compare.ConfigNodes) []string {
	kinds := make([]string, 0, len(nodes))
	for kind := range nodes {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

func init() {
	rootCmd.AddCommand(showConfigsCmd)
}
