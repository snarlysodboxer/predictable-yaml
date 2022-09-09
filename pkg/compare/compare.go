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
	NodeContent          []*Node
	ParentNode           *Node
	PreviousLineNode     *Node // only leading Scalar lines in the same parent node
	FollowingContentNode *Node // only Mapping and Sequence nodes following Scalar nodes
	Index                int
	MustBeFirst          bool
	Required             bool
	Preferred            bool
	Ditto                string
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
	IgnorePreferreds     bool
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
			if !strings.Contains(comment, "predictable-yaml:") {
				continue
			}
			comment = strings.ReplaceAll(comment, "#", "")
			comment = strings.ReplaceAll(comment, " ", "")
			comment = strings.Split(comment, ":")[1]
			splitStrings := strings.Split(comment, ",")
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

		// set previousLineNode if exists and is scalar
		if index-2 >= 0 && node.NodeContent[index-2].Kind == yaml.ScalarNode {
			n.PreviousLineNode = node.NodeContent[index-2]
		}

		// check if this is a contents node
		if index > 0 &&
			node.NodeContent[index-1].Kind == yaml.ScalarNode &&
			(n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode) {
			node.NodeContent[index-1].FollowingContentNode = n
		}
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
			return append(errs, fmt.Errorf("program error: expected file node to be Document: %#v", fileNode))
		}
		return WalkAndCompare(configNode.NodeContent[0], fileNode.NodeContent[0], sortConfs, errs)
	case yaml.MappingNode:
		if fileNode.Kind != yaml.MappingNode {
			return append(errs, fmt.Errorf("program error: expected file node to be Map: %#v", fileNode))
		}
		for _, innerConfigNode := range configNode.NodeContent {
			// only do scalar nodes because we'll walk to their FollowingContent nodes in scalar section
			if innerConfigNode.Kind != yaml.ScalarNode {
				continue
			}
			// only do scalar keys, not values
			n := firstScalarOfLine(innerConfigNode)
			if innerConfigNode != n {
				// is scalar value, not key
				continue
			}
			found := false
			for _, innerFileNode := range fileNode.NodeContent {
				if innerFileNode.Kind == yaml.ScalarNode && innerConfigNode.Value == innerFileNode.Value {
					errs = WalkAndCompare(innerConfigNode, innerFileNode, sortConfs, errs)
					found = true
					break
				}
			}
			// check 'required'
			if !found && innerConfigNode.Required && !sortConfs.FileConfigs.IgnoreRequireds {
				path := fmt.Sprintf("%s.%s", getReferencePath(fileNode, 0, ""), innerConfigNode.Value)
				errs = append(errs, fmt.Errorf("validation error: missing required key '%s'", path))
			}
		}
	case yaml.SequenceNode:
		if fileNode.Kind != yaml.SequenceNode {
			return append(errs, fmt.Errorf("program error: expected file node to be Sequence: %#v", fileNode))
		}
		for _, fNode := range fileNode.NodeContent {
			// use the same configNode for each entry in the sequence
			if len(configNode.NodeContent) > 0 {
				errs = WalkAndCompare(configNode.NodeContent[0], fNode, sortConfs, errs)
			}
		}
	case yaml.ScalarNode:
		if fileNode.Kind != yaml.ScalarNode {
			return append(errs, fmt.Errorf("program error: expected file node to be Scalar: %#v", fileNode))
		}

		// check 'first'
		if configNode.MustBeFirst && fileNode.Index != 0 {
			configPath := getReferencePath(fileNode, 0, "")
			filePath := getReferencePath(fileNode.ParentNode.NodeContent[0], 0, "")
			errs = append(errs, fmt.Errorf("validation error: want '%s' to be first, got '%s'", configPath, filePath))
		}

		// check 'after'
		errs = append(errs, walkCheckAfter(configNode, fileNode)...)

		// check 'ditto'
		if configNode.Ditto != "" {
			cN, err := getConfigNodeForDitto(configNode, sortConfs)
			if err != nil {
				errs = append(errs, err)
				break
			}
			errs = WalkAndCompare(cN, fileNode, sortConfs, errs)
		}

		// walk this scalar's following content node
		if configNode.FollowingContentNode == nil {
			return errs
		}
		if fileNode.FollowingContentNode == nil {
			filePath := getReferencePath(fileNode.FollowingContentNode, 0, "")
			return append(errs, fmt.Errorf("validation error: want '%s' to be a %s node, got nil", filePath, kindToString(configNode.FollowingContentNode.Kind)))
		}
		if fileNode.FollowingContentNode.Kind != configNode.FollowingContentNode.Kind {
			configPath := getReferencePath(configNode, 0, "")
			return append(errs, fmt.Errorf("validation error: want '%s' to be a %s node, got '%s'", configPath, kindToString(configNode.FollowingContentNode.Kind), kindToString(fileNode.FollowingContentNode.Kind)))
		}

		return WalkAndCompare(configNode.FollowingContentNode, fileNode.FollowingContentNode, sortConfs, errs)
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
			return append(errs, fmt.Errorf("program error: expected file node to be Document: %#v", fileNode))
		}
		return WalkAndSort(configNode.NodeContent[0], fileNode.NodeContent[0], sortConfs, errs)
	case yaml.MappingNode:
		if fileNode.Kind != yaml.MappingNode {
			return append(errs, fmt.Errorf("program error: expected file node to be Map: %#v", fileNode))
		}

		sortNodes(configNode, fileNode, sortConfs)

		configPairs := getKeyValuePairs(configNode.NodeContent)
		filePairs := getKeyValuePairs(fileNode.NodeContent)
		for _, cKeyValuePair := range configPairs {
			for _, fKeyValuePair := range filePairs {
				if fKeyValuePair.Key != cKeyValuePair.Key {
					continue
				}
				// handle dittos
				if cKeyValuePair.KeyNode.Ditto != "" {
					cN, err := getConfigNodeForDitto(cKeyValuePair.KeyNode, sortConfs)
					if err != nil {
						errs = append(errs, err)
						break
					}
					errs = WalkAndSort(cN.FollowingContentNode, fKeyValuePair.ValueNode, sortConfs, errs)
				} else {
					errs = WalkAndSort(cKeyValuePair.ValueNode, fKeyValuePair.ValueNode, sortConfs, errs)
					break
				}
			}
		}
	case yaml.SequenceNode:
		if fileNode.Kind != yaml.SequenceNode {
			return append(errs, fmt.Errorf("program error: expected file node to be Sequence: %#v", fileNode))
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
			return append(errs, fmt.Errorf("program error: expected file node to be Scalar, got: %#v\n\t%#v", fileNode, fileNode.Node))
		}
	}

	return errs
}

func sortNodes(configNode, fileNode *Node, sortConfs SortConfigs) {
	// for each line in the config, put matching file line in new slice
	newNodeContent := []*Node{}
	configPairs := getKeyValuePairs(configNode.NodeContent)
	filePairs := getKeyValuePairs(fileNode.NodeContent)

	for _, cKeyValuePair := range configPairs {
		// find matching keyValuePair and append it
		found := false
		for _, fKeyValuePair := range filePairs {
			if fKeyValuePair.Key == cKeyValuePair.Key {
				found = true
				fKeyValuePair.KeyNode.Node.Line = cKeyValuePair.KeyNode.Line
				fKeyValuePair.ValueNode.Node.Line = cKeyValuePair.ValueNode.Line
				newNodeContent = append(newNodeContent, fKeyValuePair.KeyNode, fKeyValuePair.ValueNode)
				break
			}
		}

		// possibly add missing required and preferred keys/values
		if (!found && cKeyValuePair.KeyNode.Required && !sortConfs.FileConfigs.IgnoreRequireds) ||
			(!found && cKeyValuePair.KeyNode.Preferred && !sortConfs.IgnorePreferreds) {
			newValueYamlNode := &yaml.Node{
				Kind:  cKeyValuePair.ValueNode.Node.Kind,
				Value: cKeyValuePair.ValueNode.Node.Value,
			}
			newValueNode := &Node{
				Node:       newValueYamlNode,
				ParentNode: fileNode.ParentNode,
			}
			newKeyYamlNode := &yaml.Node{
				Kind:  cKeyValuePair.KeyNode.Node.Kind,
				Value: cKeyValuePair.KeyNode.Node.Value,
			}
			newKeyNode := &Node{
				Node:                 newKeyYamlNode,
				ParentNode:           fileNode.ParentNode,
				FollowingContentNode: newValueNode,
			}

			newNodeContent = append(newNodeContent, newKeyNode, newValueNode)

			// set the style to match the parent. this prevents
			//   inline representations in many cases.
			fileNode.Node.Style = fileNode.ParentNode.Style
		}
	}

	// put the remaining nodes at the end or beginning
	for _, fKeyValuePair := range filePairs {
		found := false
		for _, cKeyValuePair := range configPairs {
			if fKeyValuePair.Key == cKeyValuePair.Key {
				found = true
				break
			}
		}

		if !found {
			if sortConfs.UnmatchedToBeginning {
				newNodeContent = append([]*Node{fKeyValuePair.KeyNode, fKeyValuePair.ValueNode}, newNodeContent...)
			} else {
				newNodeContent = append(newNodeContent, fKeyValuePair.KeyNode, fKeyValuePair.ValueNode)
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

func getConfigNodeForDitto(configNode *Node, sortConfs SortConfigs) (*Node, error) {
	startDot := regexp.MustCompile(`^\.`)
	if startDot.MatchString(configNode.Ditto) {
		// is local path
		rootNode := walkToRootNode(configNode)
		cN, err := walkToNodeForPath(rootNode, configNode.Ditto, 0)
		if err != nil {
			return nil, err
		}

		return cN, nil
	}

	// is path in another config altogether
	dittoKind := strings.Split(configNode.Ditto, `.`)[0]
	dittoPath := fmt.Sprintf(".%s", strings.Join(strings.Split(configNode.Ditto, `.`)[1:], `.`))
	cN, ok := sortConfs.ConfigMap[dittoKind]
	if !ok {
		filePath := getReferencePath(configNode, 0, "")
		err := fmt.Errorf("configuration error: no config found for schema '%s' specified at path: %s", dittoKind, filePath)
		return nil, err
	}
	n, err := walkToNodeForPath(cN, dittoPath, 0)
	if err != nil {
		return nil, err
	}

	return n, nil
}

func getKeyValuePairs(nodeContent []*Node) []KeyValuePair {
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

// walkCheckAfter checks that fileNode is sometime after the node before configNode
func walkCheckAfter(configNode, fileNode *Node) ValidationErrors {
	errs := ValidationErrors{}
	if configNode.PreviousLineNode == nil {
		return errs
	}

	// find matching fileNode sibling
	found := false
	for _, siblingFileNode := range fileNode.ParentNode.NodeContent {
		if siblingFileNode.Kind != yaml.ScalarNode {
			continue
		}
		if siblingFileNode.Value == configNode.PreviousLineNode.Value {
			found = true
			// do the check
			if siblingFileNode.Index > fileNode.Index {
				fileNode = firstScalarOfLine(fileNode)
				filePath := getReferencePath(fileNode, 0, "")
				sisterPath := fmt.Sprintf("%s.%s", getReferencePath(fileNode.ParentNode, 0, ""), configNode.PreviousLineNode.Value)
				errs = append(errs, fmt.Errorf("validation error: want '%s' to be after '%s', is before", filePath, sisterPath))
			}
		}
	}
	if !found {
		// check older sister's older sister
		return walkCheckAfter(configNode.PreviousLineNode, fileNode)
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

// getReferencePath takes a node and returns a path string like '.spec.template.spec.containers'
func getReferencePath(node *Node, scalarIndex int, path string) string {
	switch node.Kind {
	case yaml.DocumentNode:
		return path
	case yaml.MappingNode:
		if node.Index-1 >= 0 && node.ParentNode.NodeContent[node.Index-1].Kind == yaml.ScalarNode {
			return getReferencePath(node.ParentNode.NodeContent[node.Index-1], node.Index, path)
		}
		return getReferencePath(node.ParentNode, node.Index, path)
	case yaml.SequenceNode:
		if node.Index-1 >= 0 && node.ParentNode.NodeContent[node.Index-1].Kind == yaml.ScalarNode {
			return fmt.Sprintf("%s[%d]", getReferencePath(node.ParentNode.NodeContent[node.Index-1], node.Index, path), scalarIndex)
		}
		return fmt.Sprintf("%s[%d]", getReferencePath(node.ParentNode, node.Index, path), scalarIndex)
	case yaml.ScalarNode:
		if node.ParentNode.Kind == yaml.SequenceNode {
			return getReferencePath(node.ParentNode, node.Index, path)
		}
		return fmt.Sprintf("%s.%s", getReferencePath(node.ParentNode, node.Index, path), node.Value)
	}

	return path
}

func kindToString(kind yaml.Kind) string {
	switch kind {
	case yaml.DocumentNode:
		return "Document"
	case yaml.MappingNode:
		return "Mapping "
	case yaml.SequenceNode:
		return "Sequence"
	case yaml.ScalarNode:
		return "Scalar  "
	case yaml.AliasNode:
		return "Alias   "
	default:
		return "unknown "
	}
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
