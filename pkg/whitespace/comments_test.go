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
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPreserveComments(t *testing.T) {
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
kind: Deployment  # two spaces
metadata:   # three spaces
  namespace: example
  name: example # one space
spec:
  template:
    spec:
      containers:       # fdsa
      - name: cool-app  # this
        command:        # is
        - asdf ### asdf # a
        image: example  # comment
      - command: "" # asdf
        name: uncool-app`,
			newContent: `apiVersion: apps/v1
kind: Deployment # two spaces
metadata: # three spaces
  name: example # one space
  namespace: example
spec:
  template:
    spec:
      containers: # fdsa
      - name: cool-app # this
        image: example # comment
        command: # is
        - asdf ### asdf # a
      - name: uncool-app
        command: "" # asdf`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment  # two spaces
metadata:   # three spaces
  name: example # one space
  namespace: example
spec:
  template:
    spec:
      containers:       # fdsa
      - name: cool-app  # this
        image: example  # comment
        command:        # is
        - asdf ### asdf # a
      - name: uncool-app
        command: "" # asdf`,
		},
		{
			note: "non standard comment indentations, using head/foot comment fixers",
			oldContent: `apiVersion: apps/v1
kind: Deployment  # two spaces
metadata:   # three spaces
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
      - name: cool-app  # this

        command:        # is
        - asdf ### asdf # a
        image: example  # comment
            # this counts as a footer comment for 'example'
            # and there's more to say about it

      - command: "" # asdf
        name: uncool-app`,
			newContent: `apiVersion: apps/v1
kind: Deployment # two spaces
metadata: # three spaces
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
      - name: cool-app # this
        image: example # comment
        # this counts as a footer comment for 'example'
        # and there's more to say about it

        command: # is
        - asdf ### asdf # a
      - name: uncool-app
        command: "" # asdf`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment  # two spaces
metadata:   # three spaces
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
      - name: cool-app  # this
        image: example  # comment
            # this counts as a footer comment for 'example'
            # and there's more to say about it

        command:        # is
        - asdf ### asdf # a
      - name: uncool-app
        command: "" # asdf`,
		},
		{
			note: "empty lines above and below header and footer comments",
			oldContent: `apiVersion: apps/v1
kind: Deployment  # two spaces
metadata:   # three spaces
  namespace: example
  name: example # one space
spec:
  template:
    spec:
      containers:       # fdsa

        # comment for this container 1


      - name: cool-app  # this
        command:        # is
        - asdf ### asdf # a
        image: example  # comment

        # comment for this container 2
      - name: uncool-app
        command: "" # asdf


        # comment for this container 3
          # multiline
      - name: meh-app
        command: "" # fdsa`,
			newContent: `apiVersion: apps/v1
kind: Deployment  # two spaces
metadata: # three spaces
  namespace: example
  name: example # one space
spec:
  template:
    spec:
      containers: # fdsa
      # comment for this container 1
      - name: cool-app # this
        command: # is
        - asdf ### asdf # a
        image: example # comment
        # comment for this container 2
      - name: uncool-app
        command: "" # asdf
        # comment for this container 3
        # multiline
      - name: meh-app
        command: "" # fdsa`,
			expectedContent: `apiVersion: apps/v1
kind: Deployment  # two spaces
metadata:   # three spaces
  namespace: example
  name: example # one space
spec:
  template:
    spec:
      containers:       # fdsa
        # comment for this container 1
      - name: cool-app  # this
        command:        # is
        - asdf ### asdf # a
        image: example  # comment
        # comment for this container 2
      - name: uncool-app
        command: "" # asdf
        # comment for this container 3
          # multiline
      - name: meh-app
        command: "" # fdsa`,
		},
	}
	for _, tc := range testCases {
		// --- for confirming test input
		// node := &yaml.Node{}
		// rendered, err := marshalAndUnmarshal([]byte(tc.oldContent), node, 2)
		// if err != nil {
		//     panic(err)
		// }
		// fmt.Println(string(rendered))
		// --- for confirming test input

		got, err := PreserveComments([]byte(tc.oldContent), []byte(tc.newContent))
		if err != nil {
			t.Errorf("Description: %s: whitespace.PreserveComments(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, nil, err)
			continue
		}
		if string(got) != tc.expectedContent {
			t.Errorf("Description: %s: whitespace.PreserveComments(...): \n-expected:\n%s\n+got:\n%s\n", tc.note, tc.expectedContent, string(got))
		}
	}
}

func marshalAndUnmarshal(content []byte, node *yaml.Node, indent int) ([]byte, error) {
	err := yaml.Unmarshal(content, node)
	if err != nil {
		return []byte{}, err
	}
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(indent)
	err = encoder.Encode(node)
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}
