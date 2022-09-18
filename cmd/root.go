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
	Use:   "predictable-yaml <command>",
	Short: "Lint or fix YAML key order",
	Long:  `Compare YAML files to config files.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgDir, "config-dir", "", "directory containing config file(s), (default is $HOME/.predictable-yaml)")
	if strings.HasPrefix(cfgDir, "~/") {
		dirname, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		cfgDir = filepath.Join(dirname, cfgDir[2:])
	}
}

// getConfigMap reads files in configDir(s), populating a configMap
func getConfigMap() compare.ConfigMap {
	configDirs, err := getConfigDirs()
	if err != nil {
		log.Fatal(err)
	}

	configMap := compare.ConfigMap{}
	for _, dir := range configDirs {
		configFiles, err := ioutil.ReadDir(dir)
		if err != nil {
			log.Fatal(err)
		}
		for _, file := range configFiles {
			if file.IsDir() {
				continue
			}
			re := regexp.MustCompile(`(.*\.yaml$|.*\.yml$)`)
			if !re.MatchString(file.Name()) {
				continue
			}
			cNode := &yaml.Node{}
			path := fmt.Sprintf("%s/%s", dir, file.Name())
			_, err := getYAML(cNode, path)
			if err != nil {
				log.Fatalf("error parsing yaml for config file: %s: %v", path, err)
			}
			configNode := &compare.Node{Node: cNode}
			compare.WalkConvertYamlNodeToMainNode(configNode)
			compare.WalkParseLoadConfigComments(configNode)
			fileConfigs := compare.GetFileConfigs(configNode)
			if fileConfigs.Kind == "" {
				log.Fatalf("error determining schema for config file: %s: %v", path, err)
			}
			if _, ok := configMap[fileConfigs.Kind]; !ok {
				configMap[fileConfigs.Kind] = configNode
			}
		}
	}

	return configMap
}

func getConfigDirs() ([]string, error) {
	if cfgDir != "" {
		return []string{cfgDir}, nil
	}
	workDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	configDirs, err := walkFindConfigDirs(workDir, []string{})
	if err != nil {
		return nil, err
	}

	return configDirs, nil
}

func walkFindConfigDirs(dir string, configDirs []string) ([]string, error) {
	configDir := fmt.Sprintf("%s/.predictable-yaml", dir)
	_, err := os.Stat(configDir)
	if err == nil {
		configDirs = append(configDirs, configDir)
	} else {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	if dir == homeDir {
		return configDirs, nil
	}
	parentDir := filepath.Dir(dir)
	configDirs, err = walkFindConfigDirs(parentDir, configDirs)
	if err != nil {
		return nil, err
	}

	return configDirs, nil
}

func getYAML(node *yaml.Node, file string) ([]byte, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return data, err
	}

	err = yaml.Unmarshal(data, node)
	if err != nil {
		return data, err
	}

	return data, nil
}
