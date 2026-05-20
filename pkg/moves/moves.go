// Package moves computes line-range moves between two YAML texts by comparing
// their node trees to identify key reorderings at each mapping level.
package moves

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Move describes a block of lines that moved from one position to another.
// Line numbers are 1-indexed and inclusive.
type Move struct {
	FromStart, FromEnd int
	ToStart, ToEnd     int
}

// Compute parses oldText and newText as YAML, walks both trees in parallel,
// and returns a list of moves describing which line ranges were reordered.
func Compute(oldText, newText string) ([]Move, error) {
	var oldDoc, newDoc yaml.Node
	if err := yaml.Unmarshal([]byte(oldText), &oldDoc); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal([]byte(newText), &newDoc); err != nil {
		return nil, err
	}

	oldLines := strings.Split(strings.TrimRight(oldText, "\n"), "\n")
	newLines := strings.Split(strings.TrimRight(newText, "\n"), "\n")

	var moves []Move
	walkAndFindMoves(&oldDoc, &newDoc, oldLines, newLines, &moves)
	return moves, nil
}

func walkAndFindMoves(oldNode, newNode *yaml.Node, oldLines, newLines []string, moves *[]Move) {
	if oldNode == nil || newNode == nil {
		return
	}

	switch oldNode.Kind {
	case yaml.DocumentNode:
		if newNode.Kind != yaml.DocumentNode {
			return
		}
		if len(oldNode.Content) > 0 && len(newNode.Content) > 0 {
			walkAndFindMoves(oldNode.Content[0], newNode.Content[0], oldLines, newLines, moves)
		}

	case yaml.MappingNode:
		if newNode.Kind != yaml.MappingNode {
			return
		}
		findMappingMoves(oldNode, newNode, oldLines, newLines, moves)

		// Recurse into matching value nodes
		oldPairs := getMappingPairs(oldNode)
		newPairs := getMappingPairs(newNode)
		for _, op := range oldPairs {
			for _, np := range newPairs {
				if op.key == np.key {
					walkAndFindMoves(op.value, np.value, oldLines, newLines, moves)
					break
				}
			}
		}

	case yaml.SequenceNode:
		if newNode.Kind != yaml.SequenceNode {
			return
		}
		// Recurse into sequence items pairwise
		minLen := len(oldNode.Content)
		if len(newNode.Content) < minLen {
			minLen = len(newNode.Content)
		}
		for i := 0; i < minLen; i++ {
			walkAndFindMoves(oldNode.Content[i], newNode.Content[i], oldLines, newLines, moves)
		}
	}
}

type mappingPair struct {
	key     string
	keyNode *yaml.Node
	value   *yaml.Node
}

func getMappingPairs(node *yaml.Node) []mappingPair {
	var pairs []mappingPair
	for i := 0; i+1 < len(node.Content); i += 2 {
		pairs = append(pairs, mappingPair{
			key:     node.Content[i].Value,
			keyNode: node.Content[i],
			value:   node.Content[i+1],
		})
	}
	return pairs
}

// findMappingMoves compares key order in old vs new mapping nodes.
// For each key that changed position, it emits a Move with the full
// line range of that key-value pair.
func findMappingMoves(oldNode, newNode *yaml.Node, oldLines, newLines []string, moves *[]Move) {
	oldPairs := getMappingPairs(oldNode)
	newPairs := getMappingPairs(newNode)

	// Build a map of key -> position in each
	oldPos := make(map[string]int)
	newPos := make(map[string]int)
	for i, p := range oldPairs {
		oldPos[p.key] = i
	}
	for i, p := range newPairs {
		newPos[p.key] = i
	}

	// Check if the key order is actually different
	oldKeys := make([]string, 0, len(oldPairs))
	newKeys := make([]string, 0, len(newPairs))
	for _, p := range oldPairs {
		if _, ok := newPos[p.key]; ok {
			oldKeys = append(oldKeys, p.key)
		}
	}
	for _, p := range newPairs {
		if _, ok := oldPos[p.key]; ok {
			newKeys = append(newKeys, p.key)
		}
	}

	if keysEqual(oldKeys, newKeys) {
		return // same order, no moves
	}

	// For each key that exists in both and changed relative position,
	// emit a move
	for _, key := range oldKeys {
		oi := oldPos[key]
		ni, ok := newPos[key]
		if !ok {
			continue
		}
		if oi == ni {
			continue // same position
		}

		// Also check if it's truly reordered (not just shifted by additions/removals)
		// by comparing the relative order with its neighbors
		oldIdx := indexOf(oldKeys, key)
		newIdx := indexOf(newKeys, key)
		if oldIdx == newIdx {
			continue // same relative position among shared keys
		}

		// Get line ranges
		fromStart, fromEnd := pairLineRange(oldPairs[oi], oldLines)
		toStart, toEnd := pairLineRange(newPairs[ni], newLines)

		if fromStart > 0 && toStart > 0 {
			*moves = append(*moves, Move{
				FromStart: fromStart,
				FromEnd:   fromEnd,
				ToStart:   toStart,
				ToEnd:     toEnd,
			})
		}
	}
}

// pairLineRange returns the 1-indexed start and end lines of a key-value pair.
// The start is the key's line. The end is determined by finding the last line
// before the next sibling key starts (or end of the mapping).
func pairLineRange(pair mappingPair, lines []string) (int, int) {
	startLine := pair.keyNode.Line // yaml.Node.Line is 1-indexed
	endLine := nodeEndLine(pair.value, lines)
	if endLine < startLine {
		endLine = startLine
	}
	return startLine, endLine
}

// nodeEndLine computes the last line occupied by a node and its descendants.
func nodeEndLine(node *yaml.Node, lines []string) int {
	if node == nil {
		return 0
	}

	maxLine := node.Line

	// For nodes with content, recurse to find the deepest line
	for _, child := range node.Content {
		childEnd := nodeEndLine(child, lines)
		if childEnd > maxLine {
			maxLine = childEnd
		}
	}

	// For scalar nodes, check if the value spans multiple lines
	if node.Kind == yaml.ScalarNode && node.Style == yaml.LiteralStyle || node.Style == yaml.FoldedStyle {
		// Count the lines in the value
		valueLines := strings.Count(node.Value, "\n")
		if node.Line+valueLines > maxLine {
			maxLine = node.Line + valueLines
		}
	}

	return maxLine
}

func keysEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}
