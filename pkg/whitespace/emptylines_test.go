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
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"gopkg.in/yaml.v3"
)

func TestPreserveEmptyLines(t *testing.T) {
	type testCase struct {
		note            string
		oldContent      string
		newContent      string
		expectedContent string
	}

	testCases := []testCase{
		{
			note: "deployment",
			oldContent: `apiVersion: apps/v1
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
    someOtherThing:
      asdf: fdsa

    spec:
      containers:
      - name: cool-app
        command: # comment about command
        - asdf
        image: example

      - name: uncool-app
        command:
        - asdf
        - fdsa
      - name: mediocre-app

      otherThing:
      - name: yeah

        asdf: fdas
        qwer: trew

        bvcx: yuii

`,
			newContent: `apiVersion: apps/v1
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
      otherThing:
      - name: yeah
        asdf: fdas
        bvcx: yuii
        qwer: trew
    someOtherThing:
      asdf: fdsa
`,
			expectedContent: `apiVersion: apps/v1
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

      otherThing:
      - name: yeah

        asdf: fdas

        bvcx: yuii
        qwer: trew
    someOtherThing:
      asdf: fdsa

`,
		},
		{
			note: "kustomization.yaml",
			oldContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../base/some-app
- ../base/some-app2

patchesStrategicMerge:
- common/configMap.my-app-env.yaml

components:
- ../components/change-something

patchesJson6902:
- target: {}

  asdf: fdsa
  fdsa: rewq

images:
- newTag: my-tag
  name: busybox

generatorOptions:
  labels: {}

configMapGenerator:
- name: my-config

replicas:
- count: 1
  name: some-app
`,
			newContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
replicas:
- name: some-app
  count: 1
resources:
- ../base/some-app
- ../base/some-app2
components:
- ../components/change-something
patchesStrategicMerge:
- common/configMap.my-app-env.yaml
patchesJson6902:
- target: {}
  fdsa: rewq
  asdf: fdsa
generatorOptions:
  labels: {}
configMapGenerator:
- name: my-config
images:
- name: busybox
  newTag: my-tag
`,
			expectedContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

replicas:
- name: some-app
  count: 1

resources:
- ../base/some-app
- ../base/some-app2

components:
- ../components/change-something

patchesStrategicMerge:
- common/configMap.my-app-env.yaml

patchesJson6902:
- target: {}
  fdsa: rewq

  asdf: fdsa

generatorOptions:
  labels: {}

configMapGenerator:
- name: my-config

images:
- name: busybox
  newTag: my-tag
`,
		},
		{
			note: "new line at end",
			oldContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../base/some-app
- ../base/some-app2

patchesStrategicMerge:
- common/configMap.my-app-env.yaml

replicas:
- count: 1
  name: some-app

`,
			newContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
replicas:
- name: some-app
  count: 1
resources:
- ../base/some-app
- ../base/some-app2
patchesStrategicMerge:
- common/configMap.my-app-env.yaml
`,
			expectedContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

replicas:
- name: some-app
  count: 1

resources:
- ../base/some-app
- ../base/some-app2

patchesStrategicMerge:
- common/configMap.my-app-env.yaml

`,
		},
		{
			note: "flowStyle",
			oldContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources: ["- ../base/some-app", "- ../base/some-app2"]

patchesStrategicMerge:
- common/configMap.my-app-env.yaml

asdf: { fdsa: qwer, gfds: zxcv }

replicas:
- count: 1
  name: some-app
`,
			newContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
replicas:
- name: some-app
  count: 1
resources: ["- ../base/some-app", "- ../base/some-app2"]
patchesStrategicMerge:
- common/configMap.my-app-env.yaml
asdf: { fdsa: qwer, gfds: zxcv }
`,
			expectedContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

replicas:
- name: some-app
  count: 1

resources: ["- ../base/some-app", "- ../base/some-app2"]

patchesStrategicMerge:
- common/configMap.my-app-env.yaml

asdf: { fdsa: qwer, gfds: zxcv }
`,
		},
		{
			note: "empty lines above head comment",
			oldContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

patchesJson6902:
- target: {}

# another head comment
- target: {}
- target: {}

# this is a head comment
# multiline
resources:
- ../base/some-app
  # this is a foot comment

- ../base/some-app2
`,
			newContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
# this is a head comment
# multiline
resources:
- ../base/some-app
  # this is a foot comment
- ../base/some-app2
patchesJson6902:
- target: {}
# another head comment
- target: {}
- target: {}
`,
			expectedContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

# this is a head comment
# multiline
resources:
- ../base/some-app
  # this is a foot comment

- ../base/some-app2

patchesJson6902:
- target: {}

# another head comment
- target: {}
- target: {}
`,
		},
		{
			note: "empty lines above head comment 2",
			oldContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

# some big long
# multiline comment

# blah blah blah

resources:
- ../base/some-app
- ../base/some-app2
`,
			newContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
# some big long
# multiline comment

# blah blah blah
resources:
- ../base/some-app
- ../base/some-app2
`,
			expectedContent: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

# some big long
# multiline comment

# blah blah blah

resources:
- ../base/some-app
- ../base/some-app2
`,
		},
	}
	for _, tc := range testCases {
		got, err := PreserveEmptyLines([]byte(tc.oldContent), []byte(tc.newContent))
		if err != nil {
			t.Errorf("Description: %s: whitespace.PreserveEmptyLines(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, nil, err)
			continue
		}
		if string(got) != tc.expectedContent {
			t.Errorf("Description: %s: whitespace.PreserveEmptyLines(...): \n-expected:\n'%s'\n+got:\n'%s'\n", tc.note, tc.expectedContent, string(got))
		}
	}
}

func TestGetLineNumbersToInsertAbove(t *testing.T) {
	type testCase struct {
		note                string
		oldYaml             string
		newYaml             string
		expectedLineNumbers insertAboveLines
	}

	testCases := []testCase{
		{
			note: "deployment",
			oldYaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: example
  # comment about name
  name: example

  labels:
    asdf: fdsa
    app: example


spec:
  template:
    spec:
      containers:
      - image: example
        command: # comment about command
        - asdf
        name: cool-app

      - name: another-example`,
			newYaml: `apiVersion: apps/v1
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
      - name: another-example`,
			expectedLineNumbers: []insertAboveLine{
				{
					emptyLineCount: 1,
					lineNumber:     7,
					oldLineNumber:  8,
				},
				{
					emptyLineCount: 2,
					lineNumber:     11,
					oldLineNumber:  13,
				},
				{
					emptyLineCount: 1,
					lineNumber:     19,
					oldLineNumber:  22,
				},
			},
		},
		{
			note: "kustomization.yaml",
			oldYaml: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../base/some-app
- ../base/some-app2

patchesStrategicMerge:
- common/configMap.my-app-env.yaml

components:
- ../components/change-something

patchesJson6902:
- target: {}

  asdf: fdsa
  fdsa: rewq

images:
- newTag: my-tag
  name: busybox

generatorOptions:
  labels: {}

configMapGenerator:
- name: my-config

replicas:
- count: 1
  name: some-app
`,
			newYaml: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
replicas:
- name: some-app
  count: 1
resources:
- ../base/some-app
- ../base/some-app2
components:
- ../components/change-something
patchesStrategicMerge:
- common/configMap.my-app-env.yaml
patchesJson6902:
- target: {}
  fdsa: rewq
  asdf: fdsa
generatorOptions:
  labels: {}
configMapGenerator:
- name: my-config
images:
- name: busybox
  newTag: my-tag
`,
			expectedLineNumbers: []insertAboveLine{
				{ // resources
					emptyLineCount: 1,
					lineNumber:     6,
					oldLineNumber:  4,
				},
				{ // patchesStrategicMerge
					emptyLineCount: 1,
					lineNumber:     11,
					oldLineNumber:  8,
				},
				{ // components
					emptyLineCount: 1,
					lineNumber:     9,
					oldLineNumber:  11,
				},
				{ // patchesJson6902
					emptyLineCount: 1,
					lineNumber:     13,
					oldLineNumber:  14,
				},
				{ // asdf
					emptyLineCount: 1,
					lineNumber:     16,
					oldLineNumber:  17,
				},
				{ // images
					emptyLineCount: 1,
					lineNumber:     21,
					oldLineNumber:  20,
				},
				{ // generatorOptions
					emptyLineCount: 1,
					lineNumber:     17,
					oldLineNumber:  24,
				},
				{ // configMapGenerator
					emptyLineCount: 1,
					lineNumber:     19,
					oldLineNumber:  27,
				},
				{ // replicas
					emptyLineCount: 1,
					lineNumber:     3,
					oldLineNumber:  30,
				},
			},
		},
		{
			note: "lines above head comments",
			oldYaml: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

# this is a head comment
# multiline
resources:
- ../base/some-app
- ../base/some-app2
`,
			newYaml: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
# this is a head comment
# multiline
resources:
- ../base/some-app
- ../base/some-app2
`,
			expectedLineNumbers: []insertAboveLine{
				{
					emptyLineCount: 1,
					lineNumber:     3,
					oldLineNumber:  4,
				},
			},
		},
	}
	for _, tc := range testCases {

		oldYamlNode := &yaml.Node{}
		err := yaml.Unmarshal([]byte(tc.oldYaml), oldYamlNode)
		if err != nil {
			t.Errorf("Description: %s: whitespace.getLineNumbersToInsertAbove(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, nil, err)
			continue
		}
		oldNode := &compare.Node{Node: oldYamlNode}
		compare.WalkConvertYamlNodeToMainNode(oldNode)

		newYamlNode := &yaml.Node{}
		err = yaml.Unmarshal([]byte(tc.newYaml), newYamlNode)
		if err != nil {
			t.Fatalf("Description: %s: whitespace.getLineNumbersToInsertAbove(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, nil, err)
		}
		newNode := &compare.Node{Node: newYamlNode}
		compare.WalkConvertYamlNodeToMainNode(newNode)

		// convert to map
		oldLinesMap := getLinesMap([]byte(tc.oldYaml))
		newLinesMap := getLinesMap([]byte(tc.newYaml))

		// do it
		got, err := getLineNumbersToInsertAbove(oldLinesMap, newLinesMap, oldNode, newNode)
		if err != nil {
			t.Errorf("Description: %s: whitespace.getLineNumbersToInsertAbove(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, nil, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.expectedLineNumbers) {
			t.Errorf("Description: %s: whitespace.getLineNumbersToInsertAbove(...): \n-expected:\n%s\n+got:\n%s\n", tc.note, printExpectedLineNumbers(tc.expectedLineNumbers), printExpectedLineNumbers(got))
		}
	}
}

func TestInsertEmptyLines(t *testing.T) {
	type testCase struct {
		note             string
		inserts          insertAboveLines
		linesStr         string
		expectedLinesStr string
	}

	testCases := []testCase{
		{
			note: "simple",
			inserts: insertAboveLines{
				{
					emptyLineCount: 1,
					lineNumber:     2,
				},
				{
					emptyLineCount: 2,
					lineNumber:     4,
				},
			},
			linesStr: `asdf
fdsa
qwer
rewq`,
			expectedLinesStr: `asdf

fdsa
qwer


rewq`,
		},
		{
			note: "kustomization.yaml",
			inserts: insertAboveLines{
				{ // resources
					emptyLineCount: 1,
					lineNumber:     6,
					oldLineNumber:  4,
				},
				{ // patchesStrategicMerge
					emptyLineCount: 1,
					lineNumber:     11,
					oldLineNumber:  8,
				},
				{ // components
					emptyLineCount: 1,
					lineNumber:     9,
					oldLineNumber:  11,
				},
				{ // patchesJson6902
					emptyLineCount: 1,
					lineNumber:     13,
					oldLineNumber:  14,
				},
				{ // asdf
					emptyLineCount: 1,
					lineNumber:     16,
					oldLineNumber:  17,
				},
				{ // images
					emptyLineCount: 1,
					lineNumber:     21,
					oldLineNumber:  20,
				},
				{ // generatorOptions
					emptyLineCount: 1,
					lineNumber:     17,
					oldLineNumber:  24,
				},
				{ // configMapGenerator
					emptyLineCount: 1,
					lineNumber:     19,
					oldLineNumber:  27,
				},
				{ // replicas
					emptyLineCount: 1,
					lineNumber:     3,
					oldLineNumber:  30,
				},
			},
			linesStr: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
replicas:
- name: some-app
  count: 1
resources:
- ../base/some-app
- ../base/some-app2
components:
- ../components/change-something
patchesStrategicMerge:
- common/configMap.my-app-env.yaml
patchesJson6902:
- target: {}
  fdsa: rewq
  asdf: fdsa
generatorOptions:
  labels: {}
configMapGenerator:
- name: my-config
images:
- name: busybox
  newTag: my-tag
`,
			expectedLinesStr: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

replicas:
- name: some-app
  count: 1

resources:
- ../base/some-app
- ../base/some-app2

components:
- ../components/change-something

patchesStrategicMerge:
- common/configMap.my-app-env.yaml

patchesJson6902:
- target: {}
  fdsa: rewq

  asdf: fdsa

generatorOptions:
  labels: {}

configMapGenerator:
- name: my-config

images:
- name: busybox
  newTag: my-tag
`,
		},
	}
	for _, tc := range testCases {
		lines := strings.Split(string(tc.linesStr), "\n")

		// do it
		sort.SliceStable(tc.inserts, func(i, j int) bool {
			return tc.inserts[i].lineNumber < tc.inserts[j].lineNumber
		})
		got := insertEmptyLines(tc.inserts, lines)
		gotStr := strings.Join(got, "\n")
		if !reflect.DeepEqual(gotStr, tc.expectedLinesStr) {
			t.Errorf("Description: %s: whitespace.insertEmptyLines(...): \n-expected:\n'%s'\n+got:\n'%s'\n", tc.note, tc.expectedLinesStr, gotStr)
		}
	}
}

func TestDeduplicateInserts(t *testing.T) {
	type testCase struct {
		note            string
		inserts         insertAboveLines
		expectedInserts insertAboveLines
	}

	testCases := []testCase{
		{
			note: "simple",
			inserts: insertAboveLines{
				{
					emptyLineCount: 1,
					lineNumber:     2,
					oldLineNumber:  3,
				},
				{
					emptyLineCount: 1,
					lineNumber:     2,
					oldLineNumber:  3,
				},
				{
					emptyLineCount: 2,
					lineNumber:     4,
					oldLineNumber:  5,
				},
				{
					emptyLineCount: 2,
					lineNumber:     4,
					oldLineNumber:  5,
				},
				{
					emptyLineCount: 1,
					lineNumber:     9,
					oldLineNumber:  11,
				},
			},
			expectedInserts: insertAboveLines{
				{
					emptyLineCount: 1,
					lineNumber:     2,
					oldLineNumber:  3,
				},
				{
					emptyLineCount: 2,
					lineNumber:     4,
					oldLineNumber:  5,
				},
				{
					emptyLineCount: 1,
					lineNumber:     9,
					oldLineNumber:  11,
				},
			},
		},
	}
	for _, tc := range testCases {
		// do it
		got := deduplicateInserts(tc.inserts)
		if !reflect.DeepEqual(got, tc.expectedInserts) {
			t.Errorf("Description: %s: whitespace.deduplicateInserts(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, tc.expectedInserts, got)
		}
	}
}

func printExpectedLineNumbers(insertAboves []insertAboveLine) string {
	contents := ""
	for _, iA := range insertAboves {
		contents += fmt.Sprintf("%#v\n", iA)
	}

	return contents
}
