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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/snarlysodboxer/predictable-yaml/internal/embedded"
	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"github.com/snarlysodboxer/predictable-yaml/pkg/remote"
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
	var configDirs []string
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

	// First, load remote configs from parent config dirs (as base layer)
	hasRemoteConfig := false
	for _, dir := range configDirs {
		remoteNodes := loadRemoteConfigNodes(dir)
		if remoteNodes != nil {
			hasRemoteConfig = true
			for kind, node := range remoteNodes {
				configNodes[kind] = node
			}
		}
	}

	// If no remote config and no local configs found, try embedded defaults
	if !hasRemoteConfig && len(configDirs) == 0 {
		embeddedNodes := loadEmbeddedConfigNodes()
		for kind, node := range embeddedNodes {
			configNodes[kind] = node
		}
	}

	// Then, load local config files (override remote configs)
	for _, dir := range configDirs {
		localNodes := loadLocalConfigNodes(dir)
		for kind, node := range localNodes {
			configNodes[kind] = node
		}
	}

	cfgNodesByPaths := []configNodesByPath{{path: "", ConfigNodes: configNodes}}

	// override configNodes from child directories
	overrideConfigDirs := getConfigDirsFromFilePaths(workDir, homeDir, filePaths)
	for _, dir := range overrideConfigDirs {
		configNodes := compare.ConfigNodes{}

		// Load remote configs for this override dir
		remoteNodes := loadRemoteConfigNodes(dir)
		for kind, node := range remoteNodes {
			configNodes[kind] = node
		}

		// Load local config files (override remote)
		localNodes := loadLocalConfigNodes(dir)
		for kind, node := range localNodes {
			configNodes[kind] = node
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

// loadLocalConfigNodes reads local YAML config files from a .predictable-yaml directory.
func loadLocalConfigNodes(dir string) compare.ConfigNodes {
	configNodes := compare.ConfigNodes{}
	configFiles, err := os.ReadDir(dir)
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
		// Skip the .remote file
		if file.Name() == remote.RemoteFileName {
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
		if err := compare.WalkAndValidateConfig(configNode); err != nil {
			log.Fatalf("error validating config file '%s': %v", path, err)
		}
		fileConfigs := compare.GetFileConfigs(configNode)
		if fileConfigs.Kind == "" {
			log.Fatalf("error determining schema for config file: %s: %v", path, err)
		}
		configNodes[fileConfigs.Kind] = configNode
	}

	return configNodes
}

// loadRemoteConfigNodes checks for a .remote file in the config directory,
// fetches/caches the remote configs, and returns the parsed config nodes.
// Returns nil if no .remote file exists.
func loadRemoteConfigNodes(configDir string) compare.ConfigNodes {
	remotePath := filepath.Join(configDir, remote.RemoteFileName)
	if _, err := os.Stat(remotePath); os.IsNotExist(err) {
		return nil
	}

	rc, err := remote.ParseRemoteConfig(remotePath)
	if err != nil {
		log.Fatal(err)
	}

	// Check gitignore warnings
	checkGitignoreWarnings(configDir)

	cachePath, err := remote.FetchIfNeeded(rc, configDir)
	if err != nil {
		log.Fatal(err)
	}

	configNodes := compare.ConfigNodes{}
	configFiles, err := os.ReadDir(cachePath)
	if err != nil {
		log.Fatalf("error reading cached config dir '%s': %v", cachePath, err)
	}
	for _, file := range configFiles {
		if file.IsDir() {
			continue
		}
		if !yamlFileRegex.MatchString(file.Name()) {
			continue
		}
		cNode := &yaml.Node{}
		path := fmt.Sprintf("%s/%s", cachePath, file.Name())
		_, err := getYAML(cNode, path)
		if err != nil {
			log.Fatalf("error parsing yaml for cached config file: %s: %v", path, err)
		}
		configNode := &compare.Node{Node: cNode}
		compare.WalkConvertYamlNodeToMainNode(configNode)
		compare.WalkParseLoadConfigComments(configNode)
		if err := compare.WalkAndValidateConfig(configNode); err != nil {
			log.Fatalf("error validating cached config file '%s': %v", path, err)
		}
		fileConfigs := compare.GetFileConfigs(configNode)
		if fileConfigs.Kind == "" {
			log.Fatalf("error determining schema for cached config file: %s: %v", path, err)
		}
		configNodes[fileConfigs.Kind] = configNode
	}

	return configNodes
}

// loadEmbeddedConfigNodes loads the built-in default config nodes from embedded configs.
// Used as a fallback when no .remote file and no local configs exist.
func loadEmbeddedConfigNodes() compare.ConfigNodes {
	configNodes := compare.ConfigNodes{}
	files, err := embedded.GetConfigFiles()
	if err != nil {
		log.Printf("WARNING: error loading embedded default configs: %v", err)
		return configNodes
	}
	for name, data := range files {
		if !yamlFileRegex.MatchString(name) {
			continue
		}
		cNode := &yaml.Node{}
		if err := yaml.Unmarshal(data, cNode); err != nil {
			log.Printf("WARNING: error parsing embedded config '%s': %v", name, err)
			continue
		}
		configNode := &compare.Node{Node: cNode}
		compare.WalkConvertYamlNodeToMainNode(configNode)
		compare.WalkParseLoadConfigComments(configNode)
		if err := compare.WalkAndValidateConfig(configNode); err != nil {
			log.Printf("WARNING: error validating embedded config '%s': %v", name, err)
			continue
		}
		fileConfigs := compare.GetFileConfigs(configNode)
		if fileConfigs.Kind == "" {
			log.Printf("WARNING: unable to determine schema for embedded config '%s'", name)
			continue
		}
		configNodes[fileConfigs.Kind] = configNode
	}

	return configNodes
}

// checkGitignoreWarnings prints warnings about gitignore state for remote config files.
// These warnings print even with --quiet.
func checkGitignoreWarnings(configDir string) {
	// Find the git root by looking for .git directory
	gitRoot := findGitRoot(configDir)
	if gitRoot == "" {
		return
	}

	// Check if .cache/ is gitignored
	cacheDirRel := getCacheRelPath(gitRoot, configDir)
	if cacheDirRel != "" && !isGitignored(gitRoot, cacheDirRel) {
		log.Printf("WARNING: '%s' is not gitignored. Add it to your .gitignore to avoid committing cached remote configs.", cacheDirRel)
	}

	// Check if .remote is gitignored
	remoteFileRel := getRemoteRelPath(gitRoot, configDir)
	if remoteFileRel != "" && isGitignored(gitRoot, remoteFileRel) {
		log.Printf("WARNING: '%s' is gitignored. This file should be committed so all users of this repo use the same config version.", remoteFileRel)
	}
}

// findGitRoot searches upward from dir to find the directory containing .git.
func findGitRoot(dir string) string {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(absDir, ".git")); err == nil {
			return absDir
		}
		parent := filepath.Dir(absDir)
		if parent == absDir {
			return ""
		}
		absDir = parent
	}
}

// getCacheRelPath returns the relative path from gitRoot to the cache directory.
func getCacheRelPath(gitRoot, configDir string) string {
	absConfigDir, err := filepath.Abs(configDir)
	if err != nil {
		return ""
	}
	cachePath := filepath.Join(absConfigDir, remote.CacheDirName)
	rel, err := filepath.Rel(gitRoot, cachePath)
	if err != nil {
		return ""
	}

	return rel
}

// getRemoteRelPath returns the relative path from gitRoot to the .remote file.
func getRemoteRelPath(gitRoot, configDir string) string {
	absConfigDir, err := filepath.Abs(configDir)
	if err != nil {
		return ""
	}
	remotePath := filepath.Join(absConfigDir, remote.RemoteFileName)
	rel, err := filepath.Rel(gitRoot, remotePath)
	if err != nil {
		return ""
	}

	return rel
}

// isGitignored checks if a path is ignored by git by scanning .gitignore files.
// This is a simple check that looks at the .gitignore in the git root.
func isGitignored(gitRoot, relPath string) bool {
	gitignorePath := filepath.Join(gitRoot, ".gitignore")
	file, err := os.Open(gitignorePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Normalize path separators and check with/without trailing slash
	relPath = filepath.ToSlash(relPath)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Normalize the gitignore pattern
		pattern := filepath.ToSlash(line)
		pattern = strings.TrimPrefix(pattern, "/")
		pattern = strings.TrimSuffix(pattern, "/")
		checkPath := strings.TrimSuffix(relPath, "/")

		if pattern == checkPath {
			return true
		}
		// Also check if a parent directory is ignored
		if strings.HasPrefix(checkPath, pattern+"/") {
			return true
		}
	}

	return false
}

// configNodesForPath returns a proper set of config nodes for a particular file path.
// matches from shortest path to longest path, overriding configs from shorter paths with configs from longer paths.
//
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

// walkFindParentConfigDirs walks up the tree from dir (starting with working directory,) to homeDir or root, returning a list of discovered configDirs.
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
