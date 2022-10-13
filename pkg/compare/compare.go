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

	"gopkg.in/yaml.v3"
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

// ConfigMap is a map of names to Config Nodes
type ConfigMap map[string]*Node

// ValidationErrors allows us to return multiple validation errors
type ValidationErrors []error

// FileConfigs supports kind and overrides
type FileConfigs struct {
	Kind            string // config and target files
	Ignore          bool   // target files only
	IgnoreRequireds bool   // target files only
}

// SortConfigs represent various configs for a sorting operation
type SortConfigs struct {
	ConfigMap            ConfigMap
	FileConfigs          FileConfigs
	UnmatchedToBeginning bool
	AddPreferreds        bool
}

// KeyValuePair represent a scalar key node, and it's related value node
type KeyValuePair struct {
	Key       string
	KeyNode   *Node
	ValueNode *Node
}

// GetFileConfigs parses comments for config info
func GetFileConfigs(node *Node) FileConfigs {
	fileConfigs := FileConfigs{}
	// check config comments
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

	// check Kubernetes-esq Kind
	if fileConfigs.Kind == "" {
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

// WalkAndCompare walks the tree and does the validation
func WalkAndCompare(configNode, fileNode *Node, sortConfs SortConfigs, errs ValidationErrors) ValidationErrors {
	switch configNode.Kind {
	case yaml.DocumentNode:
		if fileNode.Kind != yaml.DocumentNode {
			return append(errs, fmt.Errorf("program error: expected Document: '%s'", GetReferencePath(fileNode, 0, "")))
		}

		return WalkAndCompare(configNode.NodeContent[0], fileNode.NodeContent[0], sortConfs, errs)
	case yaml.MappingNode:
		if fileNode.Kind != yaml.MappingNode {
			return append(errs, fmt.Errorf("program error: expected Map: '%s'", GetReferencePath(fileNode, 0, "")))
		}

		// do the checks
		configPairs := GetKeyValuePairs(configNode.NodeContent)
		filePairs := GetKeyValuePairs(fileNode.NodeContent)
		errs = checkPairs(configPairs, filePairs, sortConfs, errs)

		// walk and compare
		for _, configPair := range configPairs {
			for _, filePair := range filePairs {
				if filePair.Key != configPair.Key {
					continue
				}
				if configPair.KeyNode.Ditto == "" {
					errs = WalkAndCompare(configPair.ValueNode, filePair.ValueNode, sortConfs, errs)
				} else {
					// handle dittos
					cN, err := getConfigValueNodeForDitto(configPair, sortConfs)
					if err != nil {
						errs = append(errs, err)
						break
					}
					endsWithDot := regexp.MustCompile(`.*\.$`)
					switch {
					case endsWithDot.Match([]byte(configPair.KeyNode.Ditto)) && filePair.ValueNode.Kind == yaml.SequenceNode && cN.Kind == yaml.MappingNode:
						wrappedInSequence := &Node{
							Node: &yaml.Node{
								Kind: yaml.SequenceNode,
							},
							NodeContent: []*Node{cN},
						}
						errs = WalkAndCompare(wrappedInSequence, filePair.ValueNode, sortConfs, errs)
					default:
						errs = WalkAndCompare(cN, filePair.ValueNode, sortConfs, errs)
					}
					break
				}
				break
			}
		}
	case yaml.SequenceNode:
		if fileNode.Kind != yaml.SequenceNode {
			return append(errs, fmt.Errorf("program error: expected Sequence: '%s'", GetReferencePath(fileNode, 0, "")))
		}
		for _, fNode := range fileNode.NodeContent {
			// use the same configNode for each entry in the sequence
			if len(configNode.NodeContent) > 0 {
				errs = WalkAndCompare(configNode.NodeContent[0], fNode, sortConfs, errs)
			}
		}
	case yaml.ScalarNode:
		if fileNode.Kind != yaml.ScalarNode {
			return append(errs, fmt.Errorf("program error: expected Scalar: '%s'", GetReferencePath(fileNode, 0, "")))
		}
	default:
		return append(errs, fmt.Errorf("did not expect configNode.Kind of: %v", fileNode.Kind))
	}

	return errs
}

// WalkAndSort walks the tree and sorts the .Content and .NodeContent
func WalkAndSort(configNode, fileNode *Node, sortConfs SortConfigs, errs ValidationErrors) ValidationErrors {
	switch configNode.Kind {
	case yaml.DocumentNode:
		if fileNode.Kind != yaml.DocumentNode {
			return append(errs, fmt.Errorf("program error: expected Document: '%s'", GetReferencePath(fileNode, 0, "")))
		}

		return WalkAndSort(configNode.NodeContent[0], fileNode.NodeContent[0], sortConfs, errs)
	case yaml.MappingNode:
		if fileNode.Kind != yaml.MappingNode {
			return append(errs, fmt.Errorf("program error: expected Map: '%s'", GetReferencePath(fileNode, 0, "")))
		}

		// do the sorting
		sortNodes(configNode, fileNode, sortConfs)

		// walk and sort the contents
		configPairs := GetKeyValuePairs(configNode.NodeContent)
		filePairs := GetKeyValuePairs(fileNode.NodeContent)
		for _, configPair := range configPairs {
			for _, filePair := range filePairs {
				if filePair.Key != configPair.Key {
					continue
				}
				if configPair.KeyNode.Ditto == "" {
					errs = WalkAndSort(configPair.ValueNode, filePair.ValueNode, sortConfs, errs)
				} else {
					// handle dittos
					cN, err := getConfigValueNodeForDitto(configPair, sortConfs)
					if err != nil {
						errs = append(errs, err)
						break
					}
					// TODO this way of handling mismatched kinds feels clumsy,
					//   here and in WalkAndCompare.
					endsWithDot := regexp.MustCompile(`.*\.$`)
					switch {
					case endsWithDot.Match([]byte(configPair.KeyNode.Ditto)) && filePair.ValueNode.Kind == yaml.SequenceNode && cN.Kind == yaml.MappingNode:
						wrappedInSequence := &Node{
							Node: &yaml.Node{
								Kind: yaml.SequenceNode,
							},
							NodeContent: []*Node{cN},
						}
						errs = WalkAndSort(wrappedInSequence, filePair.ValueNode, sortConfs, errs)
					default:
						errs = WalkAndSort(cN, filePair.ValueNode, sortConfs, errs)
					}
				}
				break
			}
		}
	case yaml.SequenceNode:
		if fileNode.Kind != yaml.SequenceNode {
			return append(errs, fmt.Errorf("program error: expected Sequence: '%s'", GetReferencePath(fileNode, 0, "")))
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
				errs = WalkAndSort(configNode.NodeContent[0], fileNode.NodeContent[0], sortConfs, errs)
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
		} else {
			if len(configNode.NodeContent) == 0 {
				return errs
			}
			for _, fNode := range fileNode.NodeContent {
				errs = WalkAndSort(configNode.NodeContent[0], fNode, sortConfs, errs)
			}
		}
	case yaml.ScalarNode:
		if fileNode.Kind != yaml.ScalarNode {
			return append(errs, fmt.Errorf("program error: expected Scalar: '%s'", GetReferencePath(fileNode, 0, "")))
		}
	}

	return errs
}

func sortNodes(configNode, fileNode *Node, sortConfs SortConfigs) {
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

	fileNode.NodeContent = newNodeContent

	newContent := []*yaml.Node{}
	for _, node := range newNodeContent {
		newContent = append(newContent, node.Node)
	}
	fileNode.Content = newContent
}

func getConfigValueNodeForDitto(configPair KeyValuePair, sortConfs SortConfigs) (*Node, error) {
	startDot := regexp.MustCompile(`^\.`)
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
		rootNode, ok = sortConfs.ConfigMap[dittoKind]
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
	endsWithDot := regexp.MustCompile(`.*\.$`)
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
		panic("expected an even number of nodes")
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

// checkPairs checks that fileNode is sometime after the matching node before configNode.
func checkPairs(configPairs, filePairs []KeyValuePair, sortConfs SortConfigs, errs ValidationErrors) ValidationErrors {
	// check 'requireds'
	for _, configPair := range configPairs {
		found := false
		for _, filePair := range filePairs {
			if filePair.Key != configPair.Key {
				continue
			}
			found = true
		}
		if !found && configPair.KeyNode.Required && !sortConfs.FileConfigs.IgnoreRequireds {
			filePath := ""
			if len(filePairs) == 0 {
				filePath = GetReferencePath(configPairs[0].KeyNode.ParentNode, 0, "")
			} else {
				filePath = GetReferencePath(filePairs[0].KeyNode.ParentNode, 0, "")
			}
			path := fmt.Sprintf("%s.%s", filePath, configPair.KeyNode.Value)
			errs = append(errs, fmt.Errorf("validation error: missing required key '%s'", path))
		}
	}

	// for each filePair
	for _, filePair := range filePairs {
		// if there's no matching configPair, do nothing
		found := false
		cIndex := 0
		for i, configPair := range configPairs {
			if configPair.Key == filePair.Key {
				cIndex = i
				found = true
			}
		}
		if !found {
			continue
		}

		// check 'first'
		if configPairs[cIndex].KeyNode.MustBeFirst && filePair.KeyNode.Index != 0 {
			configPath := GetReferencePath(filePair.KeyNode, 0, "")
			filePath := GetReferencePath(filePairs[0].KeyNode, 0, "")
			errs = append(errs, fmt.Errorf("validation error: want '%s' to be first, got '%s'", configPath, filePath))
		}

		// if there is a matching configPair, check the previous configPair,
		//   and if that doesn't exist, the one before that, etc.
	ConfigPairsLoop:
		for {
			// stop if we've reached the end
			if configPairs[cIndex].KeyNode.Index == 0 {
				break
			}

			// now find the filePair matching the previous configPair to check 'afters'
			targetFound := false
			for _, targetFilePair := range filePairs {
				if targetFilePair.Key != configPairs[cIndex-1].Key {
					continue
				}
				targetFound = true
				// check 'afters'
				if filePair.KeyNode.Index < targetFilePair.KeyNode.Index {
					filePath := GetReferencePath(filePair.KeyNode, 0, "")
					sisterPath := GetReferencePath(targetFilePair.KeyNode, 0, "")
					errs = append(errs, fmt.Errorf("validation error: want '%s' to be after '%s', is before", filePath, sisterPath))
				}
				break ConfigPairsLoop
			}
			if !targetFound {
				// check previous keyValuePair
				cIndex--
			}
		}
	}

	return errs
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

func firstScalarOfLine(node *Node) *Node {
	if node.ParentNode == nil {
		panic(fmt.Sprintf("expected node to have parentNode: %#v", node.Node))
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
