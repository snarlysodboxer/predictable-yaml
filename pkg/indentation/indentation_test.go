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
		{
			note:     "lists with comments",
			reduceBy: 2,
			yaml: `images:
  ## category comment 1
  # - name: my-image
  #   newTag: dev

  ## category comment 2
  # - name: my-other-image
  #   newTag: 1.2.3
  - name: my-other-image
    newTag: dev
  # - name: fdsa
  #   newTag: dev
  - name: meh
    newName: my-new-image
    newTag: asdf

scalarList:
  # this comment reqw
  - asdf
  - fdsa
  - qewr
  # - jktr
  # - jkwe
  - yuio
`,
			expected: `images:
## category comment 1
# - name: my-image
#   newTag: dev

## category comment 2
# - name: my-other-image
#   newTag: 1.2.3
- name: my-other-image
  newTag: dev
# - name: fdsa
#   newTag: dev
- name: meh
  newName: my-new-image
  newTag: asdf

scalarList:
# this comment reqw
- asdf
- fdsa
- qewr
# - jktr
# - jkwe
- yuio
`,
		},
		{
			note:     "head comments on lists 2. they can have empty blank lines in the middle",
			reduceBy: 2,
			yaml: `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../base
  - service.yaml
  # - ../other-base  # a commented out comment
  # - ../third-base

images:
  # - name: my-image
  #   newTag: my-tag

  ## category comment
  # - name: my-other-image
  #   newTag: my-tag
  - name: my-other-image
    newTag: dev`,
			expected: `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../base
- service.yaml
# - ../other-base  # a commented out comment
# - ../third-base

images:
# - name: my-image
#   newTag: my-tag

## category comment
# - name: my-other-image
#   newTag: my-tag
- name: my-other-image
  newTag: dev`,
		},
		{
			note:     "foot comments on lists. they can have empty blank lines in the middle",
			reduceBy: 2,
			yaml: `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../base
  - service.yaml
    # - ../other-base  # a commented out comment
    # - ../third-base

images:
  - name: my-other-image
    newTag: dev
    # - name: my-image
    #   newTag: my-tag

    ## category comment
    # - name: my-other-image
    #   newTag: my-tag

  - name: my--image
    #   newTag: my-tag
asdf:
  fdsa: qwer`,
			expected: `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../base
- service.yaml
  # - ../other-base  # a commented out comment
  # - ../third-base

images:
- name: my-other-image
  newTag: dev
  # - name: my-image
  #   newTag: my-tag

  ## category comment
  # - name: my-other-image
  #   newTag: my-tag

- name: my--image
  #   newTag: my-tag
asdf:
  fdsa: qwer`,
		},
	}
	for _, tc := range testCases {
		got, err := FixLists([]byte(tc.yaml), tc.reduceBy)
		if err != nil {
			t.Errorf("Description: %s: indentation.FixLists(...): \n-expected:\n%#v\n+got:\n%#v\n", tc.note, nil, err)
		}
		if string(got) != tc.expected {
			t.Errorf("Description: %s: indentation.FixLists(...): \n-expected:\n'%s'\n+got:\n'%s'\n", tc.note, tc.expected, string(got))
		}
	}
}
