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
	"strings"
	"testing"

	"github.com/snarlysodboxer/predictable-yaml/pkg/compare"
	"go.yaml.in/yaml/v3"
)

func parseToNode(t *testing.T, content string) *compare.Node {
	t.Helper()
	yamlNode := &yaml.Node{}
	err := yaml.Unmarshal([]byte(content), yamlNode)
	if err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}
	node := &compare.Node{Node: yamlNode}
	compare.WalkConvertYamlNodeToMainNode(node)
	return node
}

func scalarKey(key, value string) KeyInfo {
	return KeyInfo{Key: key, ValueKind: yaml.ScalarNode, Value: value}
}

func mappingKey(key string) KeyInfo {
	return KeyInfo{Key: key, ValueKind: yaml.MappingNode}
}

func sequenceKey(key string) KeyInfo {
	return KeyInfo{Key: key, ValueKind: yaml.SequenceNode}
}

func TestComputeDescriptions(t *testing.T) {
	tests := []struct {
		name        string
		oldYAML     string
		newYAML     string
		wantDescs   int
		wantContain string // substring to look for in FormatSummary output
	}{
		{
			name: "no changes",
			oldYAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test`,
			newYAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test`,
			wantDescs: 0,
		},
		{
			name: "simple key reorder at top level",
			oldYAML: `metadata:
  labels:
    app: test
  name: test
  namespace: default`,
			newYAML: `metadata:
  name: test
  namespace: default
  labels:
    app: test`,
			wantDescs:   2, // name move to top, namespace move up
			wantContain: "name",
		},
		{
			name: "key move to top",
			oldYAML: `metadata:
  labels:
    app: test
  namespace: default
  name: test`,
			newYAML: `metadata:
  name: test
  labels:
    app: test
  namespace: default`,
			wantDescs:   1,
			wantContain: "move to top",
		},
		{
			name: "nested key reorder",
			oldYAML: `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - ports:
        - containerPort: 8080
        name: app
        image: img`,
			newYAML: `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: img
        ports:
        - containerPort: 8080`,
			wantDescs:   2, // name move to top, image move up
			wantContain: "name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldNode := parseToNode(t, tc.oldYAML)
			newNode := parseToNode(t, tc.newYAML)

			descs := ComputeDescriptions(oldNode, newNode)

			if len(descs) != tc.wantDescs {
				t.Errorf("got %d descriptions, want %d. descriptions: %+v", len(descs), tc.wantDescs, descs)
			}

			if tc.wantContain != "" {
				summary := FormatSummary("test.yaml", descs, nil, 0)
				if !strings.Contains(summary, tc.wantContain) {
					t.Errorf("summary doesn't contain %q:\n%s", tc.wantContain, summary)
				}
			}
		})
	}
}

func TestCountComments(t *testing.T) {
	node := parseToNode(t, `apiVersion: apps/v1 # inline
# head comment
kind: Deployment
metadata:
  name: test
  # foot comment`)

	count := CountComments(node)
	if count < 2 {
		t.Errorf("expected at least 2 comments, got %d", count)
	}
}

func TestFormatSummary(t *testing.T) {
	descs := []MoveDescription{
		{Path: "metadata", Keys: []KeyInfo{scalarKey("name", "cool-app"), scalarKey("namespace", "default")}, Action: "move up"},
		{Path: "spec.template.spec.containers[0]", Keys: []KeyInfo{scalarKey("name", "app")}, Action: "move to top"},
	}
	added := []compare.AddedField{{Path: ".spec.template.spec.containers[0]", Key: "imagePullPolicy"}}

	summary := FormatSummary("deployment.yaml", descs, added, 3)

	if !strings.Contains(summary, "deployment.yaml") {
		t.Error("summary missing file path")
	}
	if !strings.Contains(summary, "name: cool-app") {
		t.Errorf("summary missing key with value:\n%s", summary)
	}
	if !strings.Contains(summary, "# move up") {
		t.Errorf("summary missing move action as comment:\n%s", summary)
	}
	if !strings.Contains(summary, "# move to top") {
		t.Errorf("summary missing move to top as comment:\n%s", summary)
	}
	if !strings.Contains(summary, "imagePullPolicy: TODO  # add") {
		t.Errorf("summary missing added field:\n%s", summary)
	}
	if !strings.Contains(summary, "all 3 comments preserved") {
		t.Error("summary missing comment count")
	}

	// Verify nested structure: "metadata:" should appear once with
	// both moves indented under it
	metadataCount := strings.Count(summary, "    metadata:\n")
	if metadataCount != 1 {
		t.Errorf("expected metadata heading once, got %d times:\n%s", metadataCount, summary)
	}

	// Verify nesting: spec > template > spec > containers[0]
	if !strings.Contains(summary, "    spec:\n      template:\n        spec:\n          containers[0]:\n") {
		t.Errorf("expected nested path structure:\n%s", summary)
	}
}

func TestFormatSummaryGrouping(t *testing.T) {
	// Two moves at the same path should be grouped under one heading
	descs := []MoveDescription{
		{Path: "metadata", Keys: []KeyInfo{scalarKey("name", "test")}, Action: "move to top"},
		{Path: "metadata", Keys: []KeyInfo{scalarKey("namespace", "default")}, Action: "move up"},
	}

	summary := FormatSummary("test.yaml", descs, nil, 0)

	metadataCount := strings.Count(summary, "metadata:\n")
	if metadataCount != 1 {
		t.Errorf("expected 'metadata:' once, got %d:\n%s", metadataCount, summary)
	}
	if !strings.Contains(summary, "name: test  # move to top") {
		t.Errorf("missing first move:\n%s", summary)
	}
	if !strings.Contains(summary, "namespace: default  # move up") {
		t.Errorf("missing second move:\n%s", summary)
	}
}

func TestFormatSummaryValueKinds(t *testing.T) {
	descs := []MoveDescription{
		{Path: "spec", Keys: []KeyInfo{
			mappingKey("selector"),
			sequenceKey("ports"),
		}, Action: "move up"},
	}

	summary := FormatSummary("test.yaml", descs, nil, 0)

	if !strings.Contains(summary, "selector: {...}  # move up") {
		t.Errorf("mapping value should show {...}:\n%s", summary)
	}
	if !strings.Contains(summary, "ports: [...]  # move up") {
		t.Errorf("sequence value should show [...]:\n%s", summary)
	}
}

func TestFormatSummaryMergesSameAction(t *testing.T) {
	// Two moves at the same path with the same action should merge keys.
	// Since keys are now on separate lines, both should appear.
	descs := []MoveDescription{
		{Path: "spec", Keys: []KeyInfo{scalarKey("port", "8080")}, Action: "move up"},
		{Path: "spec", Keys: []KeyInfo{scalarKey("targetPort", "8080")}, Action: "move up"},
	}

	summary := FormatSummary("test.yaml", descs, nil, 0)

	if !strings.Contains(summary, "port: 8080  # move up") {
		t.Errorf("missing port move:\n%s", summary)
	}
	if !strings.Contains(summary, "targetPort: 8080  # move up") {
		t.Errorf("missing targetPort move:\n%s", summary)
	}
}
