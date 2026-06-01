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
package compare

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Node is a custom yaml.Node
type Node struct {
	*yaml.Node
	NodeContent []*Node
	ParentNode  *Node
	Index       int
	MustBeFirst bool
	Required    bool
	Preferred   bool
	Ditto       string
}

// ConfigNodes is a map of names to Config Nodes
type ConfigNodes map[string]*Node

// ValidationErrors allows us to return multiple validation errors
type ValidationErrors []error

// FileConfigs supports kind and overrides
type FileConfigs struct {
	Kind            string // config and target files
	Ignore          bool   // target files only
	IgnoreRequireds bool   // target files only
}

// AddedField represents a required field that was added during sorting.
type AddedField struct {
	Path string // parent path, e.g. ".metadata.labels"
	Key  string // key name, e.g. "app.kubernetes.io/name"
}

func (f AddedField) String() string {
	return f.Path + "." + f.Key
}

// SortConfigs represent various configs for a sorting operation
type SortConfigs struct {
	ConfigNodes          ConfigNodes
	FileConfigs          FileConfigs
	UnmatchedToBeginning bool
	AddPreferreds        bool
	AddedFields          *[]AddedField
}

// KeyValuePair represent a scalar key node, and it's related value node
type KeyValuePair struct {
	Key       string
	KeyNode   *Node
	ValueNode *Node
}

var (
	startDot    = regexp.MustCompile(`^\.`)
	endsWithDot = regexp.MustCompile(`.*\.$`)
)

// GetFileConfigs parses comments for config info
func GetFileConfigs(node *Node) FileConfigs {
	fileConfigs := FileConfigs{}
	// check config comments
	if len(node.NodeContent) != 0 {
		for _, n := range node.NodeContent[0].NodeContent {
			for _, comment := range []string{n.HeadComment, n.LineComment, n.FootComment} {
				if comment == "" {
					continue
				}
				commentLines := strings.Split(comment, "\n")
				for _, commentLine := range commentLines {
					if !strings.Contains(commentLine, "predictable-yaml:") {
						continue
					}
					commentLine = strings.ReplaceAll(commentLine, "#", "")
					commentLine = strings.ReplaceAll(commentLine, " ", "")
					commentLine = strings.Split(commentLine, ":")[1]
					splitStrings := strings.Split(commentLine, ",")
					for _, str := range splitStrings {
						switch {
						case str == "ignore":
							fileConfigs.Ignore = true
						case str == "ignore-requireds":
							fileConfigs.IgnoreRequireds = true
						case strings.Contains(str, "kind"):
							fileConfigs.Kind = strings.Split(str, "=")[1]
						}
					}
				}
			}
		}
	}

	// check Kubernetes-esq Kind
	if fileConfigs.Kind == "" {
		if len(node.NodeContent) != 0 {
			for index, n := range node.NodeContent[0].NodeContent {
				if n.Value == "kind" {
					if index+1 <= (len(node.NodeContent[0].NodeContent) - 1) {
						valueNode := node.NodeContent[0].NodeContent[index+1]
						if valueNode.Line != n.Line {
							continue
						}
						fileConfigs.Kind = valueNode.Value
					}
					break
				}
			}
		}
	}

	return fileConfigs
}

// WalkConvertYamlNodeToMainNode converts every *yaml.Node to a *main.Node with our customizations
func WalkConvertYamlNodeToMainNode(node *Node) {
	for index, innerNode := range node.Content {
		n := &Node{
			Node:       innerNode,
			ParentNode: node,
			Index:      index,
		}
		node.NodeContent = append(node.NodeContent, n)
	}
	for _, innerNode := range node.NodeContent {
		WalkConvertYamlNodeToMainNode(innerNode)
	}
}

// WalkParseLoadConfigComments loads the configs from the comments in a config file
func WalkParseLoadConfigComments(node *Node) {
	if node.LineComment != "" {
		comment := strings.ReplaceAll(node.LineComment, "#", "")
		comment = strings.ReplaceAll(comment, " ", "")
		splitStrings := strings.Split(comment, ",")
		n := firstScalarOfLine(node)
		for _, str := range splitStrings {
			switch {
			case str == "first":
				n.MustBeFirst = true
			case str == "required":
				n.Required = true
			case str == "preferred":
				n.Preferred = true
			case strings.Contains(str, "ditto"):
				n.Ditto = strings.Split(str, "=")[1]
			}
		}
	}
	for _, innerNode := range node.NodeContent {
		WalkParseLoadConfigComments(innerNode)
	}
}

// WalkAndValidateConfig validates that the config file is correctly structured
func WalkAndValidateConfig(node *Node) error {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.NodeContent) > 0 {
			return WalkAndValidateConfig(node.NodeContent[0])
		}
	case yaml.MappingNode:
		// Check for multiple 'first' directives in this map
		pairs := GetKeyValuePairs(node.NodeContent)
		firstKeys := []string{}
		for _, pair := range pairs {
			if pair.KeyNode.MustBeFirst {
				firstKeys = append(firstKeys, pair.Key)
			}
		}
		if len(firstKeys) > 1 {
			filePath := GetReferencePath(node, 0, "")
			keysStr := "'" + strings.Join(firstKeys, "', '") + "'"
			return fmt.Errorf("configuration error: multiple keys marked as 'first' in the same map at path '%s', keys: %s", filePath, keysStr)
		}

		// Check that a key marked 'first' is actually the first key in the config
		if len(firstKeys) == 1 && len(pairs) > 0 && pairs[0].Key != firstKeys[0] {
			filePath := GetReferencePath(node, 0, "")
			return fmt.Errorf("configuration error: key '%s' is marked as 'first' but is not the first key in the map at path '%s'", firstKeys[0], filePath)
		}

		// Recursively validate child nodes
		for _, pair := range pairs {
			if err := WalkAndValidateConfig(pair.ValueNode); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		// Validate the first element (which is used as template for all elements)
		if len(node.NodeContent) > 0 {
			if err := WalkAndValidateConfig(node.NodeContent[0]); err != nil {
				return err
			}
		}
	}

	return nil
}

// WalkFindNullValues walks the config and file trees together, returning errors for any
// null values in the file where the config expects a non-scalar type (map or sequence).
// This should be called before WalkAndSort to catch nulls with correct paths.
func WalkFindNullValues(configNode, fileNode *Node, sortConfs SortConfigs, errs ValidationErrors) ValidationErrors {
	switch configNode.Kind {
	case yaml.DocumentNode:
		if fileNode.Kind != yaml.DocumentNode {
			return errs
		}

		return WalkFindNullValues(configNode.NodeContent[0], fileNode.NodeContent[0], sortConfs, errs)
	case yaml.MappingNode:
		if fileNode.Kind != yaml.MappingNode {
			return errs
		}
		configPairs := GetKeyValuePairs(configNode.NodeContent)
		filePairs := GetKeyValuePairs(fileNode.NodeContent)
		for _, configPair := range configPairs {
			for _, filePair := range filePairs {
				if filePair.Key != configPair.Key {
					continue
				}
				if filePair.ValueNode.Tag == "!!null" && configPair.ValueNode.Kind != yaml.ScalarNode {
					errs = append(errs, fmt.Errorf("validation error: null value at '%s' — remove it or set a value", GetReferencePath(filePair.KeyNode, 0, "")))
					break
				}
				if configPair.KeyNode.Ditto != "" {
					cN, err := configNodeForDitto(configPair, filePair, sortConfs)
					if err != nil {
						break
					}
					if filePair.ValueNode.Tag == "!!null" && cN.Kind != yaml.ScalarNode {
						errs = append(errs, fmt.Errorf("validation error: null value at '%s' — remove it or set a value", GetReferencePath(filePair.KeyNode, 0, "")))
						break
					}
					errs = WalkFindNullValues(cN, filePair.ValueNode, sortConfs, errs)
				} else {
					errs = WalkFindNullValues(configPair.ValueNode, filePair.ValueNode, sortConfs, errs)
				}

				break
			}
		}
	case yaml.SequenceNode:
		if fileNode.Kind != yaml.SequenceNode {
			return errs
		}
		if len(configNode.NodeContent) > 0 {
			for _, fNode := range fileNode.NodeContent {
				errs = WalkFindNullValues(configNode.NodeContent[0], fNode, sortConfs, errs)
			}
		}
	}

	return errs
}

// WalkAndSort walks the tree and sorts the .Content and .NodeContent.
// Returns validation errors and whether any changes were made.
func WalkAndSort(configNode, fileNode *Node, sortConfs SortConfigs, errs ValidationErrors) (ValidationErrors, bool) {
	changed := false
	switch configNode.Kind {
	case yaml.DocumentNode:
		if fileNode.Kind != yaml.DocumentNode {
			return append(errs, fmt.Errorf("program error: expected Document: '%s'", GetReferencePath(fileNode, 0, ""))), false
		}

		return WalkAndSort(configNode.NodeContent[0], fileNode.NodeContent[0], sortConfs, errs)
	case yaml.MappingNode:
		if fileNode.Kind != yaml.MappingNode {
			return append(errs, fmt.Errorf("program error: expected Map: '%s'", GetReferencePath(fileNode, 0, ""))), false
		}

		// do the sorting
		if sortNodes(configNode, fileNode, sortConfs) {
			changed = true
		}

		// walk and sort the contents
		configPairs := GetKeyValuePairs(configNode.NodeContent)
		filePairs := GetKeyValuePairs(fileNode.NodeContent)
		for _, configPair := range configPairs {
			for _, filePair := range filePairs {
				if filePair.Key != configPair.Key {
					continue
				}
				var childChanged bool
				if configPair.KeyNode.Ditto == "" {
					errs, childChanged = WalkAndSort(configPair.ValueNode, filePair.ValueNode, sortConfs, errs)
				} else {
					cN, err := configNodeForDitto(configPair, filePair, sortConfs)
					if err != nil {
						errs = append(errs, err)
						break
					}
					errs, childChanged = WalkAndSort(cN, filePair.ValueNode, sortConfs, errs)
				}
				if childChanged {
					changed = true
				}
				break
			}
		}
	case yaml.SequenceNode:
		if fileNode.Kind != yaml.SequenceNode {
			return append(errs, fmt.Errorf("program error: expected Sequence: '%s'", GetReferencePath(fileNode, 0, ""))), false
		}
		// use the same configNode for each entry in the sequence
		if len(configNode.NodeContent) > 0 &&
			configNode.NodeContent[0].Kind == yaml.MappingNode &&
			len(fileNode.NodeContent) == 0 {
			hasRequiredValue := false
			for _, n := range configNode.NodeContent[0].NodeContent {
				if n.Required {
					hasRequiredValue = true
				}
			}
			if hasRequiredValue {
				newYamlNode := &yaml.Node{
					Kind:  yaml.MappingNode,
					Style: fileNode.Style,
				}
				newNode := &Node{
					Node:       newYamlNode,
					ParentNode: fileNode,
				}
				fileNode.NodeContent = append(fileNode.NodeContent, newNode)
				fileNode.Content = append(fileNode.Content, newYamlNode)
				changed = true
				errs, _ = WalkAndSort(configNode.NodeContent[0], fileNode.NodeContent[0], sortConfs, errs)
			}
		} else if len(configNode.NodeContent) > 0 &&
			configNode.NodeContent[0].Kind == yaml.ScalarNode &&
			len(fileNode.NodeContent) == 0 {
			newYamlNode := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: configNode.Content[0].Value,
				Style: fileNode.Style,
			}
			newNode := &Node{
				Node:       newYamlNode,
				ParentNode: fileNode,
			}
			fileNode.NodeContent = append(fileNode.NodeContent, newNode)
			fileNode.Content = append(fileNode.Content, newYamlNode)
			changed = true
		} else {
			if len(configNode.NodeContent) == 0 {
				return errs, false
			}
			for _, fNode := range fileNode.NodeContent {
				var childChanged bool
				errs, childChanged = WalkAndSort(configNode.NodeContent[0], fNode, sortConfs, errs)
				if childChanged {
					changed = true
				}
			}
		}
	case yaml.ScalarNode:
		if fileNode.Kind != yaml.ScalarNode {
			return append(errs, fmt.Errorf("program error: expected Scalar: '%s'", GetReferencePath(fileNode, 0, ""))), false
		}
	}

	return errs, changed
}

func sortNodes(configNode, fileNode *Node, sortConfs SortConfigs) bool {
	// for each line in the config, put matching file line in new slice
	newNodeContent := []*Node{}
	configPairs := GetKeyValuePairs(configNode.NodeContent)
	filePairs := GetKeyValuePairs(fileNode.NodeContent)

	for _, configPair := range configPairs {
		// find matching keyValuePair and append it
		found := false
		for _, filePair := range filePairs {
			if filePair.Key == configPair.Key {
				found = true
				filePair.KeyNode.Node.Line = configPair.KeyNode.Line
				filePair.ValueNode.Node.Line = configPair.ValueNode.Line
				newNodeContent = append(newNodeContent, filePair.KeyNode, filePair.ValueNode)
				break
			}
		}

		// possibly add missing required and preferred keys/values
		if (!found && configPair.KeyNode.Required && !sortConfs.FileConfigs.IgnoreRequireds) ||
			(!found && configPair.KeyNode.Preferred && sortConfs.AddPreferreds && !sortConfs.FileConfigs.IgnoreRequireds) {
			newValueYamlNode := &yaml.Node{
				Kind:  configPair.ValueNode.Node.Kind,
				Value: configPair.ValueNode.Node.Value,
			}
			newValueNode := &Node{
				Node:       newValueYamlNode,
				ParentNode: fileNode.ParentNode,
			}
			newKeyYamlNode := &yaml.Node{
				Kind:  configPair.KeyNode.Node.Kind,
				Value: configPair.KeyNode.Node.Value,
			}
			newKeyNode := &Node{
				Node:       newKeyYamlNode,
				ParentNode: fileNode.ParentNode,
			}

			newNodeContent = append(newNodeContent, newKeyNode, newValueNode)

			// track added fields
			if sortConfs.AddedFields != nil {
				*sortConfs.AddedFields = append(*sortConfs.AddedFields, AddedField{
					Path: GetReferencePath(fileNode, 0, ""),
					Key:  configPair.Key,
				})
			}

			// set the style to match the parent. this prevents
			//   inline representations in many cases.
			fileNode.Node.Style = fileNode.ParentNode.Style
		}
	}

	// put the remaining nodes at the end or beginning
	for _, filePair := range filePairs {
		found := false
		for _, configPair := range configPairs {
			if filePair.Key == configPair.Key {
				found = true
				break
			}
		}

		if !found {
			if sortConfs.UnmatchedToBeginning {
				newNodeContent = append([]*Node{filePair.KeyNode, filePair.ValueNode}, newNodeContent...)
			} else {
				newNodeContent = append(newNodeContent, filePair.KeyNode, filePair.ValueNode)
			}
		}
	}

	// detect if the ordering changed
	changed := len(newNodeContent) != len(fileNode.NodeContent)
	if !changed {
		for i, node := range newNodeContent {
			if node != fileNode.NodeContent[i] {
				changed = true
				break
			}
		}
	}

	fileNode.NodeContent = newNodeContent

	newContent := []*yaml.Node{}
	for _, node := range newNodeContent {
		newContent = append(newContent, node.Node)
	}
	fileNode.Content = newContent

	return changed
}

func getConfigValueNodeForDitto(configPair KeyValuePair, sortConfs SortConfigs) (*Node, error) {
	rootNode := &Node{}
	dittoPath := configPair.KeyNode.Ditto
	if startDot.MatchString(configPair.KeyNode.Ditto) {
		// is local path
		rootNode = walkToRootNode(configPair.KeyNode)
	} else {
		// is path in another config
		dittoKind := strings.Split(configPair.KeyNode.Ditto, `.`)[0]
		dittoPath = fmt.Sprintf(".%s", strings.Join(strings.Split(configPair.KeyNode.Ditto, `.`)[1:], `.`))
		ok := false
		rootNode, ok = sortConfs.ConfigNodes[dittoKind]
		if !ok {
			filePath := GetReferencePath(configPair.KeyNode, 0, "")
			err := fmt.Errorf("configuration error: no config found for schema '%s' specified at path: %s", dittoKind, filePath)
			return nil, err
		}
	}

	cN, err := walkToNodeForPath(rootNode, dittoPath, 0)
	if err != nil {
		return nil, err
	}

	// handle paths ending in dot
	if cN.Kind != yaml.ScalarNode && endsWithDot.Match([]byte(dittoPath)) {
		return cN, nil
	}

	configPairs := GetKeyValuePairs(cN.ParentNode.NodeContent)
	var valueNode *Node
	for _, configPair := range configPairs {
		if configPair.KeyNode == cN {
			valueNode = configPair.ValueNode
			break
		}
	}
	if valueNode == nil {
		filePath := GetReferencePath(configPair.KeyNode, 0, "")
		err := fmt.Errorf("configuration error: no config found for schema specified at path: %s", filePath)
		return nil, err
	}

	return valueNode, nil
}

// GetKeyValuePairs builds a list of KeyValuePairs
func GetKeyValuePairs(nodeContent []*Node) []KeyValuePair {
	if !(len(nodeContent)%2 == 0) {
		panic(fmt.Sprintf("internal error: expected an even number of nodes in mapping, got %d", len(nodeContent)))
	}

	keyValuePairs := []KeyValuePair{}
	for i := 0; i < len(nodeContent); i += 2 {
		keyValuePair := KeyValuePair{
			Key:       nodeContent[i].Value,
			KeyNode:   nodeContent[i],
			ValueNode: nodeContent[i+1],
		}
		keyValuePairs = append(keyValuePairs, keyValuePair)
	}

	return keyValuePairs
}

func walkToRootNode(node *Node) *Node {
	if node.ParentNode != nil {
		return walkToRootNode(node.ParentNode)
	}

	return node
}

func walkToNodeForPath(node *Node, path string, currentPathIndex int) (*Node, error) {
	p := strings.Replace(path, `[`, `.`, -1)
	p = strings.Replace(p, `]`, ``, -1)
	splitPath := strings.Split(p, ".")

	if currentPathIndex+1 > len(splitPath) {
		return nil, fmt.Errorf("index out of bounds: %d", currentPathIndex)
	}

	currentPathValue := splitPath[currentPathIndex]
	isPathEnd := currentPathIndex+1 == len(splitPath)
	switch node.Kind {
	case yaml.DocumentNode:
		if currentPathValue != "" {
			return nil, fmt.Errorf("no document node found in path (does it start with '.'?)")
		}
		return walkToNodeForPath(node.NodeContent[0], path, currentPathIndex+1)
	case yaml.MappingNode:
		// handle paths ending in '.'
		if isPathEnd && currentPathValue == "" {
			return node, nil
		}

		for index, innerNode := range node.NodeContent {
			if innerNode.Value == currentPathValue {
				if isPathEnd {
					return innerNode, nil
				}
				n, err := walkToNodeForPath(node.NodeContent[index+1], path, currentPathIndex+1)
				if err != nil {
					return nil, err
				}
				if n != nil {
					return n, nil
				}
			}
		}
	case yaml.SequenceNode:
		index, err := strconv.Atoi(currentPathValue)
		if err != nil {
			// it's a sequence node, but we want something else
			return nil, nil
		}
		if node.NodeContent[index] == nil {
			return nil, nil
		}
		if isPathEnd {
			return node.NodeContent[index], nil
		}
		return walkToNodeForPath(node.NodeContent[index], path, currentPathIndex+1)
	case yaml.ScalarNode:
		if node.Value == currentPathValue && isPathEnd {
			return node, nil
		}
	}

	return nil, fmt.Errorf("configuration error: '%s' configuration node not found", path)
}

// configNodeForDitto resolves the config node for a ditto reference, wrapping
// it in a synthetic sequence node when the ditto path points to a mapping but
// the file node is a sequence (e.g. ditto=.spec.containers. used on a list field).
func configNodeForDitto(configPair KeyValuePair, filePair KeyValuePair, sortConfs SortConfigs) (*Node, error) {
	cN, err := getConfigValueNodeForDitto(configPair, sortConfs)
	if err != nil {
		return nil, err
	}
	if endsWithDot.Match([]byte(configPair.KeyNode.Ditto)) && filePair.ValueNode.Kind == yaml.SequenceNode && cN.Kind == yaml.MappingNode {
		return &Node{
			Node: &yaml.Node{
				Kind: yaml.SequenceNode,
			},
			NodeContent: []*Node{cN},
		}, nil
	}
	return cN, nil
}

func firstScalarOfLine(node *Node) *Node {
	if node.ParentNode == nil {
		panic(fmt.Sprintf("internal error: expected node to have parentNode, value: %q, line: %d", node.Value, node.Line))
	}
	for index, innerNode := range node.ParentNode.NodeContent {
		if innerNode == node {
			if index-1 >= 0 {
				n := node.ParentNode.NodeContent[index-1]
				if n.Kind == yaml.ScalarNode && n.Line == node.Line {
					return n
				}
			}
		}
	}

	return node
}

// GetReferencePath takes a node and returns a path string like '.spec.template.spec.containers'
func GetReferencePath(node *Node, scalarIndex int, path string) string {
	switch node.Kind {
	case yaml.DocumentNode:
		return path
	case yaml.MappingNode:
		if node.Index-1 >= 0 && node.ParentNode.NodeContent[node.Index-1].Kind == yaml.ScalarNode {
			return GetReferencePath(node.ParentNode.NodeContent[node.Index-1], node.Index, path)
		}
		return GetReferencePath(node.ParentNode, node.Index, path)
	case yaml.SequenceNode:
		if node.Index-1 >= 0 && node.ParentNode.NodeContent[node.Index-1].Kind == yaml.ScalarNode {
			return fmt.Sprintf("%s[%d]", GetReferencePath(node.ParentNode.NodeContent[node.Index-1], node.Index, path), scalarIndex)
		}
		return fmt.Sprintf("%s[%d]", GetReferencePath(node.ParentNode, node.Index, path), scalarIndex)
	case yaml.ScalarNode:
		if node.ParentNode.Kind == yaml.SequenceNode {
			return GetReferencePath(node.ParentNode, node.Index, path)
		}
		return fmt.Sprintf("%s.%s", GetReferencePath(node.ParentNode, node.Index, path), node.Value)
	}

	return path
}

// GetValidationErrorStrings combines the error's strings
func GetValidationErrorStrings(errs ValidationErrors) string {
	errorString := ""
	for index, err := range errs {
		errorString += "\t"
		errorString += err.Error()
		if index+1 != len(errs) {
			errorString += "\n"
		}
	}

	return errorString
}
