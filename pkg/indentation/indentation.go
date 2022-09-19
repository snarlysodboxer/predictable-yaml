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
package indentation

import (
	"bufio"
	"bytes"
	"regexp"

	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"gopkg.in/yaml.v3"
)

type startStop struct {
	start     int
	stop      int
	goesToEnd bool
}

// FixLists can unindent lists in yaml. Expects consistent input indentation.
func FixLists(content []byte, reduceBy int) ([]byte, error) {
	// parse yaml
	yamlNode := &yaml.Node{}
	err := yaml.Unmarshal(content, yamlNode)
	if err != nil {
		return content, err
	}
	node := &compare.Node{Node: yamlNode}
	compare.WalkConvertYamlNodeToMainNode(node)

	// create a slice of lines
	lines := []string{}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return []byte{}, err
	}

	// get the list of lines that start/stop sequences
	sequences := walkGetStartStopSequences(node, []*startStop{})

	// convert to map
	lineMap := map[int]string{}
	for index, line := range lines {
		lineMap[index] = line
	}

	// unindent those lines in the map by reduceBy spaces
	fmtStr := "^"
	for i := 1; i <= reduceBy; i++ {
		fmtStr += " "
	}
	firstNSpaces := regexp.MustCompile(fmtStr)
	for _, ss := range sequences {
		if ss.goesToEnd {
			for i := ss.start; i <= len(lineMap)-1; i++ {
				lineMap[i] = string(firstNSpaces.ReplaceAll([]byte(lineMap[i]), []byte{}))
			}
			continue
		}
		for i := ss.start; i <= ss.stop; i++ {
			lineMap[i] = string(firstNSpaces.ReplaceAll([]byte(lineMap[i]), []byte{}))
		}
	}

	// reassemble
	newContent := ""
	for i := 0; i < len(lineMap); i++ {
		newContent += lineMap[i] + "\n"
	}

	return []byte(newContent), nil
}

func walkGetStartStopSequences(n *compare.Node, sequences []*startStop) []*startStop {
	// work against n's NodeContent
	end := len(n.NodeContent) - 1
	for index, node := range n.NodeContent {
		sequences = walkGetStartStopSequences(node, sequences)

		// only work with kind Sequence
		if node.Kind != yaml.SequenceNode {
			continue
		}
		// don't manage indentation for FlowStyle (inline)
		if node.Style == yaml.FlowStyle {
			continue
		}

		ss := startStop{start: node.Line - 1}
		switch {
		case len(n.NodeContent) == 1:
			// if the NodeContent length is only 1, then end is same as start
			ss.stop = node.Line - 1
		case index+1 <= end:
			// if there is a node following this one in NodeContent slice, then
			//   it's the line of that node minus 2
			ss.stop = n.NodeContent[index+1].Line - 2
		case index+1 > end:
			// find parent or parent of parent with following sibling, if exists
			parentStop, found := walkGetParentStop(node)
			if found {
				// is end of NodeContent but not end of file
				ss.stop = parentStop
			} else {
				// is end of file
				ss.goesToEnd = true
			}
		}
		sequences = append(sequences, &ss)
	}

	return sequences
}

// find parent or parent of parent with following sibling, if exists
func walkGetParentStop(n *compare.Node) (int, bool) {
	if n.ParentNode == nil {
		return 0, false
	}

	end := len(n.ParentNode.NodeContent) - 1
	for index, node := range n.ParentNode.NodeContent {
		if node == n {
			if index+1 <= end {
				return n.ParentNode.NodeContent[index+1].Line - 2, true
			}
			break
		}
	}

	return walkGetParentStop(n.ParentNode)
}
