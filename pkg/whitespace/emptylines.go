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
	"regexp"
	"sort"
	"strings"

	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"gopkg.in/yaml.v3"
)

type insertAboveLine struct {
	emptyLineCount int
	oldLineNumber  int
	lineNumber     int
}

type insertAboveLines []insertAboveLine

var emptyLine = regexp.MustCompile(`(?m)^\s*$[\r\n]*|[\r\n]+\s+\z`)

// PreserveEmptyLines attempts to re-add to newContent any empty lines
//   that were there in oldContent.
// current algorithm simply associates each empty line with the yaml node on the line below it if there is one. it then re-adds them to newContent above the matching yaml node. it also re-adds empty line(s) at the end of the file if there were any.
// this may not always have the desired effect, but should in many cases.
func PreserveEmptyLines(oldContent, newContent []byte) ([]byte, error) {
	// another way this might be able to be done:
	//	 find empty lines (including more than one together, keep count)
	//	 for each empty line,
	//	     if the node on the line above it has children on a following line, OR younger sisters,
	//	         associate the empty line(s) with that node
	//	     otherwise
	//	         associate the empty line(s) with the first previous sister node that has a following sibling, walking up the tree, or end of file if no sibling
	//	 reinsert the empty line after walking to the child of that node, the one with the highest line number

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

	oldLines := strings.Split(string(oldContent), "\n")
	newLines := strings.Split(string(newContent), "\n")

	// convert to map
	oldLinesMap := getLinesMap(oldContent)
	newLinesMap := getLinesMap(newContent)

	insertAboves, err := getLineNumbersToInsertAbove(oldLinesMap, newLinesMap, oldNode, newNode)
	if err != nil {
		return newContent, err
	}
	insertAboves = deduplicateInserts(insertAboves)
	sort.SliceStable(insertAboves, func(i, j int) bool {
		return insertAboves[i].lineNumber < insertAboves[j].lineNumber
	})

	// do it
	newLines = insertEmptyLines(insertAboves, newLines)

	// reinsert empty lines at end
	linesAtEnd := countEmptyLinesAbove(oldLinesMap, len(oldLines), 0)
	for i := 0; i < linesAtEnd; i++ {
		newLines = append(newLines, "")
	}

	content := strings.Join(newLines, "\n")

	return []byte(content), nil
}

func getLineNumbersToInsertAbove(oldLinesMap, newLinesMap map[int]string, oldNode, newNode *compare.Node) (insertAboveLines, error) {
	insertAboves := insertAboveLines{}

	switch oldNode.Kind {
	case yaml.DocumentNode:
		if newNode.Kind != yaml.DocumentNode {
			return insertAboves, fmt.Errorf("program error: expected Document: '%s'", compare.GetReferencePath(newNode, 0, ""))
		}

		return getLineNumbersToInsertAbove(oldLinesMap, newLinesMap, oldNode.NodeContent[0], newNode.NodeContent[0])
	case yaml.MappingNode:
		if newNode.Kind != yaml.MappingNode {
			return insertAboves, fmt.Errorf("program error: expected Mapping: '%s'", compare.GetReferencePath(newNode, 0, ""))
		}

		if oldNode.Style == yaml.FlowStyle {
			return insertAboves, nil
		}

		if oldNode.HeadComment != "" {
			commentsLineCount := len(strings.Split(oldNode.HeadComment, "\n"))
			oldLineNumber := oldNode.Line - commentsLineCount
			newLineNumber := newNode.Line - commentsLineCount
			if !emptyLine.MatchString(oldLinesMap[oldLineNumber-1]) {
				return insertAboves, nil
			}

			iAs := insertAboveLine{
				emptyLineCount: countEmptyLinesAbove(oldLinesMap, oldLineNumber-1, 1),
				oldLineNumber:  oldLineNumber,
				lineNumber:     newLineNumber,
			}

			insertAboves = append(insertAboves, iAs)
		}

		oldPairs := compare.GetKeyValuePairs(oldNode.NodeContent)
		newPairs := compare.GetKeyValuePairs(newNode.NodeContent)
		for _, oldPair := range oldPairs {
			for _, newPair := range newPairs {
				if oldPair.Key == newPair.Key {
					iAs, err := getLineNumbersToInsertAbove(oldLinesMap, newLinesMap, oldPair.KeyNode, newPair.KeyNode)
					if err != nil {
						return insertAboves, err
					}
					insertAboves = append(insertAboves, iAs...)

					// values
					if oldPair.ValueNode.Kind == yaml.ScalarNode {
						continue
					}
					iAs, err = getLineNumbersToInsertAbove(oldLinesMap, newLinesMap, oldPair.ValueNode, newPair.ValueNode)
					if err != nil {
						return insertAboves, err
					}
					insertAboves = append(insertAboves, iAs...)
					break
				}
			}
		}

	case yaml.SequenceNode:
		if newNode.Kind != yaml.SequenceNode {
			return insertAboves, fmt.Errorf("program error: expected Sequence: '%s'", compare.GetReferencePath(newNode, 0, ""))
		}

		if oldNode.Style == yaml.FlowStyle {
			return insertAboves, nil
		}

		// we don't reorder sequences, so just use index
		for oIndex, oNode := range oldNode.NodeContent {
			iAs, err := getLineNumbersToInsertAbove(oldLinesMap, newLinesMap, oNode, newNode.NodeContent[oIndex])
			if err != nil {
				return insertAboves, err
			}
			insertAboves = append(insertAboves, iAs...)
		}

	case yaml.ScalarNode:
		if newNode.Kind != yaml.ScalarNode {
			return insertAboves, fmt.Errorf("program error: expected Scalar: '%s'", compare.GetReferencePath(newNode, 0, ""))
		}

		// find empty lines above head comments
		if oldNode.HeadComment != "" {
			commentsLineCount := len(strings.Split(oldNode.HeadComment, "\n"))
			oldLineNumber := oldNode.Line - commentsLineCount
			newLineNumber := newNode.Line - commentsLineCount

			// comment may be at a new location and divided differently between
			//   HeadComment and FootComment, find where it went.
			if oldNode.HeadComment != newNode.HeadComment {
				oldTrimComment := strings.TrimSpace(oldNode.HeadComment)
				oldTrimCount := len(strings.Split(oldTrimComment, "\n"))
				for _, node := range newNode.ParentNode.NodeContent {
					// get lines above node
					newLines := []string{}
					for i := oldTrimCount - 1; i >= 0; i-- {
						newLines = append(newLines, newLinesMap[node.Line-1-i])
					}
					newLinesStr := strings.Join(newLines, "\n")

					if newLinesStr == oldTrimComment {
						newLineNumber = node.Line - oldTrimCount
						break
					}
				}
			}

			// create an insertAboveLine if needed
			if emptyLine.MatchString(oldLinesMap[oldLineNumber-1]) {
				iAs := insertAboveLine{
					emptyLineCount: countEmptyLinesAbove(oldLinesMap, oldLineNumber-1, 1),
					oldLineNumber:  oldLineNumber,
					lineNumber:     newLineNumber,
				}
				insertAboves = append(insertAboves, iAs)
			}
		}

		// find empty lines above scalar nodes
		oldLineNumber := oldNode.Line
		newLineNumber := newNode.Line

		// ensure there is a line above this one
		if _, ok := oldLinesMap[oldLineNumber-1]; !ok {
			return insertAboves, nil
		}

		// create an insertAboveLine if needed
		if !emptyLine.MatchString(oldLinesMap[oldLineNumber-1]) {
			return insertAboves, nil
		}

		iAs := insertAboveLine{
			emptyLineCount: countEmptyLinesAbove(oldLinesMap, oldLineNumber-1, 1),
			oldLineNumber:  oldLineNumber,
			lineNumber:     newLineNumber,
		}

		return append(insertAboves, iAs), nil
	}

	return insertAboves, nil
}

func countEmptyLinesAbove(linesMap map[int]string, lineNumber, count int) int {
	if _, ok := linesMap[lineNumber-1]; ok {
		if linesMap[lineNumber-1] == "" {
			count++
			return countEmptyLinesAbove(linesMap, lineNumber-1, count)
		}
	}

	return count
}

// insertEmptyLines adds empty lines where they should be.
//   requires a sorted `insertAboveLines`.
func insertEmptyLines(insertAboves insertAboveLines, lines []string) []string {
	if len(insertAboves) == 0 {
		return lines
	}

	// delete this one
	var iA insertAboveLine
	iA, insertAboves = insertAboves[0], insertAboves[1:]

	for i := 0; i < iA.emptyLineCount; i++ {
		lines = insertLine(lines, iA.lineNumber-1, "")
	}

	if len(insertAboves) == 0 {
		return lines
	}

	// bump remaining line numbers by emptyLineCount
	for i := range insertAboves {
		insertAboves[i].lineNumber = insertAboves[i].lineNumber + iA.emptyLineCount
	}

	return insertEmptyLines(insertAboves, lines)
}

func insertLine(lines []string, index int, value string) []string {
	if len(lines) == index {
		return append(lines, value)
	}
	lines = append(lines[:index+1], lines[index:]...)
	lines[index] = value

	return lines
}

func deduplicateInserts(inserts insertAboveLines) insertAboveLines {
	newInserts := insertAboveLines{}
	for _, insert := range inserts {
		exists := false
		for _, i := range newInserts {
			if insert.emptyLineCount == i.emptyLineCount &&
				insert.lineNumber == i.lineNumber &&
				insert.oldLineNumber == i.oldLineNumber {
				exists = true
				break
			}
		}
		if !exists {
			newInserts = append(newInserts, insert)
		}
	}

	return newInserts
}
