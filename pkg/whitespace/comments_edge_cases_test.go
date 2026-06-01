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
	"bytes"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestCommentPreservationEdgeCases tests PreserveComments with explicit
// oldContent/newContent pairs covering edge cases.
// The newContent is what yaml.v3 produces after parse→marshal (with possible
// reordering by WalkAndSort), and PreserveComments should restore original
// comment spacing from oldContent.
func TestCommentPreservationEdgeCases(t *testing.T) {
	type testCase struct {
		note            string
		oldContent      string
		newContent      string
		expectedContent string
	}

	testCases := []testCase{
		// --- Comment placement edge cases ---
		{
			note: "head comment on first key in a mapping",
			oldContent: `# this is the api version
apiVersion: apps/v1
kind: Deployment`,
			newContent: `# this is the api version
apiVersion: apps/v1
kind: Deployment`,
			expectedContent: `# this is the api version
apiVersion: apps/v1
kind: Deployment`,
		},
		{
			note: "head comment on a key in the middle of a mapping",
			oldContent: `apiVersion: apps/v1
# this is the kind
kind: Deployment
metadata:
  name: test`,
			newContent: `apiVersion: apps/v1
# this is the kind
kind: Deployment
metadata:
  name: test`,
			expectedContent: `apiVersion: apps/v1
# this is the kind
kind: Deployment
metadata:
  name: test`,
		},
		{
			note: "line comment after a value with custom spacing",
			oldContent: `apiVersion: apps/v1
kind: Deployment  # two spaces
metadata:
  name: test`,
			newContent: `apiVersion: apps/v1
kind: Deployment # two spaces
metadata:
  name: test`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment  # two spaces
metadata:
  name: test`,
		},
		{
			note: "line comment on a key before the colon with custom spacing",
			oldContent: `apiVersion: apps/v1
kind: Deployment
metadata:   # three spaces
  name: test`,
			newContent: `apiVersion: apps/v1
kind: Deployment
metadata: # three spaces
  name: test`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment
metadata:   # three spaces
  name: test`,
		},
		{
			note: "multiple consecutive comment lines (comment block)",
			oldContent: `apiVersion: apps/v1
# line one of block
# line two of block
# line three of block
kind: Deployment`,
			newContent: `apiVersion: apps/v1
# line one of block
# line two of block
# line three of block
kind: Deployment`,
			expectedContent: `apiVersion: apps/v1
# line one of block
# line two of block
# line three of block
kind: Deployment`,
		},
		{
			note: "comment on a null/empty value node",
			oldContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: # empty value with comment
  namespace: default`,
			newContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: # empty value with comment
  namespace: default`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: # empty value with comment
  namespace: default`,
		},
		{
			note: "comment on a node whose value is a mapping",
			oldContent: `apiVersion: apps/v1
metadata: # mapping comment
  name: test
  namespace: default`,
			newContent: `apiVersion: apps/v1
metadata: # mapping comment
  name: test
  namespace: default`,
			expectedContent: `apiVersion: apps/v1
metadata: # mapping comment
  name: test
  namespace: default`,
		},
		{
			note: "comment that contains special YAML characters",
			oldContent: `apiVersion: apps/v1
kind: Deployment # contains: colons, - dashes, # hashes
metadata:
  name: test`,
			newContent: `apiVersion: apps/v1
kind: Deployment # contains: colons, - dashes, # hashes
metadata:
  name: test`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment # contains: colons, - dashes, # hashes
metadata:
  name: test`,
		},
		{
			note: "very long comment line with custom spacing",
			oldContent: `apiVersion: apps/v1
kind: Deployment  # this is a very long comment that goes on and on and on and on and on and on and on and on and on to test preservation
metadata:
  name: test`,
			newContent: `apiVersion: apps/v1
kind: Deployment # this is a very long comment that goes on and on and on and on and on and on and on and on and on to test preservation
metadata:
  name: test`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment  # this is a very long comment that goes on and on and on and on and on and on and on and on and on to test preservation
metadata:
  name: test`,
		},
		{
			note: "comment-only lines between keys (head comment block)",
			oldContent: `apiVersion: apps/v1
# standalone comment between keys
# another standalone line
kind: Deployment
metadata:
  name: test`,
			newContent: `apiVersion: apps/v1
# standalone comment between keys
# another standalone line
kind: Deployment
metadata:
  name: test`,
			expectedContent: `apiVersion: apps/v1
# standalone comment between keys
# another standalone line
kind: Deployment
metadata:
  name: test`,
		},

		// --- Reordering + comment interaction ---
		// These tests simulate WalkAndSort having already reordered the keys.
		// PreserveComments then restores original comment spacing.
		{
			note: "head comment attached to a key that moved position",
			oldContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: default
  # comment on name
  name: test`,
			newContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  # comment on name
  name: test
  namespace: default`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  # comment on name
  name: test
  namespace: default`,
		},
		{
			note: "head comment attached to a key that stays while neighbors move",
			oldContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  # comment on name stays
  name: test
  labels:
    app: test
  namespace: default`,
			newContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  # comment on name stays
  name: test
  namespace: default
  labels:
    app: test`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  # comment on name stays
  name: test
  namespace: default
  labels:
    app: test`,
		},
		{
			note: "inline comments on multiple keys that get reordered",
			oldContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  labels:  # label comment
    app: test
  name: test  # name comment
  namespace: default  # ns comment`,
			newContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test # name comment
  namespace: default # ns comment
  labels: # label comment
    app: test`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test  # name comment
  namespace: default  # ns comment
  labels:  # label comment
    app: test`,
		},
		{
			note: "head comment with non-standard indentation, using head comment fixer",
			oldContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: example

# this is a head comment, one line
  name: example # one space
spec:
  template:
    spec:
        # this is a head comment, multiline
        # blah blah blah
        # maybe you don't know
        # what containers are :}
      containers:       # fdsa
      - name: cool-app`,
			newContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  # this is a head comment, one line
  name: example # one space
  namespace: example
spec:
  template:
    spec:
      # this is a head comment, multiline
      # blah blah blah
      # maybe you don't know
      # what containers are :}
      containers: # fdsa
      - name: cool-app`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment
metadata:
# this is a head comment, one line
  name: example # one space
  namespace: example
spec:
  template:
    spec:
        # this is a head comment, multiline
        # blah blah blah
        # maybe you don't know
        # what containers are :}
      containers:       # fdsa
      - name: cool-app`,
		},
		{
			note: "foot comment after a value",
			oldContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: example
    # this counts as a footer comment for 'example'
    # and there's more to say about it`,
			newContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: example
  # this counts as a footer comment for 'example'
  # and there's more to say about it`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: example
    # this counts as a footer comment for 'example'
    # and there's more to say about it`,
		},
		{
			note: "comment between sequence items (head comment on second item)",
			oldContent: `spec:
  containers:
    - name: app1
      image: img1
    # between containers
    - name: app2
      image: img2`,
			newContent: `spec:
  containers:
    - name: app1
      image: img1
    # between containers
    - name: app2
      image: img2`,
			expectedContent: `spec:
  containers:
    - name: app1
      image: img1
    # between containers
    - name: app2
      image: img2`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.note, func(t *testing.T) {
			got, err := PreserveComments([]byte(tc.oldContent), []byte(tc.newContent))
			if err != nil {
				t.Fatalf("PreserveComments error: %v", err)
			}
			if string(got) != tc.expectedContent {
				t.Errorf("PreserveComments mismatch:\n-expected:\n%s\n+got:\n%s\n", tc.expectedContent, string(got))
			}
		})
	}
}

// TestCommentRoundTripThroughYamlV3 tests what yaml.v3 does to comments
// during parse → marshal, documenting known behaviors and limitations.
func TestCommentRoundTripThroughYamlV3(t *testing.T) {
	type testCase struct {
		note            string
		input           string
		expectedRaw     string // after marshal only (no PreserveComments)
		expectedFixed   string // after marshal + PreserveComments
		expectFixChange bool   // true if PreserveComments should change the output
	}

	testCases := []testCase{
		{
			note:  "inline comment spacing is normalized by yaml.v3",
			input: `kind: Deployment  # two spaces`,
			expectedRaw: `kind: Deployment # two spaces
`,
			expectedFixed:   `kind: Deployment  # two spaces`,
			expectFixChange: true,
		},
		{
			note: "head comment indentation is normalized by yaml.v3",
			input: `metadata:
    # indented head comment
  name: test`,
			expectedRaw: `metadata:
    # indented head comment
    name: test
`,
			expectedFixed: `metadata:
    # indented head comment
    name: test`,
			expectFixChange: false,
		},
		{
			note: "foot comment is preserved through round-trip",
			input: `metadata:
  name: test
  # foot comment on metadata`,
			expectedRaw: `metadata:
    name: test
    # foot comment on metadata
`,
			expectedFixed: `metadata:
    name: test
  # foot comment on metadata`,
			expectFixChange: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.note, func(t *testing.T) {
			node := &yaml.Node{}
			marshaled, err := roundTrip([]byte(tc.input), node, 4)
			if err != nil {
				t.Fatalf("round-trip error: %v", err)
			}

			if string(marshaled) != tc.expectedRaw {
				t.Errorf("raw round-trip mismatch:\n-expected:\n%q\n+got:\n%q\n", tc.expectedRaw, string(marshaled))
			}

			fixed, err := PreserveComments([]byte(tc.input), marshaled)
			if err != nil {
				t.Fatalf("PreserveComments error: %v", err)
			}

			fixedStr := strings.TrimRight(string(fixed), "\n")
			if fixedStr != tc.expectedFixed {
				t.Errorf("fixed mismatch:\n-expected:\n%s\n+got:\n%s\n", tc.expectedFixed, fixedStr)
			}

			if tc.expectFixChange && string(marshaled) == string(fixed) {
				t.Error("expected PreserveComments to change the output, but it didn't")
			}
		})
	}
}

// TestHeadCommentAboveDocumentStartNotDuplicated tests that a comment above
// a --- document start marker is not duplicated by PreserveComments.
func TestHeadCommentAboveDocumentStartNotDuplicated(t *testing.T) {
	t.Run("comment above document start marker is not duplicated", func(t *testing.T) {
		// Bug: fixHeadComment wrote replacement lines even when it couldn't find
		// where the comment went, causing duplication.
		oldContent := `# file-level comment
---
apiVersion: apps/v1
kind: Deployment`
		newContent := `# file-level comment
---
apiVersion: apps/v1
kind: Deployment`

		got, err := PreserveComments([]byte(oldContent), []byte(newContent))
		if err != nil {
			t.Fatalf("PreserveComments error: %v", err)
		}
		count := strings.Count(string(got), "# file-level comment")
		if count != 1 {
			t.Errorf("comment appears %d times, expected 1:\n%s", count, string(got))
		}
	})
}

// TestFullPipelineCommentAndEmptyLines tests PreserveComments and
// PreserveEmptyLines working together, since they run sequentially
// in the fix command.
func TestFullPipelineCommentAndEmptyLines(t *testing.T) {
	type testCase struct {
		note            string
		oldContent      string
		newContent      string
		expectedContent string
	}

	testCases := []testCase{
		{
			note: "multiple empty lines above a comment are preserved",
			oldContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: default


  # comment with two blank lines above
  name: test`,
			newContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  # comment with two blank lines above
  name: test
  namespace: default`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment
metadata:


  # comment with two blank lines above
  name: test
  namespace: default`,
		},
		{
			note: "three empty lines between keys are preserved",
			oldContent: `apiVersion: apps/v1



kind: Deployment
metadata:
  name: test`,
			newContent: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test`,
			expectedContent: `apiVersion: apps/v1



kind: Deployment
metadata:
  name: test`,
		},
		{
			note: "empty lines both above and below a comment",
			oldContent: `apiVersion: apps/v1

# important note

kind: Deployment
metadata:
  name: test`,
			newContent: `apiVersion: apps/v1
# important note
kind: Deployment
metadata:
  name: test`,
			expectedContent: `apiVersion: apps/v1

# important note

kind: Deployment
metadata:
  name: test`,
		},
		{
			note: "comment and empty lines on a key that moves",
			oldContent: `metadata:

  labels:
    app: test

  # about name
  name: test
  namespace: default`,
			newContent: `metadata:
  # about name
  name: test
  namespace: default
  labels:
    app: test`,
			// Empty lines get associated with the node below them.
			// The empty line above "labels" stays above "labels" in new position.
			// The empty line above "# about name" stays above name's comment.
			expectedContent: `metadata:

  # about name
  name: test
  namespace: default

  labels:
    app: test`,
		},
		{
			note: "inline comment spacing preserved alongside empty lines",
			oldContent: `apiVersion: apps/v1
kind: Deployment  # two spaces

metadata:   # three spaces
  name: test`,
			newContent: `apiVersion: apps/v1
kind: Deployment # two spaces
metadata: # three spaces
  name: test`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment  # two spaces

metadata:   # three spaces
  name: test`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.note, func(t *testing.T) {
			// Run the same pipeline as cmd/fix.go: PreserveComments then PreserveEmptyLines
			got, err := PreserveComments([]byte(tc.oldContent), []byte(tc.newContent))
			if err != nil {
				t.Fatalf("PreserveComments error: %v", err)
			}
			got, err = PreserveEmptyLines([]byte(tc.oldContent), got)
			if err != nil {
				t.Fatalf("PreserveEmptyLines error: %v", err)
			}
			gotStr := strings.TrimRight(string(got), "\n")
			if gotStr != tc.expectedContent {
				t.Errorf("full pipeline mismatch:\n-expected:\n%s\n+got:\n%s\n", tc.expectedContent, gotStr)
			}
		})
	}
}

// TestKnownLimitations documents cases where comment handling has known
// limitations due to yaml.v3 behavior that we cannot fix.
func TestKnownLimitations(t *testing.T) {
	t.Run("head comment indentation normalized by yaml.v3 cannot be restored", func(t *testing.T) {
		// yaml.v3 normalizes head comment indentation to match the node's
		// indentation level. If the original had non-standard indentation
		// (e.g., extra indentation), the marshal step changes it and
		// PreserveComments can only fix the spacing, not the indent level
		// when yaml.v3 has already re-indented during parse.
		//
		// However, the fixHeadComment function in comments.go does handle
		// many non-standard indentation cases by searching for comment text
		// in sisters/aunts. This test just documents the behavior.
		oldContent := `metadata:
      # over-indented head comment
  name: test`

		node := &yaml.Node{}
		marshaled, err := roundTrip([]byte(oldContent), node, 2)
		if err != nil {
			t.Fatalf("round-trip error: %v", err)
		}

		// yaml.v3 normalizes the head comment indentation
		if !strings.Contains(string(marshaled), "  # over-indented head comment") {
			t.Log("yaml.v3 changed the head comment indentation (expected behavior)")
		}
	})

	t.Run("comment above document start marker triggers warning", func(t *testing.T) {
		// When a comment appears above --- and yaml.v3 attaches it as a
		// HeadComment on the first key, fixHeadComment can't find where it
		// came from in the tree and logs a WARNING. The comment is still
		// preserved in the output (yaml.v3 puts it there), but the spacing
		// restoration doesn't apply.
		oldContent := `# file-level comment
---
apiVersion: apps/v1
kind: Deployment`

		newContent := `# file-level comment
---
apiVersion: apps/v1
kind: Deployment`

		got, err := PreserveComments([]byte(oldContent), []byte(newContent))
		if err != nil {
			t.Fatalf("PreserveComments error: %v", err)
		}
		// The comment should appear exactly once (bug was fixed)
		count := strings.Count(string(got), "# file-level comment")
		if count != 1 {
			t.Errorf("comment appears %d times, expected 1", count)
		}
	})
}

func roundTrip(content []byte, node *yaml.Node, indent int) ([]byte, error) {
	err := yaml.Unmarshal(content, node)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(indent)
	err = encoder.Encode(node)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
