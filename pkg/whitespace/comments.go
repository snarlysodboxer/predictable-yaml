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
package whitespace

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"gopkg.in/yaml.v3"
)

// PreserveComments attempts to preserve the spacing before all comments
// walk matching nodes, and for each that has (Head|Line|Foot)Comment != "",
//   overwrite the newContent spacing and comment with the oldContent spacing and comment.
func PreserveComments(oldContent, newContent []byte) ([]byte, error) {
	// parse old yaml
	oldYamlNode := &yaml.Node{}
	err := yaml.Unmarshal(oldContent, oldYamlNode)
	if err != nil {
		return newContent, err
	}
	oldNode := &compare.Node{Node: oldYamlNode}
	compare.WalkConvertYamlNodeToMainNode(oldNode)

	// parse new yaml
	newYamlNode := &yaml.Node{}
	err = yaml.Unmarshal(newContent, newYamlNode)
	if err != nil {
		return newContent, err
	}
	newNode := &compare.Node{Node: newYamlNode}
	compare.WalkConvertYamlNodeToMainNode(newNode)

	oldLinesMap := getLinesMap(oldContent)
	newLinesMap := getLinesMap(newContent)

	// do it
	err = walkAndFix(oldLinesMap, newLinesMap, oldNode, newNode)
	if err != nil {
		return newContent, err
	}

	// reassemble
	content := ""
	for i := 1; i <= len(newLinesMap); i++ {
		if i == len(newLinesMap) {
			content += newLinesMap[i]
		} else {
			content += newLinesMap[i] + "\n"
		}
	}

	return []byte(content), nil
}

// getLinesMap is 1 based, like lines in a file
func getLinesMap(content []byte) map[int]string {
	linesMap := map[int]string{}
	lines := strings.Split(string(content), "\n")
	for index, line := range lines {
		linesMap[index+1] = line
	}

	return linesMap
}

func walkAndFix(oldLinesMap, newLinesMap map[int]string, oldNode, newNode *compare.Node) error {
	// defensive
	_, ok := oldLinesMap[oldNode.Line]
	if !ok {
		return fmt.Errorf("program error: no line '%d' found in oldLinesMap", oldNode.Line)
	}
	_, ok = newLinesMap[newNode.Line]
	if !ok {
		return fmt.Errorf("program error: no line '%d' found in newLinesMap", oldNode.Line)
	}

	// comments may be at new locations and divided differently between
	//   HeadComment and FootComment. fixHeadComment and fixFootComment attempt to find where each one went.
	if oldNode.HeadComment != "" {
		err := fixHeadComment(oldLinesMap, newLinesMap, oldNode, newNode)
		if err != nil {
			log.Println(err)
		}
	}

	if oldNode.LineComment != "" {
		err := fixLineComment(oldLinesMap, newLinesMap, oldNode, newNode)
		if err != nil {
			return err
		}
	}

	if oldNode.FootComment != "" {
		err := fixFootComment(oldLinesMap, newLinesMap, oldNode, newNode)
		if err != nil {
			log.Println(err)
		}
	}

	switch oldNode.Kind {
	case yaml.DocumentNode:
		if newNode.Kind != yaml.DocumentNode {
			return fmt.Errorf("program error: expected Document: '%s'", compare.GetReferencePath(newNode, 0, ""))
		}

		return walkAndFix(oldLinesMap, newLinesMap, oldNode.NodeContent[0], newNode.NodeContent[0])
	case yaml.MappingNode:
		if newNode.Kind != yaml.MappingNode {
			return fmt.Errorf("program error: expected Mapping: '%s'", compare.GetReferencePath(newNode, 0, ""))
		}

		if oldNode.Style == yaml.FlowStyle {
			return nil
		}

		oldPairs := compare.GetKeyValuePairs(oldNode.NodeContent)
		newPairs := compare.GetKeyValuePairs(newNode.NodeContent)
		for _, oldPair := range oldPairs {
			for _, newPair := range newPairs {
				if oldPair.Key != newPair.Key {
					continue
				}
				// keys
				var err error
				err = walkAndFix(oldLinesMap, newLinesMap, oldPair.KeyNode, newPair.KeyNode)
				if err != nil {
					return err
				}

				// values
				err = walkAndFix(oldLinesMap, newLinesMap, oldPair.ValueNode, newPair.ValueNode)
				if err != nil {
					return err
				}
				break
			}
		}

	case yaml.SequenceNode:
		if newNode.Kind != yaml.SequenceNode {
			return fmt.Errorf("program error: expected Sequence: '%s'", compare.GetReferencePath(newNode, 0, ""))
		}

		if oldNode.Style == yaml.FlowStyle {
			return nil
		}

		for index, oNode := range oldNode.NodeContent {
			var err error
			err = walkAndFix(oldLinesMap, newLinesMap, oNode, newNode.NodeContent[index])
			if err != nil {
				return err
			}
		}

	case yaml.ScalarNode:
		if newNode.Kind != yaml.ScalarNode {
			return fmt.Errorf("program error: expected Scalar: '%s'", compare.GetReferencePath(newNode, 0, ""))
		}

		return nil
	}

	return nil
}

func fixHeadComment(oldLinesMap, newLinesMap map[int]string, oldNode, newNode *compare.Node) error {
	// find oldLineNumber
	oldTrimComment := strings.TrimSpace(oldNode.HeadComment)
	oldCount := len(strings.Split(oldTrimComment, "\n"))
	oldLineNumber := oldNode.Line
	// check for up to 20 blank lines between this and HeadComment
	for i := 0; i < 20; i++ {
		lines := []string{}
		for j := oldCount - 1; j >= 0; j-- {
			// trim lead spaces in the line
			lines = append(lines, strings.TrimSpace(oldLinesMap[oldNode.Line-i-j-1]))
		}
		linesStr := strings.Join(lines, "\n")

		// if they match, adjust oldLineNumber
		if linesStr == oldTrimComment {
			oldLineNumber = oldNode.Line - i
			break
		}

	}

	// find newLineNumber
	newLineNumber := newNode.Line
	found := false
	sistersAndAunts := []*compare.Node{}
	if newNode.ParentNode != nil {
		sistersAndAunts = append(sistersAndAunts, newNode.ParentNode.NodeContent...)
		if newNode.ParentNode.ParentNode != nil {
			sistersAndAunts = append(sistersAndAunts, newNode.ParentNode.ParentNode.NodeContent...)
		}
	}
Found:
	for _, node := range sistersAndAunts {
		// get lines above node
		lines := []string{}
		for i := oldCount; i > 0; i-- {
			// trim lead spaces in the line
			lines = append(lines, strings.TrimSpace(newLinesMap[node.Line-i]))
		}
		linesStr := strings.Join(lines, "\n")

		// if they match, adjust newLineNumber
		if linesStr == strings.TrimSpace(oldNode.HeadComment) {
			found = true
			newLineNumber = node.Line
			break Found
		}
	}

	// update values with old versions
	for index := 0; index < oldCount; index++ {
		newLinesMap[newLineNumber-index-1] = oldLinesMap[oldLineNumber-index-1]
	}

	if !found {
		return fmt.Errorf("warning: unable to find where this HeadComment came from, skipping: '%s'", oldNode.HeadComment)
	}

	return nil
}

func fixLineComment(oldLinesMap, newLinesMap map[int]string, oldNode, newNode *compare.Node) error {
	oldLine := oldLinesMap[oldNode.Line]
	newLine := newLinesMap[newNode.Line]

	// get the comment and lead spaces from the oldLine
	escaped := regexp.QuoteMeta(oldNode.LineComment)
	afterYamlNode := regexp.MustCompile(fmt.Sprintf(`\s*%s$`, escaped))
	oldLocation := afterYamlNode.FindStringIndex(oldLine)
	if oldLocation == nil {
		return fmt.Errorf("program error: no match '%s' found in line '%s'", escaped, oldLine)
	}

	// get the location where the spaces and comments start in newLine
	newLocation := afterYamlNode.FindStringIndex(newLine)
	if newLocation == nil {
		return fmt.Errorf("program error: no match '%s' found in line '%s'", escaped, newLine)
	}

	// assemble the new newLine
	newLine = newLine[:newLocation[0]] + oldLine[oldLocation[0]:oldLocation[1]]
	newLinesMap[newNode.Line] = newLine

	return nil
}

func fixFootComment(oldLinesMap, newLinesMap map[int]string, oldNode, newNode *compare.Node) error {
	// find oldLineNumber
	oldCount := len(strings.Split(oldNode.FootComment, "\n"))
	oldLineNumber := oldNode.Line
	// check for up to 20 blank lines between this and footComment
	for i := 0; i < 20; i++ {
		lines := []string{}
		for j := 0; j < oldCount; j++ {
			// trim lead spaces in the line
			lines = append(lines, strings.TrimSpace(oldLinesMap[oldNode.Line+i+j+1]))
		}
		linesStr := strings.Join(lines, "\n")

		// if they match, adjust oldLineNumber
		if linesStr == oldNode.FootComment {
			oldLineNumber = oldNode.Line + i
			break
		}

	}

	// find newLineNumber
	newLineNumber := newNode.Line
	found := false
	sistersAndAunts := []*compare.Node{}
	if newNode.ParentNode != nil {
		sistersAndAunts = append(sistersAndAunts, newNode.ParentNode.NodeContent...)
		if newNode.ParentNode.ParentNode != nil {
			sistersAndAunts = append(sistersAndAunts, newNode.ParentNode.ParentNode.NodeContent...)
		}
	}
Found:
	for _, node := range sistersAndAunts {
		// get lines below node
		lines := []string{}
		for i := 0; i < oldCount; i++ {
			// trim lead spaces in the line
			lines = append(lines, strings.TrimSpace(newLinesMap[node.Line+i+1]))
		}
		linesStr := strings.Join(lines, "\n")

		// if they match, adjust newLineNumber
		if linesStr == oldNode.FootComment {
			found = true
			newLineNumber = node.Line
			break Found
		}
	}

	// update values with old versions
	for index := 0; index < oldCount; index++ {
		newLinesMap[newLineNumber+index+1] = oldLinesMap[oldLineNumber+index+1]
	}

	if !found {
		return fmt.Errorf("warning: unable to find where this FootComment came from, skipping: '%s'", oldNode.FootComment)
	}

	return nil
}
