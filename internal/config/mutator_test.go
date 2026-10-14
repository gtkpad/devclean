package config

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func renderYAML(t *testing.T, node *yaml.Node) string {
	t.Helper()
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		t.Fatalf("failed to encode yaml node: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("failed to close yaml encoder: %v", err)
	}
	return buf.String()
}

func parseYAML(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}
	return &root
}

func TestMutator_CommentPreservation_Scalars(t *testing.T) {
	initialYAML := `# DevClean Configuration
# Global scan directories
scan_paths:
  - "/Users/user/Projects" # primary workspace

ecosystems:
  # Node.js build cleanup
  node:
    enabled: true # active by default
    max_age_days: 14 # 2 weeks retention
`
	root := parseYAML(t, initialYAML)
	mapNode := GetMappingNode(root)
	if mapNode == nil {
		t.Fatal("GetMappingNode returned nil")
	}

	ecoNode := FindOrCreateMapping(mapNode, "ecosystems")
	nodeEco := FindOrCreateMapping(ecoNode, "node")

	// Mutate enabled to false and max_age_days to 30
	SetScalarValue(nodeEco, "enabled", "false", "!!bool")
	SetScalarValue(nodeEco, "max_age_days", "30", "!!int")

	savedStr := renderYAML(t, root)

	// Assert comments were preserved
	if !strings.Contains(savedStr, "# DevClean Configuration") {
		t.Errorf("lost header comment, got:\n%s", savedStr)
	}
	if !strings.Contains(savedStr, "# Global scan directories") {
		t.Errorf("lost scan directories comment, got:\n%s", savedStr)
	}
	if !strings.Contains(savedStr, "# primary workspace") {
		t.Errorf("lost primary workspace comment, got:\n%s", savedStr)
	}
	if !strings.Contains(savedStr, "# Node.js build cleanup") {
		t.Errorf("lost Node.js cleanup comment, got:\n%s", savedStr)
	}
	if !strings.Contains(savedStr, "# active by default") {
		t.Errorf("lost enabled line comment, got:\n%s", savedStr)
	}
	if !strings.Contains(savedStr, "# 2 weeks retention") {
		t.Errorf("lost max_age_days line comment, got:\n%s", savedStr)
	}

	// Assert values were updated
	if !strings.Contains(savedStr, "enabled: false") {
		t.Errorf("expected enabled: false, got:\n%s", savedStr)
	}
	if !strings.Contains(savedStr, "max_age_days: 30") {
		t.Errorf("expected max_age_days: 30, got:\n%s", savedStr)
	}
}

func TestMutator_CommentPreservation_Sequences(t *testing.T) {
	initialYAML := `# Scan path list
scan_paths:
  # First path
  - "/path/one" # path 1
  - "/path/two" # path 2
`
	root := parseYAML(t, initialYAML)
	mapNode := GetMappingNode(root)
	seqNode := FindOrCreateSequence(mapNode, "scan_paths")

	// Append unique
	if !AppendUniqueToSequence(seqNode, "/path/three") {
		t.Errorf("expected AppendUniqueToSequence to return true for new path")
	}
	// Append duplicate
	if AppendUniqueToSequence(seqNode, "/path/one") {
		t.Errorf("expected AppendUniqueToSequence to return false for duplicate path")
	}

	// Remove path two
	removed := RemoveFromSequence(seqNode, "/path/two")
	if removed != 1 {
		t.Errorf("expected 1 item removed, got %d", removed)
	}

	savedStr := renderYAML(t, root)

	// Assert comments preserved
	if !strings.Contains(savedStr, "# Scan path list") {
		t.Errorf("lost scan path list comment, got:\n%s", savedStr)
	}
	if !strings.Contains(savedStr, "# First path") {
		t.Errorf("lost first path comment, got:\n%s", savedStr)
	}
	if !strings.Contains(savedStr, "# path 1") {
		t.Errorf("lost path 1 comment, got:\n%s", savedStr)
	}

	// Assert paths updated
	if !strings.Contains(savedStr, "/path/one") {
		t.Errorf("expected /path/one to exist, got:\n%s", savedStr)
	}
	if strings.Contains(savedStr, "/path/two") {
		t.Errorf("expected /path/two to be removed, got:\n%s", savedStr)
	}
	if !strings.Contains(savedStr, "/path/three") {
		t.Errorf("expected /path/three to exist, got:\n%s", savedStr)
	}
}

func TestMutator_Idempotency(t *testing.T) {
	initialYAML := `# DevClean Configuration
# Global scan directories
scan_paths:
  - "/Users/user/Projects" # primary workspace

ecosystems:
  # Node.js build cleanup
  node:
    enabled: true # active by default
    max_age_days: 14 # 2 weeks retention
`
	root1 := parseYAML(t, initialYAML)
	round1 := renderYAML(t, root1)

	root2 := parseYAML(t, round1)
	round2 := renderYAML(t, root2)

	if round1 != round2 {
		t.Errorf("idempotency failed:\n--- Round 1 ---\n%s\n--- Round 2 ---\n%s", round1, round2)
	}
}
