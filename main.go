package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var configFile = flag.String("configFile", "deployment-config.yaml", "The configuration file")

// TODO support multiple config types at once, e.g. deployment, service, etc
// var inferTypeFromK8sKind = flag.Bool("infer-type-from-k8s-kind", false, "Infer the config type from Kubernetes-esq kind")

// Node is a custom yaml.Node
type Node struct {
	*yaml.Node
	NodeContent          []*Node
	ParentNode           *Node
	PreviousLineNode     *Node // only lead Scalar lines in the same parent node
	FollowingContentNode *Node // only Mapping and Sequence nodes following Scalar nodes
	Index                int
	MustBeFirst          bool
	Required             bool
	Ditto                string
}

// ValidationErrors allows us to return multiple validation errors
type ValidationErrors []error

func main() {
	flag.Parse()

	cNode := &yaml.Node{}
	err := getYAML(cNode, *configFile)
	if err != nil {
		log.Fatalf("error getting config: %v", err)
	}
	configNode := &Node{Node: cNode}
	walkConvertYamlNodeToMainNode(configNode)
	walkParseLoadConfigComments(configNode)

	filePaths, err := getFilePathsFromStdin()
	if err != nil {
		log.Fatal(err)
	}

	success := true
	for _, filePath := range filePaths {
		errs := ValidationErrors{}
		fNode := &yaml.Node{}
		err := getYAML(fNode, filePath)
		if err != nil {
			log.Fatalf("error getting file: %s: %v", filePath, err)
		}
		fileNode := &Node{Node: fNode}
		walkConvertYamlNodeToMainNode(fileNode)

		errs = append(errs, walkAndCompare(configNode, fileNode, errs)...)

		if len(errs) != 0 {
			success = false
			log.Printf("File '%s' has validation errors:\n%v", filePath, getValidationErrorStrings(errs))
		} else {
			log.Printf("File '%s' is valid!", filePath)
		}
	}
	if !success {
		log.Fatal("FAIL")
	}
	log.Println("SUCCESS")
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

// walkConvertYamlNodeToMainNode converts every *yaml.Node to a *main.Node with our customizations
func walkConvertYamlNodeToMainNode(node *Node) {
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
		walkConvertYamlNodeToMainNode(innerNode)
	}
}

func walkParseLoadConfigComments(node *Node) {
	if node.Kind == yaml.ScalarNode && node.LineComment != "" {
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
			case strings.Contains(str, "ditto"):
				n.Ditto = strings.Split(str, ":")[1]
			}
		}
	}
	for _, innerNode := range node.NodeContent {
		walkParseLoadConfigComments(innerNode)
	}
}

func walkAndCompare(configNode, fileNode *Node, errs ValidationErrors) ValidationErrors {
	switch configNode.Kind {
	case yaml.DocumentNode:
		if fileNode.Kind != yaml.DocumentNode {
			return append(errs, fmt.Errorf("program error: expected file node to be Document: %v", fileNode))
		}
		return walkAndCompare(configNode.NodeContent[0], fileNode.NodeContent[0], errs)
	case yaml.MappingNode:
		if fileNode.Kind != yaml.MappingNode {
			return append(errs, fmt.Errorf("program error: expected file node to be Map: %v", fileNode))
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
					errs = walkAndCompare(innerConfigNode, innerFileNode, errs)
					found = true
					break
				}
			}
			// check required
			if !found && innerConfigNode.Required {
				path := fmt.Sprintf("%s.%s", getReferencePath(fileNode, 0, ""), innerConfigNode.Value)
				errs = append(errs, fmt.Errorf("validation error: missing required key '%s'", path))
			}
		}
	case yaml.SequenceNode:
		if fileNode.Kind != yaml.SequenceNode {
			return append(errs, fmt.Errorf("program error: expected file node to be Sequence: %v", fileNode))
		}
		for _, fNode := range fileNode.NodeContent {
			// use the same configNode for each entry in the sequence
			errs = walkAndCompare(configNode.NodeContent[0], fNode, errs)
		}
	case yaml.ScalarNode:
		if fileNode.Kind != yaml.ScalarNode {
			return append(errs, fmt.Errorf("program error: expected file node to be Scalar: %v", fileNode))
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
			rootNode := walkToRootNode(configNode)
			cN, err := walkToNodeForPath(rootNode, strings.Split(configNode.Ditto, "."), 0)
			if err != nil {
				errs = append(errs, err)
			} else {
				errs = append(errs, walkAndCompare(cN, fileNode, errs)...)
			}
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

		return walkAndCompare(configNode.FollowingContentNode, fileNode.FollowingContentNode, errs)
	default:
		return append(errs, fmt.Errorf("did not expect configNode.Kind of: %v", fileNode.Kind))
	}

	return errs
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

func walkToNodeForPath(node *Node, path []string, currentPathIndex int) (*Node, error) {
	if currentPathIndex+1 > len(path) {
		return nil, fmt.Errorf("index out of bounds: %d", currentPathIndex)
	}

	currentPathValue := path[currentPathIndex]
	isPathEnd := currentPathIndex+1 == len(path)
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

	return nil, fmt.Errorf("configuration error: '%s' configuration node not found", strings.Join(path, "."))
}

func firstScalarOfLine(node *Node) *Node {
	if node.Kind != yaml.ScalarNode {
		panic(fmt.Sprintf("expected to be passed a ScalarNode, got: %s", kindToString(node.Kind)))
	}
	if node.ParentNode == nil {
		panic(fmt.Sprintf("expected node to have parentNode: %#v", node.Node))
	}
	for index, innerNode := range node.ParentNode.NodeContent {
		if innerNode == node {
			if index-1 >= 0 {
				n := node.ParentNode.NodeContent[index-1]
				if n.Line == node.Line {
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

func getValidationErrorStrings(errs ValidationErrors) string {
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
