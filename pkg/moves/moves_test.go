package moves

import (
	"testing"
)

func TestComputeSimpleSwap(t *testing.T) {
	old := "a: 1\nb: 2\nc: 3"
	new := "b: 2\na: 1\nc: 3"

	moves, err := Compute(old, new)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if len(moves) == 0 {
		t.Fatal("expected moves, got none")
	}

	// Both a and b should be reported as moved
	if len(moves) != 2 {
		t.Fatalf("expected 2 moves, got %d", len(moves))
	}

	// Verify one of them is a->line2, b->line1
	foundA := false
	foundB := false
	for _, m := range moves {
		if m.FromStart == 1 && m.ToStart == 2 {
			foundA = true // "a: 1" moved from line 1 to line 2
		}
		if m.FromStart == 2 && m.ToStart == 1 {
			foundB = true // "b: 2" moved from line 2 to line 1
		}
	}
	if !foundA {
		t.Error("expected move for 'a: 1' from line 1 to line 2")
	}
	if !foundB {
		t.Error("expected move for 'b: 2' from line 2 to line 1")
	}
}

func TestComputeNoChanges(t *testing.T) {
	text := "a: 1\nb: 2\nc: 3"
	moves, err := Compute(text, text)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}
	if len(moves) != 0 {
		t.Errorf("expected 0 moves for identical text, got %d", len(moves))
	}
}

func TestComputeNestedMoves(t *testing.T) {
	old := `metadata:
  labels:
    component: exporter
    name: kube-state-metrics
  name: kube-state-metrics
  namespace: kube-system`

	new := `metadata:
  name: kube-state-metrics
  namespace: kube-system
  labels:
    name: kube-state-metrics
    component: exporter`

	moves, err := Compute(old, new)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if len(moves) == 0 {
		t.Fatal("expected moves for nested reordering, got none")
	}

	// Should have moves at both the metadata level (labels/name/namespace)
	// and within labels (component/name)
	t.Logf("Found %d moves:", len(moves))
	for _, m := range moves {
		t.Logf("  from %d-%d -> to %d-%d", m.FromStart, m.FromEnd, m.ToStart, m.ToEnd)
	}

	// At minimum we should see moves for:
	// - metadata.name moving up
	// - metadata.namespace moving up
	// - metadata.labels moving down
	// - labels.component moving down
	// - labels.name moving up
	if len(moves) < 4 {
		t.Errorf("expected at least 4 moves for nested reordering, got %d", len(moves))
	}
}

func TestComputeMultiLineValues(t *testing.T) {
	old := `spec:
  containers:
  - image: nginx
    name: web
    ports:
    - containerPort: 80
      name: http`

	new := `spec:
  containers:
  - name: web
    image: nginx
    ports:
    - name: http
      containerPort: 80`

	moves, err := Compute(old, new)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if len(moves) == 0 {
		t.Fatal("expected moves, got none")
	}

	t.Logf("Found %d moves:", len(moves))
	for _, m := range moves {
		t.Logf("  from %d-%d -> to %d-%d", m.FromStart, m.FromEnd, m.ToStart, m.ToEnd)
	}
}

func TestComputeDeploymentYAML(t *testing.T) {
	old := `---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/component: exporter
    app.kubernetes.io/name: kube-state-metrics
    app.kubernetes.io/version: 2.18.0
  name: kube-state-metrics
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: kube-state-metrics
  template:
    metadata:
      labels:
        app.kubernetes.io/component: exporter
        app.kubernetes.io/name: kube-state-metrics
        app.kubernetes.io/version: 2.18.0
    spec:
      automountServiceAccountToken: true
      containers:
      - image: registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.18.0
        livenessProbe:
          httpGet:
            path: /livez
            port: http-metrics
          initialDelaySeconds: 5
          timeoutSeconds: 5
        name: kube-state-metrics
        ports:
        - containerPort: 8080
          name: http-metrics
        - containerPort: 8081
          name: telemetry
        readinessProbe:
          httpGet:
            path: /readyz
            port: telemetry
          initialDelaySeconds: 5
          timeoutSeconds: 5
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 65534
          seccompProfile:
            type: RuntimeDefault
      nodeSelector:
        kubernetes.io/os: linux
      serviceAccountName: kube-state-metrics`

	new := `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kube-state-metrics
  namespace: kube-system
  labels:
    app.kubernetes.io/name: kube-state-metrics
    app.kubernetes.io/component: exporter
    app.kubernetes.io/version: 2.18.0
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: kube-state-metrics
  template:
    metadata:
      labels:
        app.kubernetes.io/name: kube-state-metrics
        app.kubernetes.io/component: exporter
        app.kubernetes.io/version: 2.18.0
    spec:
      serviceAccountName: kube-state-metrics
      automountServiceAccountToken: true
      containers:
      - name: kube-state-metrics
        image: registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.18.0
        ports:
        - name: http-metrics
          containerPort: 8080
        - name: telemetry
          containerPort: 8081
        livenessProbe:
          initialDelaySeconds: 5
          timeoutSeconds: 5
          httpGet:
            port: http-metrics
            path: /livez
        readinessProbe:
          initialDelaySeconds: 5
          timeoutSeconds: 5
          httpGet:
            port: telemetry
            path: /readyz
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 65534
          seccompProfile:
            type: RuntimeDefault
      nodeSelector:
        kubernetes.io/os: linux`

	moves, err := Compute(old, new)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if len(moves) == 0 {
		t.Fatal("expected moves for deployment YAML reordering, got none")
	}

	t.Logf("Found %d moves:", len(moves))
	for _, m := range moves {
		t.Logf("  from %d-%d -> to %d-%d", m.FromStart, m.FromEnd, m.ToStart, m.ToEnd)
	}

	// Should detect at least these categories of moves:
	// - metadata level: labels/name/namespace reorder
	// - metadata.labels inner reorder
	// - spec.template.spec: serviceAccountName moved
	// - container keys: name, image, ports, livenessProbe reorder
	// - liveness/readiness inner reorder
	// - ports inner reorder
	if len(moves) < 10 {
		t.Errorf("expected at least 10 moves for full deployment reordering, got %d", len(moves))
	}
}
