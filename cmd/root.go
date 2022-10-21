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
	"sort"
	"strings"

	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const configDirName = ".predictable-yaml"

var (
	cfgDir        string
	quiet         bool
	yamlFileRegex = regexp.MustCompile(`(.*\.yaml$|.*\.yml$)`)
)

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
	rootCmd.PersistentFlags().StringVar(&cfgDir, "config-dir", "", fmt.Sprintf("directory containing config file(s), (default is $HOME/%s)", configDirName))
	if strings.HasPrefix(cfgDir, "~/") {
		dirname, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		cfgDir = filepath.Join(dirname, cfgDir[2:])
	}
}

// configNodesByPath allows using a slice to maintain order of configNodes
type configNodesByPath struct {
	path string
	compare.ConfigNodes
}

// getConfigNodesByPath reads files in configDirFlag or found config dir(s), populating []configNodesByPath
func getConfigNodesByPath(configDirFlag, workDir, homeDir string, filePaths []string) []configNodesByPath {
	configDirs := []string{}
	if configDirFlag != "" {
		configDirs = []string{configDirFlag}
	} else {
		var err error
		configDirs, err = walkFindParentConfigDirs(workDir, homeDir, []string{})
		if err != nil {
			log.Fatal(err)
		}
	}

	// regular configNodes
	// override more root configs with more specific configs
	configNodes := compare.ConfigNodes{}
	for _, dir := range configDirs {
		configFiles, err := ioutil.ReadDir(dir)
		if err != nil {
			log.Fatalf("error reading dir '%s': %v", dir, err)
		}
		for _, file := range configFiles {
			if file.IsDir() {
				continue
			}
			if !yamlFileRegex.MatchString(file.Name()) {
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
			configNodes[fileConfigs.Kind] = configNode
		}
	}

	cfgNodesByPaths := []configNodesByPath{{path: "", ConfigNodes: configNodes}}

	// override configNodes
	overrideConfigDirs := getConfigDirsFromFilePaths(workDir, homeDir, filePaths)
	for _, dir := range overrideConfigDirs {
		configFiles, err := ioutil.ReadDir(dir)
		if err != nil {
			log.Fatal(err)
		}

		configNodes := compare.ConfigNodes{}
		for _, file := range configFiles {
			if file.IsDir() {
				continue
			}
			if !yamlFileRegex.MatchString(file.Name()) {
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
			configNodes[fileConfigs.Kind] = configNode
		}
		cfgNodesByPaths = append(cfgNodesByPaths, configNodesByPath{
			path:        strings.ReplaceAll(dir, configDirName, ""),
			ConfigNodes: configNodes,
		})
	}

	sort.SliceStable(cfgNodesByPaths, func(i, j int) bool {
		return cfgNodesByPaths[i].path < cfgNodesByPaths[j].path
	})

	return cfgNodesByPaths
}

// configNodesForPath returns a proper set of config nodes for a particular file path.
// matches from shortest path to longest path, overriding
//   configs from shorter paths with configs from longer paths.
// expects sorted []configNodesByPath.
func configNodesForPath(cfgNodesByPaths []configNodesByPath, filePath string) compare.ConfigNodes {
	configNodes := cfgNodesByPaths[0].ConfigNodes
	for _, cfgNodeByPath := range cfgNodesByPaths {
		if !strings.Contains(filePath, cfgNodeByPath.path) {
			continue
		}
		for kind, cN := range cfgNodeByPath.ConfigNodes {
			configNodes[kind] = cN
		}
	}

	return configNodes
}

func getYAML(node *yaml.Node, file string) ([]byte, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return data, fmt.Errorf("error reading '%s': %w", file, err)
	}

	err = yaml.Unmarshal(data, node)
	if err != nil {
		return data, fmt.Errorf("error unmarshaling '%s': %w", file, err)
	}

	return data, nil
}

// walkFindParentConfigDirs walks up the tree from dir (starting with working directory,)
//   to homeDir or root, returning a list of discovered configDirs.
func walkFindParentConfigDirs(dir, homeDir string, configDirs []string) ([]string, error) {
	configDir := fmt.Sprintf("%s/%s", dir, configDirName)
	_, err := os.Stat(configDir)
	if err == nil {
		configDirs = append(configDirs, configDir)
	} else {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error reading config dir '%s': %w", configDir, err)
		}
	}
	if dir == homeDir {
		return configDirs, nil
	}
	parentDir := filepath.Dir(dir)
	if parentDir == dir {
		// reached root
		return configDirs, nil
	}
	configDirs, err = walkFindParentConfigDirs(parentDir, homeDir, configDirs)
	if err != nil {
		return nil, err
	}

	// sort from shortest to longest absolute path
	sort.Strings(configDirs)

	return configDirs, nil
}

func getConfigDirsFromFilePaths(workDir, homeDir string, filePaths []string) []string {
	dirs := map[string]bool{}
	for _, filePath := range filePaths {
		dirs = getFilePathParentDirs(workDir, homeDir, filePath, dirs)
	}

	configDirs := []string{}
	for dir := range dirs {
		configDir := fmt.Sprintf("%s/%s", dir, configDirName)
		_, err := os.Stat(configDir)
		if err == nil {
			configDirs = append(configDirs, configDir)
		}
	}

	return configDirs
}

func getFilePathParentDirs(workDir, homeDir, filePath string, dirs map[string]bool) map[string]bool {
	parent := filepath.Dir(filePath)
	if parent == "." || parent == "/" || parent == homeDir || parent == workDir {
		return dirs
	}
	dirs[parent] = true

	return getFilePathParentDirs(workDir, homeDir, parent, dirs)
}

// getAllFilePaths checks for paths that are directories, searching them for yaml files
func getAllFilePaths(filePaths []string) ([]string, error) {
	allFilePaths := []string{}
	for _, filePath := range filePaths {
		fileStat, err := os.Stat(filePath)
		if err != nil {
			return filePaths, err
		}
		if !fileStat.IsDir() {
			allFilePaths = append(allFilePaths, filePath)
			continue
		}
		err = filepath.Walk(filePath, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && yamlFileRegex.MatchString(info.Name()) {
				// make sure we're not returning any config files
				if !strings.Contains(p, configDirName) {
					allFilePaths = append(allFilePaths, p)
				}
			}
			return nil
		})
		if err != nil {
			return filePaths, err
		}
	}

	return allFilePaths, nil
}
