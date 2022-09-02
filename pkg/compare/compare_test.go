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
	"strings"
	"testing"

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
		note                         string
		expectedNode                 *yaml.Node
		gotNode                      *yaml.Node
		confirmConfigNode            *yaml.Node
		expectedPreviousLineNode     *yaml.Node
		gotPreviousLineNode          *Node
		expectedFollowingContentNode *yaml.Node
		gotFollowingContentNode      *Node
	}
	testCases := []testCase{
		{
			note:                         "expected root map",
			expectedNode:                 yRootMap,
			gotNode:                      node.NodeContent[0].Node,
			confirmConfigNode:            node.Content[0],
			expectedFollowingContentNode: nil,
			gotFollowingContentNode:      nil,
		},
		{
			note:                         "expected .spec scalar",
			expectedNode:                 ySpecScalar,
			gotNode:                      node.NodeContent[0].NodeContent[0].Node,
			confirmConfigNode:            node.Content[0].Content[0],
			expectedFollowingContentNode: ySpecMap,
			gotFollowingContentNode:      node.NodeContent[0].NodeContent[0].FollowingContentNode,
		},
		{
			note:                         "expected .spec map",
			expectedNode:                 ySpecMap,
			gotNode:                      node.NodeContent[0].NodeContent[1].Node,
			confirmConfigNode:            node.Content[0].Content[1],
			expectedFollowingContentNode: nil,
			gotFollowingContentNode:      nil,
		},
		{
			note:                         "expected .spec.template scalar",
			expectedNode:                 ySpecTemplateScalar,
			gotNode:                      node.NodeContent[0].NodeContent[1].NodeContent[0].Node,
			confirmConfigNode:            node.Content[0].Content[1].Content[0],
			expectedFollowingContentNode: ySpecTemplateMap,
			gotFollowingContentNode:      node.NodeContent[0].NodeContent[1].NodeContent[0].FollowingContentNode,
		},
		{
			note:                         "expected .spec.template map",
			expectedNode:                 ySpecTemplateMap,
			gotNode:                      node.NodeContent[0].NodeContent[1].NodeContent[1].Node,
			confirmConfigNode:            node.Content[0].Content[1].Content[1],
			expectedFollowingContentNode: nil,
			gotFollowingContentNode:      nil,
		},
		{
			note:                         "expected .spec.template.spec scalar",
			expectedNode:                 ySpecTemplateSpecScalar,
			gotNode:                      node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[0].Node,
			confirmConfigNode:            node.Content[0].Content[1].Content[1].Content[0],
			expectedFollowingContentNode: ySpecTemplateSpecMap,
			gotFollowingContentNode:      node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[0].FollowingContentNode,
		},
		{
			note:                         "expected .spec.template.spec map",
			expectedNode:                 ySpecTemplateSpecMap,
			gotNode:                      node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].Node,
			confirmConfigNode:            node.Content[0].Content[1].Content[1].Content[1],
			expectedFollowingContentNode: nil,
			gotFollowingContentNode:      nil,
		},
		{
			note:                         "expected .spec.template.spec.containers scalar",
			expectedNode:                 ySpecTemplateSpecContainersScalar,
			gotNode:                      node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].Node,
			confirmConfigNode:            node.Content[0].Content[1].Content[1].Content[1].Content[0],
			expectedFollowingContentNode: ySpecTemplateSpecContainersSequence,
			gotFollowingContentNode:      node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].FollowingContentNode,
		},
		{
			note:                         "expected .spec.template.spec.containers sequence",
			expectedNode:                 ySpecTemplateSpecContainersSequence,
			gotNode:                      node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].Node,
			confirmConfigNode:            node.Content[0].Content[1].Content[1].Content[1].Content[1],
			expectedFollowingContentNode: nil,
			gotFollowingContentNode:      nil,
		},
		{
			note:                         "expected .spec.template.spec.containers[0] map",
			expectedNode:                 ySpecTemplateSpecContainersZeroMap,
			gotNode:                      node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].Node,
			confirmConfigNode:            node.Content[0].Content[1].Content[1].Content[1].Content[1].Content[0],
			expectedFollowingContentNode: nil,
			gotFollowingContentNode:      nil,
		},
		{
			note:                         "expected .spec.template.spec.containers[0].name scalar",
			expectedNode:                 ySpecTemplateSpecContainersZeroNameScalar,
			gotNode:                      node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[0].Node,
			confirmConfigNode:            node.Content[0].Content[1].Content[1].Content[1].Content[1].Content[0].Content[0],
			expectedFollowingContentNode: nil,
			gotFollowingContentNode:      nil,
		},
		{
			note:                         "expected .spec.template.spec.containers[0].name value scalar",
			expectedNode:                 ySpecTemplateSpecContainersZeroNameValueScalar,
			gotNode:                      node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[1].Node,
			confirmConfigNode:            node.Content[0].Content[1].Content[1].Content[1].Content[1].Content[0].Content[1],
			expectedFollowingContentNode: nil,
			gotFollowingContentNode:      nil,
		},

		{
			note:                         "expected .spec.template.spec.containers[0].command scalar",
			expectedNode:                 ySpecTemplateSpecContainersZeroCommandScalar,
			gotNode:                      node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[2].Node,
			confirmConfigNode:            node.Content[0].Content[1].Content[1].Content[1].Content[1].Content[0].Content[2],
			expectedPreviousLineNode:     ySpecTemplateSpecContainersZeroNameScalar,
			gotPreviousLineNode:          node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[2].PreviousLineNode,
			expectedFollowingContentNode: ySpecTemplateSpecContainersZeroCommandSequence,
			gotFollowingContentNode:      node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[2].FollowingContentNode,
		},

		{
			note:                         "expected .spec.template.spec.containers[0].command sequence",
			expectedNode:                 ySpecTemplateSpecContainersZeroCommandSequence,
			gotNode:                      node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[3].Node,
			confirmConfigNode:            node.Content[0].Content[1].Content[1].Content[1].Content[1].Content[0].Content[3],
			expectedFollowingContentNode: nil,
			gotFollowingContentNode:      nil,
		},
	}

	for _, tc := range testCases {
		// Node
		//   confirm we didn't mess up the tests
		if tc.confirmConfigNode != tc.expectedNode {
			t.Errorf("Description: %s: main.WalkConvertYamlNodeToMainNode(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.expectedNode, tc.confirmConfigNode)
		}
		if tc.gotNode != tc.expectedNode {
			t.Errorf("Description: %s: main.WalkConvertYamlNodeToMainNode(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.expectedNode, tc.gotNode)
		}

		// PreviousLineNode
		switch {
		case tc.gotPreviousLineNode != nil && tc.gotPreviousLineNode.Node != tc.expectedPreviousLineNode:
			t.Errorf("Description: %s: main.WalkConvertYamlNodeToMainNode(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.expectedPreviousLineNode, tc.gotPreviousLineNode.Node)
		case tc.gotPreviousLineNode == nil && tc.expectedPreviousLineNode != nil:
			t.Errorf("Description: %s: main.WalkConvertYamlNodeToMainNode(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.expectedPreviousLineNode, nil)
		}

		// FollowingContentNode
		switch {
		case tc.gotFollowingContentNode != nil && tc.gotFollowingContentNode.Node != tc.expectedFollowingContentNode:
			t.Errorf("Description: %s: main.WalkConvertYamlNodeToMainNode(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.expectedFollowingContentNode, tc.gotFollowingContentNode.Node)
		case tc.gotFollowingContentNode == nil && tc.expectedFollowingContentNode != nil:
			t.Errorf("Description: %s: main.WalkConvertYamlNodeToMainNode(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.expectedFollowingContentNode, nil)
		}
	}

}

func TestGetSchemaType(t *testing.T) {
	type testCase struct {
		note     string
		yaml     string
		expected string
	}

	testCases := []testCase{
		{
			note: "header comment kind generic",
			yaml: `---
# predictable-yaml: kind=generic,ignore-requireds
kind: Deployment
spec:
  asdf`,
			expected: "generic",
		},
		{
			note: "line comment kind generic",
			yaml: `---
kind: Deployment  # predictable-yaml: kind=generic
spec:
  asdf`,
			expected: "generic",
		},
		{
			note: "footer comment kind generic",
			yaml: `---
kind: Deployment
# predictable-yaml: kind=generic

spec:
  asdf`,
			expected: "generic",
		},
		{
			note: "regular kind deployment",
			yaml: `---
kind: Deployment
spec:
  asdf`,
			expected: "Deployment",
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
		if got.Kind != tc.expected {
			t.Errorf("Description: %s: main.GetFileConfigs(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, tc.expected, got)
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
			path:         ".spec",
			expectedNode: node.NodeContent[0].NodeContent[0],
		},
		{
			path:         ".spec.template.spec",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[0],
		},
		{
			path:         ".spec.template.spec.containers",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0],
		},
		{
			path:         ".spec.template.spec.containers.0",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0],
		},
		{
			path:         ".spec.template.spec.containers.0.name",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[0],
		},
		{
			path:         ".spec.template.spec.containers.0.command",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[2],
		},
		{
			path:         ".spec.template.spec.containers.0.command.1",
			expectedNode: node.NodeContent[0].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[1].NodeContent[0].NodeContent[3].NodeContent[1],
		},
	}

	for _, tc := range testCases {
		// do it
		splitPath := strings.Split(tc.path, ".")
		gotNode, err := walkToNodeForPath(node, splitPath, 0)
		if err != nil {
			t.Errorf("Description: %s: main.walkToNodeForPath(...): \n-expected:\nno error\n+got:\n%#v\n", tc.path, err)
		}
		if gotNode != tc.expectedNode {
			t.Errorf("Description: %s: main.walkToNodeForPath(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.path, tc.expectedNode.Node, gotNode.Node)
			continue
		}
	}
}

func TestWalkParseLoadConfigComments(t *testing.T) {
	type testCase struct {
		note     string
		yaml     string
		path     string
		first    bool
		required bool
		ditto    string
	}

	// note: some of these comments might not make sense for a Kubernetes
	//   deployment but are here for testing.
	testYamlContainers := `---
spec:  # first
  template:  # required
    spec:  # first, required
      initContainers:  # ditto: .spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf
`

	testCases := []testCase{
		{
			note:     ".spec: first",
			yaml:     testYamlContainers,
			path:     ".spec",
			first:    true,
			required: false,
			ditto:    "",
		},
		{
			note:     ".spec.template: required",
			yaml:     testYamlContainers,
			path:     ".spec.template",
			first:    false,
			required: true,
			ditto:    "",
		},
		{
			note:     ".spec.template.spec: first, required",
			yaml:     testYamlContainers,
			path:     ".spec.template.spec",
			first:    true,
			required: true,
			ditto:    "",
		},
		{
			note:     ".spec.template.spec.initContainers: ditto: .spec.template.spec.containers",
			yaml:     testYamlContainers,
			path:     ".spec.template.spec.initContainers",
			first:    false,
			required: false,
			ditto:    ".spec.template.spec.containers",
		},
		{
			note:     ".spec.template.spec.containers: (defaults)",
			yaml:     testYamlContainers,
			path:     ".spec.template.spec.containers",
			first:    false,
			required: false,
			ditto:    "",
		},
	}

	for _, tc := range testCases {
		// convert yaml
		n := &yaml.Node{}
		err := yaml.Unmarshal([]byte(tc.yaml), n)
		if err != nil {
			t.Fatalf("Description: %s: main.WalkParseLoadConfigComments(...): failed unmarshaling test data!\n\t%v", tc.note, err)
		}

		node := &Node{Node: n}
		WalkConvertYamlNodeToMainNode(node)
		WalkParseLoadConfigComments(node)

		// find node to test via path
		splitPath := strings.Split(tc.path, ".")
		childNode, err := walkToNodeForPath(node, splitPath, 0)
		if err != nil {
			t.Fatalf("Description: %s: main.WalkParseLoadConfigComments(...): expected: no error, got: %#v", tc.note, err)
		}
		if childNode == nil {
			t.Fatalf("Description: %s: main.WalkParseLoadConfigComments(...): failed finding test data node path!", tc.note)
		}

		// do it
		if childNode.MustBeFirst != tc.first {
			t.Errorf("Description: %s: main.WalkParseLoadConfigComments(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.first, childNode.MustBeFirst)
		}
		if childNode.Required != tc.required {
			t.Errorf("Description: %s: main.WalkParseLoadConfigComments(...): -expected, +got:\n-%#v\n+%#v\n", tc.note, tc.required, childNode.Required)
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
			path:     ".spec.template.spec.containers.0.name",
			expected: ".spec.template.spec.containers[0].name",
			yaml:     testYamlContainers,
		},
		{
			note:     ".spec.template.spec.containers[1].command[2]",
			path:     ".spec.template.spec.containers.1.command.2",
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
		splitPath := strings.Split(tc.path, ".")
		childNode, err := walkToNodeForPath(node, splitPath, 0)
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
		configYaml   string
		fileYaml     string
	}

	testCases := []testCase{
		{
			note:         "all in order",
			expectedErrs: ValidationErrors{},
			configYaml: `---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      initContainers:  # ditto: .spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`,
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
			configYaml: `---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      initContainers:  # ditto: .spec.template.spec.containers
      containers:
      - name: cool-app  # first, required`,
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
			configYaml: `---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      initContainers:  # ditto: .spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`,
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
			configYaml: `---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      initContainers:  # ditto: .spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`,
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
			configYaml: `---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      initContainers:  # ditto: .spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`,
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
				fmt.Errorf("validation error: want '.spec.template.spec.initContainers[0].name' to be first, got '.spec.template.spec.initContainers[0].args'"),
				fmt.Errorf("validation error: want '.spec.template.spec.initContainers[0].args' to be after '.spec.template.spec.initContainers[0].name', is before"),
				fmt.Errorf("validation error: missing required key '.spec.template.spec.initContainers[1].name'"),
			},
			configYaml: `---
kind: Deployment # first
spec:
  template:  # required
    spec:  # first, required
      initContainers:  # ditto: .spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`,
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
			note:         "overrides 1",
			expectedErrs: ValidationErrors{},
			configYaml: `---
# predictable-yaml: kind=Deployment
spec: # first
  template:  # required
    spec:  # first, required
      initContainers:  # ditto: .spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`,
			fileYaml: `---
# predictable-yaml: ignore-requireds, kind=Deployment
spec:
  template: {}`,
		},
		{
			note:         "overrides ignore",
			expectedErrs: ValidationErrors{},
			configYaml: `---
# predictable-yaml: kind=Deployment
spec: # first
  template:  # required
    spec:  # first, required
      initContainers:  # ditto: .spec.template.spec.containers
      containers:
      - name: cool-app  # first, required
        command:
        - asdf
        args:
        - asdf`,
			fileYaml: `---
# predictable-yaml: ignore
spec:
  template: {}`,
		},
	}

	for _, tc := range testCases {
		// convert yaml
		cN := &yaml.Node{}
		err := yaml.Unmarshal([]byte(tc.configYaml), cN)
		if err != nil {
			t.Errorf("Description: %s: main.WalkAndCompare(...): failed unmarshaling config test data!", tc.note)
			continue
		}
		configNode := &Node{Node: cN}
		WalkConvertYamlNodeToMainNode(configNode)
		WalkParseLoadConfigComments(configNode)

		fN := &yaml.Node{}
		err = yaml.Unmarshal([]byte(tc.fileYaml), fN)
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
		gotErrs := WalkAndCompare(configNode, fileNode, fileConfigs, ValidationErrors{})
		expected := GetValidationErrorStrings(tc.expectedErrs)
		got := GetValidationErrorStrings(gotErrs)
		if got != expected {
			t.Errorf("Description: %s: main.WalkAndCompare(...): \n-expected:\n%v\n+got:\n%v\n", tc.note, expected, got)
			continue
		}
	}
}
