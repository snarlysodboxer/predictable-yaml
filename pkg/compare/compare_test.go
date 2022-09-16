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
	"bytes"
	"fmt"
	"testing"

	// "github.com/kylelemons/godebug/diff"

	"gopkg.in/yaml.v3"
)

func TestWalkConvertYamlNodeToMainNode(t *testing.T) {
	// document
	yDocument := &yaml.Node{
		Kind: yaml.DocumentNode,
	}

	// root map
	yRootMap := &yaml.Node{
		Kind: yaml.MappingNode,
	}
	yDocument.Content = append(yDocument.Content, yRootMap)

	// .spec
	ySpecScalar := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "spec",
	}
	yRootMap.Content = append(yRootMap.Content, ySpecScalar)
	ySpecMap := &yaml.Node{
		Kind: yaml.MappingNode,
	}
	yRootMap.Content = append(yRootMap.Content, ySpecMap)

	// .spec.template
	ySpecTemplateScalar := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "template",
	}
	ySpecMap.Content = append(ySpecMap.Content, ySpecTemplateScalar)
	ySpecTemplateMap := &yaml.Node{
		Kind: yaml.MappingNode,
	}
	ySpecMap.Content = append(ySpecMap.Content, ySpecTemplateMap)

	// .spec.template.spec
	ySpecTemplateSpecScalar := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "spec",
	}
	ySpecTemplateMap.Content = append(ySpecTemplateMap.Content, ySpecTemplateSpecScalar)
	ySpecTemplateSpecMap := &yaml.Node{
		Kind: yaml.MappingNode,
	}
	ySpecTemplateMap.Content = append(ySpecTemplateMap.Content, ySpecTemplateSpecMap)

	// .spec.template.spec.containers
	ySpecTemplateSpecContainersScalar := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "containers",
	}
	ySpecTemplateSpecMap.Content = append(ySpecTemplateSpecMap.Content, ySpecTemplateSpecContainersScalar)
	ySpecTemplateSpecContainersSequence := &yaml.Node{
		Kind: yaml.SequenceNode,
	}
	ySpecTemplateSpecMap.Content = append(ySpecTemplateSpecMap.Content, ySpecTemplateSpecContainersSequence)

	// .spec.template.spec.containers[0]
	ySpecTemplateSpecContainersZeroMap := &yaml.Node{
		Kind: yaml.MappingNode,
	}
	ySpecTemplateSpecContainersSequence.Content = append(ySpecTemplateSpecContainersSequence.Content, ySpecTemplateSpecContainersZeroMap)

	// .spec.template.spec.containers[0].name
	ySpecTemplateSpecContainersZeroNameScalar := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "name",
	}
	ySpecTemplateSpecContainersZeroMap.Content = append(ySpecTemplateSpecContainersZeroMap.Content, ySpecTemplateSpecContainersZeroNameScalar)
	ySpecTemplateSpecContainersZeroNameValueScalar := &yaml.Node{
		Kind:        yaml.ScalarNode,
		Value:       "doesn't matter",
		LineComment: "# first, required",
	}
	ySpecTemplateSpecContainersZeroMap.Content = append(ySpecTemplateSpecContainersZeroMap.Content, ySpecTemplateSpecContainersZeroNameValueScalar)

	// .spec.template.spec.containers[0].command
	ySpecTemplateSpecContainersZeroCommandScalar := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "command",
	}
	ySpecTemplateSpecContainersZeroMap.Content = append(ySpecTemplateSpecContainersZeroMap.Content, ySpecTemplateSpecContainersZeroCommandScalar)
	ySpecTemplateSpecContainersZeroCommandSequence := &yaml.Node{
		Kind: yaml.SequenceNode,
	}
	ySpecTemplateSpecContainersZeroMap.Content = append(ySpecTemplateSpecContainersZeroMap.Content, ySpecTemplateSpecContainersZeroCommandSequence)

	// do it
	node := &Node{Node: yDocument}
	WalkConvertYamlNodeToMainNode(node)

	type testCase struct {
		note              string
		expectedNode      *yaml.Node
		gotNode           *yaml.Node
		confirmConfigNode *yaml.Node
	}
	testCases := []testCase{
		{
			note:              "expected root map",
			expectedNode:      yRootMap,
			gotNode:           node.NodeContent[0].Node,
			confirmConfigNode: node.Content[0],
		},
		{
			note:              "expected .spec scalar",
			expectedNode:      ySpecScalar,
			gotNode:           node.NodeContent[0].NodeContent[0].Node,
			confirmConfigNode: node.Content[0].Content[0],
		},
		{
			note:              "expected .spec map",
			expectedNode:      ySpecMap,
			gotNode:           node.NodeContent[0].NodeContent[1].Node,
			confirmConfigNode: node.Content[0].Content[1],
		},
		{
			note:              "expected .spec.template scalar",
			expectedNode:      ySpecTemplateScalar,
			gotNode:           node.NodeContent[0].NodeContent[1].NodeContent[0].Node,
			confirmConfigNode: node.Content[0].Content[1].Content[0],
		},
		{
			note:              "expected .spec.template map",
			expectedNode:      ySpecTemplateMap,
			gotNode:           node.NodeContent[0].NodeContent[1].NodeContent[1].Node,
			confirmConfigNode: node.Content[0].Content[1].Content[1],
		},
		{
			note:              "expected .spec.template.spec scalar",
			expectedNode:      ySpecTemplateSpecScalar,
			gotNode:           node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[0].Node,
			confirmConfigNode: node.Content[0].Content[1].Content[1].Content[0],
		},
		{
			note:              "expected .spec.template.spec map",
			expectedNode:      ySpecTemplateSpecMap,
			gotNode:           node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].Node,
			confirmConfigNode: node.Content[0].Content[1].Content[1].Content[1],
		},
		{
			note:              "expected .spec.template.spec.containers scalar",
			expectedNode:      ySpecTemplateSpecContainersScalar,
			gotNode:           node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].Node,
			confirmConfigNode: node.Content[0].Content[1].Content[1].Content[1].Content[0],
		},
		{
			note:              "expected .spec.template.spec.containers sequence",
			expectedNode:      ySpecTemplateSpecContainersSequence,
			gotNode:           node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].Node,
			confirmConfigNode: node.Content[0].Content[1].Content[1].Content[1].Content[1],
		},
		{
			note:              "expected .spec.template.spec.containers[0] map",
			expectedNode:      ySpecTemplateSpecContainersZeroMap,
			gotNode:           node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].Node,
			confirmConfigNode: node.Content[0].Content[1].Content[1].Content[1].Content[1].Content[0],
		},
		{
			note:              "expected .spec.template.spec.containers[0].name scalar",
			expectedNode:      ySpecTemplateSpecContainersZeroNameScalar,
			gotNode:           node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[0].Node,
			confirmConfigNode: node.Content[0].Content[1].Content[1].Content[1].Content[1].Content[0].Content[0],
		},
		{
			note:              "expected .spec.template.spec.containers[0].name value scalar",
			expectedNode:      ySpecTemplateSpecContainersZeroNameValueScalar,
			gotNode:           node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[1].Node,
			confirmConfigNode: node.Content[0].Content[1].Content[1].Content[1].Content[1].Content[0].Content[1],
		},

		{
			note:              "expected .spec.template.spec.containers[0].command scalar",
			expectedNode:      ySpecTemplateSpecContainersZeroCommandScalar,
			gotNode:           node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[2].Node,
			confirmConfigNode: node.Content[0].Content[1].Content[1].Content[1].Content[1].Content[0].Content[2],
		},

		{
			note:              "expected .spec.template.spec.containers[0].command sequence",
			expectedNode:      ySpecTemplateSpecContainersZeroCommandSequence,
			gotNode:           node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[3].Node,
			confirmConfigNode: node.Content[0].Content[1].Content[1].Content[1].Content[1].Content[0].Content[3],
		},
	}

	for _, tc := range testCases {
		//   confirm we didn't mess up the tests
		if tc.confirmConfigNode != tc.expectedNode {
			t.Errorf("Description: %s: main.WalkConvertYamlNodeToMainNode(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.expectedNode, tc.confirmConfigNode)
		}
		if tc.gotNode != tc.expectedNode {
			t.Errorf("Description: %s: main.WalkConvertYamlNodeToMainNode(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.expectedNode, tc.gotNode)
		}
	}

}

func TestGetFileConfigs(t *testing.T) {
	type testCase struct {
		note                   string
		yaml                   string
		expectedKind           string
		expectedIgnore         bool
		expectedIgnoreRequired bool
	}

	testCases := []testCase{
		{
			note: "header comment kind generic",
			yaml: `---
# predictable-yaml: kind=generic,ignore-requireds
kind: Deployment
spec:
  asdf: fdsa`,
			expectedKind:           "generic",
			expectedIgnoreRequired: true,
			expectedIgnore:         false,
		},
		{
			note: "line comment kind generic",
			yaml: `---
kind: Deployment  # predictable-yaml: kind=generic, ignore
spec:
  asdf: fdsa`,
			expectedKind:           "generic",
			expectedIgnoreRequired: false,
			expectedIgnore:         true,
		},
		{
			note: "footer comment kind generic",
			yaml: `---
kind: Deployment
# predictable-yaml: kind=generic
spec:
  asdf: fdsa`,
			expectedKind:           "generic",
			expectedIgnoreRequired: false,
			expectedIgnore:         false,
		},
		{
			note: "regular kind deployment",
			yaml: `---
kind: Deployment
spec:
  asdf: fdsa`,
			expectedKind:           "Deployment",
			expectedIgnoreRequired: false,
			expectedIgnore:         false,
		},
		{
			note: "multiline comments 1: with colon",
			yaml: `---
# TODO: comment
# this comment
# this comment
# predictable-yaml: ignore-requireds
apiVersion: TODO
kind: Deployment
spec:
  asdf: fdsa`,
			expectedKind:           "Deployment",
			expectedIgnoreRequired: true,
			expectedIgnore:         false,
		},
		{
			note: "multiline comments 2",
			yaml: `---
# predictable-yaml: ignore-requireds
# this comment
# this comment
apiVersion: TODO
kind: Deployment
spec:
  asdf: fdsa`,
			expectedKind:           "Deployment",
			expectedIgnoreRequired: true,
			expectedIgnore:         false,
		},
	}

	for _, tc := range testCases {
		// convert yaml
		n := &yaml.Node{}
		err := yaml.Unmarshal([]byte(tc.yaml), n)
		if err != nil {
			t.Fatalf("Description: main.GetFileConfigs(...): failed unmarshaling config test data!")
		}
		node := &Node{Node: n}
		WalkConvertYamlNodeToMainNode(node)

		// do it
		got := GetFileConfigs(node)
		if got.Kind != tc.expectedKind {
			t.Errorf("Description: %s: main.GetFileConfigs(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, tc.expectedKind, got)
		}
		if got.Ignore != tc.expectedIgnore {
			t.Errorf("Description: %s: main.GetFileConfigs(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, tc.expectedIgnoreRequired, got.Ignore)
		}
		if got.IgnoreRequireds != tc.expectedIgnoreRequired {
			t.Errorf("Description: %s: main.GetFileConfigs(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, tc.expectedIgnoreRequired, got.IgnoreRequireds)
		}
	}
}

func TestWalkToNodeForPath(t *testing.T) {
	type testCase struct {
		path         string
		expectedNode *Node
	}

	testYamlContainers := `---
spec:
  template:
    spec:
      containers:
      - name: cool-app
        command:
        - asdf
        - fdsa
      - name: uncool-app
        command:
        - asdf`

	// convert yaml
	n := &yaml.Node{}
	err := yaml.Unmarshal([]byte(testYamlContainers), n)
	if err != nil {
		t.Fatalf("Description: main.walkToNodeForPath(...): failed unmarshaling config test data!")
	}
	node := &Node{Node: n}
	WalkConvertYamlNodeToMainNode(node)

	testCases := []testCase{
		{
			path:         ".",
			expectedNode: node.NodeContent[0],
		},
		{
			path:         ".spec",
			expectedNode: node.NodeContent[0].NodeContent[0],
		},
		{
			path:         ".spec.template.spec",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[0],
		},
		{
			path:         ".spec.template.spec.",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1],
		},
		{
			path:         ".spec.template.spec.containers",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0],
		},
		{
			path:         ".spec.template.spec.containers[0]",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0],
		},
		{
			path:         ".spec.template.spec.containers[0].name",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[0],
		},
		{
			path:         ".spec.template.spec.containers[0].command",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[2],
		},
		{
			path:         ".spec.template.spec.containers[0].command[1]",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[3].NodeContent[1],
		},
	}

	for _, tc := range testCases {
		// do it
		gotNode, err := walkToNodeForPath(node, tc.path, 0)
		if err != nil {
			t.Errorf("Description: %s: main.walkToNodeForPath(...): \n-expected:\nno error\n+got:\n%#v\n", tc.path, err)
			continue
		}
		if gotNode != tc.expectedNode {
			t.Errorf("Description: %s: main.walkToNodeForPath(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.path, tc.expectedNode.Node, gotNode.Node)
		}
	}
}

func TestWalkParseLoadConfigComments(t *testing.T) {
	type testCase struct {
		note      string
		yaml      string
		path      string
		first     bool
		required  bool
		preferred bool
		ditto     string
	}

	// note: some of these comments might not make sense for a Kubernetes
	//   deployment but are here for testing.
	testYamlContainers := `---
spec:  # first
  template:  # required
    spec:  # first, required
      initContainers: []  # preferred, ditto=.spec.template.spec.containers
      containers:  # required
      - name: cool-app  # first, required
        command:  # preferred
        - asdf
        args:
        - asdf
`

	testCases := []testCase{
		{
			note:      ".spec: first",
			yaml:      testYamlContainers,
			path:      ".spec",
			first:     true,
			required:  false,
			preferred: false,
			ditto:     "",
		},
		{
			note:      ".spec.template: required",
			yaml:      testYamlContainers,
			path:      ".spec.template",
			first:     false,
			required:  true,
			preferred: false,
			ditto:     "",
		},
		{
			note:      ".spec.template.spec: first, required",
			yaml:      testYamlContainers,
			path:      ".spec.template.spec",
			first:     true,
			required:  true,
			preferred: false,
			ditto:     "",
		},
		{
			note:      ".spec.template.spec.initContainers: ditto=.spec.template.spec.containers",
			yaml:      testYamlContainers,
			path:      ".spec.template.spec.initContainers",
			first:     false,
			required:  false,
			preferred: true,
			ditto:     ".spec.template.spec.containers",
		},
		{
			note:      ".spec.template.spec.containers: (defaults)",
			yaml:      testYamlContainers,
			path:      ".spec.template.spec.containers",
			first:     false,
			required:  true,
			preferred: false,
			ditto:     "",
		},
		{
			note:      ".spec.template.spec.containers[0].command: preferred",
			yaml:      testYamlContainers,
			path:      ".spec.template.spec.containers[0].command",
			first:     false,
			required:  false,
			preferred: true,
			ditto:     "",
		},
	}

	for _, tc := range testCases {
		// convert yaml
		n := &yaml.Node{}
		err := yaml.Unmarshal([]byte(tc.yaml), n)
		if err != nil {
			t.Errorf("Description: %s: main.WalkParseLoadConfigComments(...): failed unmarshaling test data!\n\t%v", tc.note, err)
			continue
		}

		node := &Node{Node: n}
		WalkConvertYamlNodeToMainNode(node)
		WalkParseLoadConfigComments(node)

		// find node to test via path
		childNode, err := walkToNodeForPath(node, tc.path, 0)
		if err != nil {
			t.Errorf("Description: %s: main.WalkParseLoadConfigComments(...): expected: no error, got: %#v", tc.note, err)
			continue
		}
		if childNode == nil {
			t.Errorf("Description: %s: main.WalkParseLoadConfigComments(...): failed finding test data node path!", tc.note)
			continue
		}

		// do it
		if childNode.MustBeFirst != tc.first {
			t.Errorf("Description: %s: main.WalkParseLoadConfigComments(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.first, childNode.MustBeFirst)
		}
		if childNode.Required != tc.required {
			t.Errorf("Description: %s: main.WalkParseLoadConfigComments(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.required, childNode.Required)
		}
		if childNode.Preferred != tc.preferred {
			t.Errorf("Description: %s: main.WalkParseLoadConfigComments(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.preferred, childNode.Preferred)
		}
		if childNode.Ditto != tc.ditto {
			t.Errorf("Description: %s: main.WalkParseLoadConfigComments(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.ditto, childNode.Ditto)
		}
	}
}

func TestGetReferencePath(t *testing.T) {
	type testCase struct {
		note     string
		expected string
		yaml     string
		path     string
	}

	testYamlContainers := `---
spec:
  template:
    spec:
      containers:
      - name: cool-app
        command:
        - asdf
      - name: uncool-app
        command:
        - qwer
        - trew
        - fghj
        args:
        - fdsa`
	testYamlother := `---
    my-config:  asdf
    a-config: fdsa
    a-list:
    - qwer
    - rewq
    a-list-of-lists:
    -
      - qwer
      - rewq`

	testCases := []testCase{
		{
			note:     ".spec",
			path:     ".spec",
			expected: ".spec",
			yaml:     testYamlContainers,
		},
		{
			note:     ".spec.template",
			path:     ".spec.template",
			expected: ".spec.template",
			yaml:     testYamlContainers,
		},
		{
			note:     ".spec.template.spec",
			path:     ".spec.template.spec",
			expected: ".spec.template.spec",
			yaml:     testYamlContainers,
		},
		{
			note:     ".spec.template.spec.containers[0].name",
			path:     ".spec.template.spec.containers[0].name",
			expected: ".spec.template.spec.containers[0].name",
			yaml:     testYamlContainers,
		},
		{
			note:     ".spec.template.spec.containers[1].command[2]",
			path:     ".spec.template.spec.containers[1].command[2]",
			expected: ".spec.template.spec.containers[1].command[2]",
			yaml:     testYamlContainers,
		},
		{
			note:     ".my-config",
			path:     ".my-config",
			expected: ".my-config",
			yaml:     testYamlother,
		},
		// {
		//     note:     ".a-list-of-lists[0][1]",
		//     path:     ".a-list-of-lists[0][1]",
		//     expected: ".a-list-of-lists[0][1]",
		//     yaml:     testYamlother,
		// },
	}

	for _, tc := range testCases {
		// convert yaml
		n := &yaml.Node{}
		err := yaml.Unmarshal([]byte(tc.yaml), n)
		if err != nil {
			t.Fatalf("Description: %s: main.getReferencePath(...): failed unmarshaling test data!", tc.note)
		}
		node := &Node{Node: n}
		WalkConvertYamlNodeToMainNode(node)

		// find node to test via path
		childNode, err := walkToNodeForPath(node, tc.path, 0)
		if err != nil {
			t.Fatalf("Description: %s: main.getReferencePath(...): expected: no error, got: %#v", tc.note, err)
		}
		if childNode == nil {
			t.Fatalf("Description: %s: main.getReferencePath(...): failed finding test data node path!", tc.note)
		}

		// do it
		got := getReferencePath(childNode, 0, "")
		if got != tc.expected {
			t.Errorf("Description: %s: main.getReferencePath(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.expected, got)
		}
	}
}

func TestWalkAndCompare(t *testing.T) {
	type testCase struct {
		note         string
		expectedErrs ValidationErrors
		configYamls  []string
		fileYaml     string
	}

	testCases := []testCase{
		{
			note:         "all in order",
			expectedErrs: ValidationErrors{},
			configYamls: []string{
				`---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      initContainers: []  # ditto=.spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`},
			fileYaml: `---
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: cool-app
        command:
        - asdf
      - name: uncool-app
        command:
        - asdf`,
		},
		{
			note: "firsts 1",
			expectedErrs: ValidationErrors{
				fmt.Errorf("validation error: want '.spec.template.spec.containers[0].name' to be first, got '.spec.template.spec.containers[0].command'"),
				fmt.Errorf("validation error: want '.spec.template.spec.containers[1].name' to be first, got '.spec.template.spec.containers[1].command'"),
			},
			configYamls: []string{`---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      initContainers: []  # ditto=.spec.template.spec.containers
      containers:
      - name: cool-app  # first, required`},
			fileYaml: `---
kind: Deployment
spec:
  template:
    spec:
      containers:
      - command:
        - asdf
        name: cool-app
      - command:
        - asdf
        name: uncool-app`,
		},
		{
			note: "requireds 1",
			expectedErrs: ValidationErrors{
				fmt.Errorf("validation error: missing required key '.spec.template.spec'"),
			},
			configYamls: []string{`---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      initContainers: []  # ditto=.spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`},
			fileYaml: `---
kind: Deployment
spec:
  template:
    asdrf:
    - fdsa`,
		},
		{
			note: "requireds 2",
			expectedErrs: ValidationErrors{
				fmt.Errorf("validation error: missing required key '.spec.template.spec.containers[0].name'"),
			},
			configYamls: []string{`---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      initContainers: []  # ditto=.spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`},
			fileYaml: `---
kind: Deployment
spec:
  template:
    spec:
      containers:
      - command:
        - asdf`,
		},
		{
			note: "afters 1, comments",
			expectedErrs: ValidationErrors{
				fmt.Errorf("validation error: want '.spec.template.spec.containers' to be after '.spec.template.spec.initContainers', is before"),
				fmt.Errorf("validation error: want '.spec.template.spec.containers[0].args' to be after '.spec.template.spec.containers[0].command', is before"),
			},
			configYamls: []string{`---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      initContainers: []  # ditto=.spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`},
			fileYaml: `---
kind: Deployment
# this is that
spec:
  template:
    # asdf
    spec:
    # that comment
      containers:
      - name: uncool-app # other
        args:
        - asdf
        command:
        # this comment
        - asdf
      initContainers:
      - name: uncool-app2
        args: # that comment
        - asdf`,
		},
		{
			note: "ditto 1",
			expectedErrs: ValidationErrors{
				fmt.Errorf("validation error: want '.spec.template.spec.initContainers[0].args' to be after '.spec.template.spec.initContainers[0].name', is before"),
				fmt.Errorf("validation error: want '.spec.template.spec.initContainers[0].name' to be first, got '.spec.template.spec.initContainers[0].args'"),
				fmt.Errorf("validation error: missing required key '.spec.template.spec.initContainers[1].name'"),
			},
			configYamls: []string{`---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      initContainers: []  # ditto=.spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`},
			fileYaml: `---
kind: Deployment
spec:
  template:
    spec:
      initContainers:
      - args:
        - asdf
        name: uncool-app2
      - args:
        - fdsa
      containers:
      - name: uncool-app
        command:
        - asdf
        args:
        - asdf`,
		},
		{
			note: "ditto 2",
			expectedErrs: ValidationErrors{
				fmt.Errorf("validation error: want '.spec.template.spec.containers[0].readinessProbe.httpGet' to be after '.spec.template.spec.containers[0].readinessProbe.periodSeconds', is before"),
				fmt.Errorf("validation error: want '.spec.template.spec.containers[0].readinessProbe.periodSeconds' to be first, got '.spec.template.spec.containers[0].readinessProbe.httpGet'"),
				fmt.Errorf("validation error: want '.spec.template.spec.containers[0].readinessProbe.httpGet.path' to be after '.spec.template.spec.containers[0].readinessProbe.httpGet.port', is before"),
				fmt.Errorf("validation error: want '.spec.template.spec.containers[0].readinessProbe.httpGet.port' to be first, got '.spec.template.spec.containers[0].readinessProbe.httpGet.path'"),
			},
			configYamls: []string{`---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      containers:
      - name: cool-app  # first, required
        livenessProbe:
          periodSeconds: 10  # first, required
          httpGet:
            port: http  # first, required
            path: /
            scheme: HTTP
        readinessProbe: {}  # ditto=.spec.template.spec.containers[0].livenessProbe`},
			fileYaml: `---
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: uncool-app
        livenessProbe:
          periodSeconds: 10
          httpGet:
            port: http
            path: /
            scheme: HTTP
        readinessProbe:
          httpGet:
            path: /
            port: http
            scheme: HTTP
          periodSeconds: 10`,
		},
		{
			note: "ditto 3, different config schema",
			expectedErrs: ValidationErrors{
				fmt.Errorf("validation error: want '.spec.template.spec.containers[0].readinessProbe.httpGet' to be after '.spec.template.spec.containers[0].readinessProbe.periodSeconds', is before"),
				fmt.Errorf("validation error: want '.spec.template.spec.containers[0].readinessProbe.periodSeconds' to be first, got '.spec.template.spec.containers[0].readinessProbe.httpGet'"),
				fmt.Errorf("validation error: want '.spec.template.spec.containers[0].readinessProbe.httpGet.path' to be after '.spec.template.spec.containers[0].readinessProbe.httpGet.port', is before"),
				fmt.Errorf("validation error: want '.spec.template.spec.containers[0].readinessProbe.httpGet.port' to be first, got '.spec.template.spec.containers[0].readinessProbe.httpGet.path'"),
			},
			configYamls: []string{
				`---
kind: Pod  # first
spec:
  containers:
  - name: cool-app  # first, required
    livenessProbe:
      periodSeconds: 10  # first, required
      httpGet:
        port: http  # first, required
        path: /
        scheme: HTTP
    readinessProbe: {}  # ditto=.spec.containers[0].livenessProbe`,
				`---
kind: Deployment  # first
spec:
  template:  # required
    spec: {}  # ditto=Pod.spec`,
			},
			fileYaml: `---
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: uncool-app
        livenessProbe:
          periodSeconds: 10
          httpGet:
            port: http
            path: /
            scheme: HTTP
        readinessProbe:
          httpGet:
            path: /
            port: http
            scheme: HTTP
          periodSeconds: 10`,
		},
		{
			note:         "ditto 4: list ending in dot",
			expectedErrs: ValidationErrors{},
			configYamls: []string{
				`---
apiVersion: core/v1  # first, required
kind: DeploymentList  # required
metadata:  # required
  name: TODO  # first, required
items: []  # ditto=Deployment.`,
				`apiVersion: core/v1  # first, required
kind: Deployment # required
spec:  # required
  template:  # required
    spec:  # required
      initContainers: []  # first, ditto=.spec.template.spec.containers
      containers:  # required
      - name: cool-app  # first, required
        command:  # preferred
        - asdf
        args:  # preferred
        - asdf`},
			fileYaml: `---
apiVersion: core/v1
kind: DeploymentList
metadata:
  name: TODO
items:
- apiVersion: core/v1
  kind: Deployment
  spec:
    template:
      spec:
        containers:
        - name: cool-app
          command:
          - asdf
        - name: uncool-app
          command:
          - asdf`,
		},
		{
			note: "ditto 5: list ending in dot, errors",
			expectedErrs: ValidationErrors{
				fmt.Errorf("validation error: want '.items[0].spec.template.spec.containers[0].command' to be after '.items[0].spec.template.spec.containers[0].name', is before"),
				fmt.Errorf("validation error: want '.items[0].spec.template.spec.containers[0].name' to be first, got '.items[0].spec.template.spec.containers[0].command'"),
				fmt.Errorf("validation error: want '.items[1].spec.template.spec.initContainers[1].command' to be after '.items[1].spec.template.spec.initContainers[1].name', is before"),
				fmt.Errorf("validation error: want '.items[1].spec.template.spec.initContainers[1].name' to be first, got '.items[1].spec.template.spec.initContainers[1].command'"),
				fmt.Errorf("validation error: want '.items[1].spec.template.spec.containers[1].command' to be after '.items[1].spec.template.spec.containers[1].name', is before"),
				fmt.Errorf("validation error: want '.items[1].spec.template.spec.containers[1].name' to be first, got '.items[1].spec.template.spec.containers[1].command'"),
			},
			configYamls: []string{
				`---
apiVersion: core/v1  # first, required
kind: DeploymentList  # required
metadata:  # required
  name: TODO  # first, required
items: []  # ditto=Deployment.`,
				`apiVersion: core/v1  # first, required
kind: Deployment # required
spec:  # required
  template:  # required
    spec:  # required
      initContainers: []  # first, ditto=.spec.template.spec.containers
      containers:  # required
      - name: cool-app  # first, required
        command:  # preferred
        - asdf
        args:  # preferred
        - asdf`},
			fileYaml: `---
apiVersion: core/v1
kind: DeploymentList
metadata:
  name: example
items:
- apiVersion: core/v1
  kind: Deployment
  spec:
    template:
      spec:
        containers:
        - command:
          - asdf
          name: cool-app
          asdf: fdsa
        - name: uncool-app
          command:
          - asdf
- apiVersion: core/v1
  kind: Deployment
  spec:
    template:
      spec:
        initContainers:
        - name: cool-app
          command:
          - asdf
        - command:
          - asdf
          name: uncool-app
          grewq: fdsa
        containers:
        - name: cool-app
          command:
          - asdf
        - command:
          - asdf
          name: uncool-app
          grewq: fdsa`,
		},
		{
			note: "ditto 6: other ending in dot",
			expectedErrs: ValidationErrors{
				fmt.Errorf("validation error: missing required key '.spec.template.spec'"),
				fmt.Errorf("validation error: want '.spec.template.testThis[0].containers[0].command' to be after '.spec.template.testThis[0].containers[0].name', is before"),
				fmt.Errorf("validation error: want '.spec.template.testThis[0].containers[0].name' to be first, got '.spec.template.testThis[0].containers[0].command'"),
			},
			configYamls: []string{
				`apiVersion: core/v1  # first, required
kind: Deployment # required
spec:  # required
  template:  # required
    testThis: []  # ditto=.spec.template.spec.
    spec:  # required
      initContainers: []  # first, ditto=.spec.template.spec.containers
      containers:  # required
      - name: cool-app  # first, required
        command:  # preferred
        - asdf
        args:  # preferred
        - asdf`},
			fileYaml: `---
apiVersion: core/v1
kind: Deployment
spec:
  template:
    testThis:
    - containers:
      - command:
        - asdf
        name: cool-app
        asdf: fdsa
      - name: uncool-app
        command:
        - asdf`,
		},
		{
			note:         "overrides 1",
			expectedErrs: ValidationErrors{},
			configYamls: []string{`---
# predictable-yaml: kind=Deployment
spec: # first
  template:  # required
    spec:  # first, required
      initContainers: []  # ditto=.spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`},
			fileYaml: `---
# predictable-yaml: ignore-requireds, kind=Deployment
spec:
  template: {}`,
		},
		{
			note:         "overrides ignore",
			expectedErrs: ValidationErrors{},
			configYamls: []string{`---
# predictable-yaml: kind=Deployment
spec: # first
  template:  # required
    spec:  # first, required
      initContainers: []  # ditto=.spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`},
			fileYaml: `---
# predictable-yaml: ignore
spec:
  template: {}`,
		},
		{
			note:         "first doesn't count if not required and non existent",
			expectedErrs: ValidationErrors{},
			configYamls: []string{`---
kind: Deployment  # first
spec:
  template:  # required
    spec:  # first
      initContainers: []  # ditto=.spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`},
			fileYaml: `---
kind: Deployment  # first
spec:
  template:
    asdf: []`,
		},
	}

	for _, tc := range testCases {
		// convert yaml
		configMap := ConfigMap{}
		for _, cYaml := range tc.configYamls {
			cN := &yaml.Node{}
			err := yaml.Unmarshal([]byte(cYaml), cN)
			if err != nil {
				t.Errorf("Description: %s: main.WalkAndCompare(...): failed unmarshaling config test data!", tc.note)
				continue
			}
			configNode := &Node{Node: cN}
			WalkConvertYamlNodeToMainNode(configNode)
			WalkParseLoadConfigComments(configNode)
			fileConfigs := GetFileConfigs(configNode)
			if fileConfigs.Kind == "" {
				t.Errorf("Description: %s: main.WalkAndCompare(...): failed getting kind for config test data!", tc.note)
			}
			configMap[fileConfigs.Kind] = configNode
		}

		fN := &yaml.Node{}
		err := yaml.Unmarshal([]byte(tc.fileYaml), fN)
		if err != nil {
			t.Errorf("Description: %s: main.WalkAndCompare(...): failed unmarshaling file test data: %v", tc.note, err)
			continue
		}
		fileNode := &Node{Node: fN}
		WalkConvertYamlNodeToMainNode(fileNode)
		fileConfigs := GetFileConfigs(fileNode)
		if fileConfigs.Ignore {
			continue
		}

		// do it
		sortConfigs := SortConfigs{
			ConfigMap:   configMap,
			FileConfigs: fileConfigs,
		}
		gotErrs := WalkAndCompare(configMap[fileConfigs.Kind], fileNode, sortConfigs, ValidationErrors{})
		expected := GetValidationErrorStrings(tc.expectedErrs)
		got := GetValidationErrorStrings(gotErrs)
		if got != expected {
			t.Errorf("Description: %s: main.WalkAndCompare(...): \n-expected:\n%v\n+got:\n%v\n", tc.note, expected, got)
			continue
		}
	}
}

func TestWalkAndSort(t *testing.T) {
	type testCase struct {
		note          string
		toBeginning   bool
		addPreferreds bool
		expectedErrs  ValidationErrors
		configYamls   []string
		fileYaml      string
		expectedYaml  string
	}

	testCases := []testCase{
		{
			note:         "all in order",
			toBeginning:  false,
			expectedErrs: ValidationErrors{},
			configYamls: []string{
				`---
apiVersion: apps/v1  # first, required
kind: Deployment  # required
metadata:  # required
  name: example  # first, required
  namespace: example
  labels:  # required
    app: example  # first, required
spec:  # required
  template:  # required
    metadata:
      labels: {}  # ditto=.metadata.labels
    spec:  # first, required
      initContainers: []  # ditto=.spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        image: example
        command:
        - asdf
        args:
        - asdf
        livenessProbe:
          periodSeconds: 10  # first, required
          httpGet:
            port: http  # first, required
            path: /
            scheme: HTTP
        readinessProbe: {}  # ditto=.spec.template.spec.containers[0].livenessProbe`},
			fileYaml: `---
# predictable-yaml: ignore-requireds
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: example
  # comment about name
  name: example
  labels:
    asdf: fdsa
    # comment about asdf

    app: example
spec:
  template:
    someOtherThing:
      asdf: fdsa
    spec:
      containers:
      - command: # comment about command
        - asdf
        image: example
        name: cool-app
      - name: uncool-app
        command:
        - asdf
      initContainers:
      - command:
        - asdf
        image: example
        name: cool-app
        livenessProbe:
          periodSeconds: 10
          httpGet:
            port: http
            scheme: HTTP
            path: /
        readinessProbe:
          httpGet:
            scheme: HTTP
            path: /
            port: http
          periodSeconds: 10
    metadata:
      labels:
        asdf: example
        app: example`,
			expectedYaml: `# predictable-yaml: ignore-requireds
apiVersion: apps/v1
kind: Deployment
metadata:
  # comment about name
  name: example
  namespace: example
  labels:
    app: example
    asdf: fdsa
    # comment about asdf
spec:
  template:
    metadata:
      labels:
        app: example
        asdf: example
    spec:
      initContainers:
        - name: cool-app
          image: example
          command:
            - asdf
          livenessProbe:
            periodSeconds: 10
            httpGet:
              port: http
              path: /
              scheme: HTTP
          readinessProbe:
            periodSeconds: 10
            httpGet:
              port: http
              path: /
              scheme: HTTP
      containers:
        - name: cool-app
          image: example
          command: # comment about command
            - asdf
        - name: uncool-app
          command:
            - asdf
    someOtherThing:
      asdf: fdsa
`,
		},
		{
			note:         "ditto from another kind",
			toBeginning:  false,
			expectedErrs: ValidationErrors{},
			configYamls: []string{
				`---
kind: Pod  # first
spec:
  initContainers: []  # ditto=.spec.containers
  containers:
  - name: cool-app  # first, required
    image: example
    command:
    - asdf
    args:
    - asdf
    livenessProbe:
      periodSeconds: 10  # first, required
      httpGet:
        port: http  # first, required
        path: /
        scheme: HTTP
    readinessProbe: {}  # ditto=.spec.containers[0].livenessProbe`,
				`---
apiVersion: apps/v1  # first, required
kind: Deployment  # required
metadata:  # required
  name: example  # first, required
  namespace: example
  labels:  # required
    app: example  # first, required
spec:  # required
  template:  # required
    metadata:
      labels: {}  # ditto=.metadata.labels
    spec: {}  # ditto=Pod.spec`},
			fileYaml: `---
# predictable-yaml: ignore-requireds
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: example
  # comment about name
  name: example
  labels:
    asdf: fdsa
    # comment about asdf

    app: example
spec:
  template:
    someOtherThing:
      asdf: fdsa
    spec:
      containers:
      - command: # comment about command
        - asdf
        image: example
        name: cool-app
      - name: uncool-app
        command:
        - asdf
      initContainers:
      - command:
        - asdf
        image: example
        name: cool-app
        livenessProbe:
          periodSeconds: 10
          httpGet:
            port: http
            scheme: HTTP
            path: /
        readinessProbe:
          httpGet:
            scheme: HTTP
            path: /
            port: http
          periodSeconds: 10
    metadata:
      labels:
        asdf: example
        app: example`,
			expectedYaml: `# predictable-yaml: ignore-requireds
apiVersion: apps/v1
kind: Deployment
metadata:
  # comment about name
  name: example
  namespace: example
  labels:
    app: example
    asdf: fdsa
    # comment about asdf
spec:
  template:
    metadata:
      labels:
        app: example
        asdf: example
    spec:
      initContainers:
        - name: cool-app
          image: example
          command:
            - asdf
          livenessProbe:
            periodSeconds: 10
            httpGet:
              port: http
              path: /
              scheme: HTTP
          readinessProbe:
            periodSeconds: 10
            httpGet:
              port: http
              path: /
              scheme: HTTP
      containers:
        - name: cool-app
          image: example
          command: # comment about command
            - asdf
        - name: uncool-app
          command:
            - asdf
    someOtherThing:
      asdf: fdsa
`,
		},
		{
			note:         "ditto: other ending in dot",
			expectedErrs: ValidationErrors{},
			configYamls: []string{
				`apiVersion: core/v1  # first, required
kind: Asdf # required
spec:  # required
  template:  # required
    testThis: []  # ditto=.spec.template.spec.
    spec:  # required
      initContainers: []  # first, ditto=.spec.template.spec.containers
      containers:  # required
      - name: cool-app  # first, required
        command:  # preferred
        - asdf
        args:  # preferred
        - asdf`},
			fileYaml: `# predictable-yaml: ignore-requireds
apiVersion: core/v1
kind: Asdf
spec:
  template:
    testThis:
    - containers:
      - command:
        - asdf
        name: cool-app
        asdf: fdsa
      - name: uncool-app
        command:
        - asdf`,
			expectedYaml: `# predictable-yaml: ignore-requireds
apiVersion: core/v1
kind: Asdf
spec:
  template:
    testThis:
      - containers:
          - name: cool-app
            command:
              - asdf
            asdf: fdsa
          - name: uncool-app
            command:
              - asdf
`,
		},
		{
			note:         "missing required 1",
			toBeginning:  false,
			expectedErrs: ValidationErrors{},
			configYamls: []string{
				`---
apiVersion: apps/v1  # first, required
kind: Deployment  # required
metadata:  # required
  name: example  # first, required
  namespace: example
  labels:  # required
    app: example  # first, required
spec:  # required
  template: {}  # required`},
			fileYaml: `---
kind: Deployment
apiVersion: apps/v1
metadata:
  namespace: example
  name: example
  labels:
    app: example
spec: {}`,
			expectedYaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
  namespace: example
  labels:
    app: example
spec:
  template: {}
`,
		},
		{
			note:          "missing required 2: include preferreds",
			toBeginning:   true,
			expectedErrs:  ValidationErrors{},
			addPreferreds: true,
			configYamls: []string{
				`---
kind: Pod  # first
spec:  # required
  initContainers: []  # required, ditto=.spec.containers
  containers:  # required
  - name: TODO  # first, required
    image: TODO  # required
    command:  # preferred
    - TODO
    args: []  # preferred
    livenessProbe:
      periodSeconds: 10  # first, required
      httpGet:
        port: http  # first, required
        path: /
        scheme: HTTP
    readinessProbe: {}  # ditto=.spec.containers[0].livenessProbe`,
				`---
apiVersion: apps/v1  # first, required
kind: Deployment  # required
metadata:  # required
  name: TODO  # first, required
  namespace: TODO  # preferred
  labels:  # required
    app: TODO  # first, required
spec:  # required
  template:  # required
    metadata:
      labels: {}  # ditto=.metadata.labels
    spec: {}  # required, ditto=Pod.spec`},
			fileYaml: `---
kind: Deployment
spec: {}`,
			expectedYaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: TODO
  namespace: TODO
  labels:
    app: TODO
spec:
  template:
    spec:
      initContainers:
        - name: TODO
          image: TODO
          command:
            - TODO
          args: []
      containers:
        - name: TODO
          image: TODO
          command:
            - TODO
          args: []
`,
		},
		{
			note:          "missing required 3: addPreferreds",
			toBeginning:   true,
			addPreferreds: true,
			expectedErrs:  ValidationErrors{},
			configYamls: []string{
				`---
kind: Pod  # first
spec:  # required
  initContainers: []  # ditto=.spec.containers
  containers:  # required
  - name: TODO  # first, required
    image: TODO  # required
    command: []  # preferred
    args: []  # preferred
    livenessProbe:
      periodSeconds: 10  # first, required
      httpGet:
        port: http  # first, required
        path: /
        scheme: HTTP
    readinessProbe: {}  # ditto=.spec.containers[0].livenessProbe`,
				`---
apiVersion: apps/v1  # first, required
kind: Deployment  # required
metadata:  # required
  name: example  # first, required
  namespace: example
  labels:  # required
    app: example  # first, required
spec:  # required
  template:  # required
    metadata:
      labels: {}  # ditto=.metadata.labels
    spec: {}  # required, ditto=Pod.spec`},
			fileYaml: `---
kind: Deployment
apiVersion: apps/v1
metadata:
  name: example
  labels:
    app: example
spec: {}`,
			expectedYaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
  labels:
    app: example
spec:
  template:
    spec:
      containers:
        - name: TODO
          image: TODO
          command: []
          args: []
`,
		},
	}

	for _, tc := range testCases {
		// convert yaml
		configMap := ConfigMap{}
		for _, cYaml := range tc.configYamls {
			cN := &yaml.Node{}
			err := yaml.Unmarshal([]byte(cYaml), cN)
			if err != nil {
				t.Errorf("Description: %s: main.WalkAndSort(...): failed unmarshaling config test data!", tc.note)
				continue
			}
			configNode := &Node{Node: cN}
			WalkConvertYamlNodeToMainNode(configNode)
			WalkParseLoadConfigComments(configNode)
			fileConfigs := GetFileConfigs(configNode)
			if fileConfigs.Kind == "" {
				t.Fatalf("Description: %s: main.WalkAndSort(...): failed getting kind for config test data!", tc.note)
				continue
			}
			configMap[fileConfigs.Kind] = configNode
		}

		fN := &yaml.Node{}
		err := yaml.Unmarshal([]byte(tc.fileYaml), fN)
		if err != nil {
			t.Errorf("Description: %s: main.WalkAndSort(...): failed unmarshaling file test data: %v", tc.note, err)
			continue
		}
		fileNode := &Node{Node: fN}
		WalkConvertYamlNodeToMainNode(fileNode)
		fileConfigs := GetFileConfigs(fileNode)
		if fileConfigs.Ignore {
			continue
		}

		// do it
		sortConfs := SortConfigs{configMap, fileConfigs, tc.toBeginning, tc.addPreferreds}
		gotErrs := WalkAndSort(configMap[fileConfigs.Kind], fileNode, sortConfs, ValidationErrors{})
		expected := GetValidationErrorStrings(tc.expectedErrs)
		got := GetValidationErrorStrings(gotErrs)
		switch {
		case len(gotErrs) != 0 && len(tc.expectedErrs) == 0:
			t.Errorf("Description: %s: main.WalkAndSort(...): \n-expected:\n%v\n+got:\n%v\n", tc.note, expected, got)
			continue
		case len(gotErrs) != 0 && len(tc.expectedErrs) != 0:
			t.Errorf("Description: %s: main.WalkAndSort(...): \n-expected:\n%v\n+got:\n%v\n", tc.note, expected, got)
			continue
		case len(gotErrs) == 0 && len(tc.expectedErrs) != 0:
			t.Errorf("Description: %s: main.WalkAndSort(...): \n-expected:\n%v\n+got:\n%v\n", tc.note, expected, got)
			continue
		}
		var buf bytes.Buffer
		encoder := yaml.NewEncoder(&buf)
		encoder.SetIndent(2)
		err = encoder.Encode(fileNode.Node)
		if err != nil {
			t.Errorf("Description: %s: main.WalkAndSort(...): \n-expected:\nno error\n+got:\n%v\n", tc.note, err)
			continue
		}
		if buf.String() != tc.expectedYaml {
			t.Errorf("Description: %s: main.WalkAndSort(...): \n-expected:\n%v\n+got:\n%v\n", tc.note, tc.expectedYaml, buf.String())
			continue
		}
		// if buf.String() != tc.expectedYaml {
		//     dif := diff.Diff(tc.expectedYaml, buf.String())
		//     t.Errorf("Description: %s: main.WalkAndSort(...): got diff:\n%s\n", tc.note, dif)
		//     continue
		// }
	}
}
