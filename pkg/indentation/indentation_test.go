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
)

func TestFixLists(t *testing.T) {
	type testCase struct {
		note             string
		indentationLevel int
		expected         string
		yaml             string
	}

	testCases := []testCase{
		{
			note:             "deployment",
			indentationLevel: 2,
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
			note:             "indentationLevel 4",
			indentationLevel: 4,
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
			note:             "other lists",
			indentationLevel: 2,
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
		got := FixLists([]byte(tc.yaml), tc.indentationLevel)
		if string(got) != tc.expected {
			t.Errorf("Description: %s: main.FixLists(...): \n-expected:\n%s\n+got:\n%s\n", tc.note, tc.expected, string(got))
		}
	}
}
