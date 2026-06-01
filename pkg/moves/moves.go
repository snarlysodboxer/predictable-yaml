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
package moves

import (
	"fmt"
	"os"
	"strings"

	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"gopkg.in/yaml.v3"
)

// KeyInfo holds a key name plus information about its value for display.
type KeyInfo struct {
	Key       string
	ValueKind yaml.Kind // ScalarNode, MappingNode, SequenceNode
	Value     string    // scalar value, empty for maps/sequences
}

// valueDisplay returns the YAML-like value representation.
func (k KeyInfo) valueDisplay() string {
	switch k.ValueKind {
	case yaml.MappingNode:
		return "{...}"
	case yaml.SequenceNode:
		return "[...]"
	default:
		return k.Value
	}
}

// MoveDescription describes a structural change at a particular YAML path.
type MoveDescription struct {
	Path   string // e.g., "metadata", "spec.template.spec.containers[0]"
	Keys   []KeyInfo
	Action string // e.g., "move before labels", "move to top"
}

// ComputeDescriptions compares old and new YAML node trees and returns
// structural descriptions of what moved.
func ComputeDescriptions(oldNode, newNode *compare.Node) []MoveDescription {
	var descriptions []MoveDescription
	walkDescriptions(oldNode, newNode, "", &descriptions)

	return descriptions
}

func walkDescriptions(oldNode, newNode *compare.Node, path string, descriptions *[]MoveDescription) {
	switch oldNode.Kind {
	case yaml.DocumentNode:
		if newNode.Kind != yaml.DocumentNode || len(oldNode.NodeContent) == 0 || len(newNode.NodeContent) == 0 {
			return
		}
		walkDescriptions(oldNode.NodeContent[0], newNode.NodeContent[0], path, descriptions)

	case yaml.MappingNode:
		if newNode.Kind != yaml.MappingNode {
			return
		}

		oldPairs := compare.GetKeyValuePairs(oldNode.NodeContent)
		newPairs := compare.GetKeyValuePairs(newNode.NodeContent)

		// Find keys that moved position
		moves := findMoves(oldPairs, newPairs)
		if len(moves) > 0 {
			for _, m := range moves {
				desc := MoveDescription{
					Path:   path,
					Keys:   m.keys,
					Action: m.action,
				}
				*descriptions = append(*descriptions, desc)
			}
		}

		// Recurse into children
		for _, newPair := range newPairs {
			for _, oldPair := range oldPairs {
				if oldPair.Key != newPair.Key {
					continue
				}
				childPath := newPair.Key
				if path != "" {
					childPath = path + "." + newPair.Key
				}

				if oldPair.ValueNode.Kind == yaml.MappingNode || oldPair.ValueNode.Kind == yaml.SequenceNode {
					walkDescriptions(oldPair.ValueNode, newPair.ValueNode, childPath, descriptions)
				}
				break
			}
		}

	case yaml.SequenceNode:
		if newNode.Kind != yaml.SequenceNode {
			return
		}
		// Sequences aren't reordered, but recurse into items
		for i, oldItem := range oldNode.NodeContent {
			if i >= len(newNode.NodeContent) {
				break
			}
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			walkDescriptions(oldItem, newNode.NodeContent[i], itemPath, descriptions)
		}
	}
}

type moveGroup struct {
	keys   []KeyInfo
	action string
}

// findMoves compares old and new key orderings and returns descriptions
// of keys that were promoted (moved earlier). Keys that were merely pushed
// down as a consequence are not reported.
func findMoves(oldPairs, newPairs []compare.KeyValuePair) []moveGroup {
	if len(oldPairs) == 0 || len(newPairs) == 0 {
		return nil
	}

	// Build position maps
	oldPos := map[string]int{}
	for i, pair := range oldPairs {
		oldPos[pair.Key] = i
	}
	newPos := map[string]int{}
	newPairMap := map[string]compare.KeyValuePair{}
	for i, pair := range newPairs {
		newPos[pair.Key] = i
		newPairMap[pair.Key] = pair
	}

	// Get shared keys in new order
	type keyEntry struct {
		key    string
		oldIdx int
		newIdx int
	}
	var sharedKeyEntries []keyEntry
	for _, pair := range newPairs {
		if oldIndex, ok := oldPos[pair.Key]; ok {
			sharedKeyEntries = append(sharedKeyEntries, keyEntry{key: pair.Key, oldIdx: oldIndex, newIdx: newPos[pair.Key]})
		}
	}

	if len(sharedKeyEntries) == 0 {
		return nil
	}

	// Only report keys that moved earlier (promoted) and whose relative
	// order actually changed vs at least one other key.
	var moves []moveGroup
	for _, sharedKeyEntry := range sharedKeyEntries {
		if sharedKeyEntry.newIdx >= sharedKeyEntry.oldIdx {
			continue // didn't move earlier
		}

		// Verify relative order actually changed
		relativelyMoved := false
		for _, other := range sharedKeyEntries {
			if other.key == sharedKeyEntry.key {
				continue
			}
			oldBefore := sharedKeyEntry.oldIdx < other.oldIdx
			newBefore := sharedKeyEntry.newIdx < other.newIdx
			if oldBefore != newBefore {
				relativelyMoved = true
				break
			}
		}
		if !relativelyMoved {
			continue
		}

		action := ""
		if sharedKeyEntry.newIdx == 0 {
			action = "move to top"
		} else {
			action = "move up"
		}

		pair := newPairMap[sharedKeyEntry.key]
		keyInfo := KeyInfo{
			Key:       sharedKeyEntry.key,
			ValueKind: pair.ValueNode.Kind,
			Value:     pair.ValueNode.Value,
		}

		moves = append(moves, moveGroup{
			keys:   []KeyInfo{keyInfo},
			action: action,
		})
	}

	return mergeConsecutiveMoves(moves)
}

func mergeConsecutiveMoves(moves []moveGroup) []moveGroup {
	if len(moves) <= 1 {
		return moves
	}

	merged := []moveGroup{moves[0]}
	for i := 1; i < len(moves); i++ {
		last := &merged[len(merged)-1]
		if last.action == moves[i].action {
			last.keys = append(last.keys, moves[i].keys...)
		} else {
			merged = append(merged, moves[i])
		}
	}

	return merged
}

// summaryNode is a tree node used to build a nested summary display.
type summaryNode struct {
	segment  string // path segment, e.g. "metadata", "containers[0]"
	moves    []MoveDescription
	added    []string // leaf keys that were added as required fields
	children []*summaryNode
}

func (n *summaryNode) findOrCreateChild(segment string) *summaryNode {
	for _, child := range n.children {
		if child.segment == segment {
			return child
		}
	}
	child := &summaryNode{segment: segment}
	n.children = append(n.children, child)

	return child
}

// splitPath splits a dotted path like "spec.template.spec.containers[0]"
// into segments: ["spec", "template", "spec", "containers[0]"]
func splitPath(path string) []string {
	if path == "" {
		return nil
	}

	return strings.Split(path, ".")
}

// FormatSummary produces a human-readable summary of changes for a file.
// The output nests paths hierarchically to resemble a YAML structure.
func FormatSummary(filePath string, descriptions []MoveDescription, addedFields []compare.AddedField, commentCount int) string {
	if len(descriptions) == 0 && len(addedFields) == 0 {
		return ""
	}

	color := IsTerminal()

	var stringBuilder strings.Builder
	fmt.Fprintf(&stringBuilder, "File: %s\n", filePath)

	// Merge descriptions that share the same path and action
	descriptions = mergeDescriptions(descriptions)

	// Build a tree from the flat descriptions and added fields
	root := &summaryNode{}
	for _, desc := range descriptions {
		segments := splitPath(desc.Path)
		node := root
		for _, seg := range segments {
			node = node.findOrCreateChild(seg)
		}
		node.moves = append(node.moves, desc)
	}

	// Insert added fields into the same tree.
	// Each AddedField has a separate Path (e.g. ".metadata.labels") and
	// Key (e.g. "app.kubernetes.io/name"), so we can split the path
	// correctly without ambiguity from dots in key names.
	for _, field := range addedFields {
		segments := splitPath(strings.TrimPrefix(field.Path, "."))
		node := root
		for _, seg := range segments {
			node = node.findOrCreateChild(seg)
		}
		node.added = append(node.added, field.Key)
	}

	stringBuilder.WriteString("\n  Changes:\n")
	renderTree(&stringBuilder, root, "    ", color)

	if commentCount > 0 {
		fmt.Fprintf(&stringBuilder, "\n  Comments: all %d comments preserved\n", commentCount)
	}

	return stringBuilder.String()
}

// IsTerminal reports whether stdout is connected to a terminal.
func IsTerminal() bool {
	file, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return file.Mode()&os.ModeCharDevice != 0
}

// mergeDescriptions merges descriptions that share the same path and action.
func mergeDescriptions(descriptions []MoveDescription) []MoveDescription {
	type key struct {
		path   string
		action string
	}
	order := []key{}
	merged := map[key]*MoveDescription{}

	for _, decription := range descriptions {
		k := key{path: decription.Path, action: decription.Action}
		if existing, ok := merged[k]; ok {
			existing.Keys = append(existing.Keys, decription.Keys...)
		} else {
			desc := decription
			merged[k] = &desc
			order = append(order, k)
		}
	}

	result := make([]MoveDescription, 0, len(order))
	for _, k := range order {
		result = append(result, *merged[k])
	}

	return result
}

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33;1m" // bold yellow — attention-grabbing but not "error red"
)

func renderTree(stringBuilder *strings.Builder, node *summaryNode, indent string, color bool) {
	// Render this node's segment as a heading if it has one
	if node.segment != "" {
		fmt.Fprintf(stringBuilder, "%s%s:\n", indent, node.segment)
		indent += "  "
	}

	// Render moves at this level - each key on its own line, YAML-style
	for _, move := range node.moves {
		for _, keyInfo := range move.Keys {
			comment := fmt.Sprintf("# %s", move.Action)
			if color {
				comment = colorGreen + comment + colorReset
			}
			fmt.Fprintf(stringBuilder, "%s%s: %s  %s\n", indent, keyInfo.Key, keyInfo.valueDisplay(), comment)
		}
	}

	// Render added fields at this level
	for _, key := range node.added {
		comment := "# add"
		if color {
			comment = colorYellow + comment + colorReset
		}
		fmt.Fprintf(stringBuilder, "%s%s: TODO  %s\n", indent, key, comment)
	}

	// Render children
	for _, child := range node.children {
		renderTree(stringBuilder, child, indent, color)
	}
}

// CountComments counts the total number of comments in a YAML node tree.
func CountComments(node *compare.Node) int {
	count := 0
	walkCountComments(node, &count)

	return count
}

func walkCountComments(node *compare.Node, count *int) {
	if node.HeadComment != "" {
		*count++
	}
	if node.LineComment != "" {
		*count++
	}
	if node.FootComment != "" {
		*count++
	}
	for _, child := range node.NodeContent {
		walkCountComments(child, count)
	}
}
