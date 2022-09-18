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
	"testing"

	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"gopkg.in/yaml.v3"
)

func TestFixLists(t *testing.T) {
	type testCase struct {
		note     string
		reduceBy int
		expected string
		yaml     string
	}

	testCases := []testCase{
		{
			note:     "deployment",
			reduceBy: 2,
			yaml: `apiVersion: apps/v1
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
    spec:
      containers:
        - name: cool-app
          image: example
          command: # comment about command
            - asdf
        - name: uncool-app
          command:
            - asdf
            - fdsa
        - name: mediocre-app
    someOtherThing:
      asdf: fdsa
`,
			expected: `apiVersion: apps/v1
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
    spec:
      containers:
      - name: cool-app
        image: example
        command: # comment about command
        - asdf
      - name: uncool-app
        command:
        - asdf
        - fdsa
      - name: mediocre-app
    someOtherThing:
      asdf: fdsa
`,
		},
		{
			note:     "string value that looks like yaml",
			reduceBy: 2,
			yaml: `---
kind: ConfigMap
containers:
  - name: cool-app
    image: example
    command: # comment about command
      - asdf
    args:
      - fdsa
      - zxcv
    asdf: [asdf, fdsa]
  - name: uncool-app
    command:
      - asdf
      - fdsa
  - name: mediocre-app
beta:
  some-thing:
    asdf:
      fdsa:
        - key: name
          value: qwer
data:
  some-file.yaml: |+
    asdf:
      fdsa:
      - key: name
        value: qwer
      - key: fdsa
        value: qwer
other:
  - target:
      version: v1
      kind: Deployment
      name: example
      namespace: example
    patch: |-
      - op: replace
        path: /spec/strategy/type
        value: Recreate
someOtherThing:
  asdf: fdsa
another:
  - patch: |-
      - op: replace
        path: /spec/strategy/type
        value: Recreate
`,
			expected: `---
kind: ConfigMap
containers:
- name: cool-app
  image: example
  command: # comment about command
  - asdf
  args:
  - fdsa
  - zxcv
  asdf: [asdf, fdsa]
- name: uncool-app
  command:
  - asdf
  - fdsa
- name: mediocre-app
beta:
  some-thing:
    asdf:
      fdsa:
      - key: name
        value: qwer
data:
  some-file.yaml: |+
    asdf:
      fdsa:
      - key: name
        value: qwer
      - key: fdsa
        value: qwer
other:
- target:
    version: v1
    kind: Deployment
    name: example
    namespace: example
  patch: |-
    - op: replace
      path: /spec/strategy/type
      value: Recreate
someOtherThing:
  asdf: fdsa
another:
- patch: |-
    - op: replace
      path: /spec/strategy/type
      value: Recreate
`,
		},
		{
			note:     "reduceBy 4",
			reduceBy: 4,
			yaml: `---
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
    spec:
      containers:
          - name: cool-app
            image: example
            command: # comment about command
                - asdf
          - name: uncool-app
            command:
                - asdf
                - fdsa
          - name: mediocre-app
    someOtherThing:
      asdf: fdsa
`,
			expected: `---
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
    spec:
      containers:
      - name: cool-app
        image: example
        command: # comment about command
        - asdf
      - name: uncool-app
        command:
        - asdf
        - fdsa
      - name: mediocre-app
    someOtherThing:
      asdf: fdsa
`,
		},
		{
			note:     "other lists",
			reduceBy: 2,
			yaml: `asdf:
  - asdf
  - name
  - fdsa
  -
      - qwer
      - rewq
`,
			expected: `asdf:
- asdf
- name
- fdsa
-
  - qwer
  - rewq
`,
		},
	}
	for _, tc := range testCases {
		yNode := &yaml.Node{}
		err := yaml.Unmarshal([]byte(tc.yaml), yNode)
		if err != nil {
			t.Errorf("Description: %s: main.FixLists(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, nil, err)
			continue
		}
		fileNode := &compare.Node{Node: yNode}
		compare.WalkConvertYamlNodeToMainNode(fileNode)
		got, err := FixLists(fileNode, []byte(tc.yaml), tc.reduceBy)
		if err != nil {
			t.Errorf("Description: %s: main.FixLists(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, nil, err)
		}
		if string(got) != tc.expected {
			t.Errorf("Description: %s: main.FixLists(...): \n-expected:\n%s\n+got:\n%s\n", tc.note, tc.expected, string(got))
		}
	}
}
