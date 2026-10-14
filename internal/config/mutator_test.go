package config

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestMutator_GetMappingNode_EdgeCases(t *testing.T) {
	// Nil node
	if GetMappingNode(nil) != nil {
		t.Error("expected nil for nil input")
	}

	// Empty DocumentNode creates mapping node
	docEmpty := &yaml.Node{Kind: yaml.DocumentNode}
	mapNode := GetMappingNode(docEmpty)
	if mapNode == nil || mapNode.Kind != yaml.MappingNode {
		t.Fatalf("expected MappingNode created for empty DocumentNode")
	}

	// Direct MappingNode
	directMap := &yaml.Node{Kind: yaml.MappingNode}
	if GetMappingNode(directMap) != directMap {
		t.Error("expected direct MappingNode returned")
	}

	// ScalarNode should return nil
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Value: "foo"}
	if GetMappingNode(scalar) != nil {
		t.Error("expected nil for scalar node")
	}
}

func TestMutator_FindOrCreateMapping_EdgeCases(t *testing.T) {
	if FindOrCreateMapping(nil, "key") != nil {
		t.Error("expected nil for nil mapNode")
	}

	// Convert non-mapping value to mapping
	mapNode := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "target"},
			{Kind: yaml.ScalarNode, Value: "scalar_val"},
		},
	}
	child := FindOrCreateMapping(mapNode, "target")
	if child == nil || child.Kind != yaml.MappingNode {
		t.Fatalf("expected child to be converted to MappingNode")
	}

	// Finding existing mapping node directly
	child2 := FindOrCreateMapping(mapNode, "target")
	if child2 != child {
		t.Fatalf("expected same child returned")
	}

	// Create new key
	child3 := FindOrCreateMapping(mapNode, "new_key")
	if child3 == nil || child3.Kind != yaml.MappingNode {
		t.Fatalf("expected new mapping node created")
	}
}

func TestMutator_FindSequence_And_FindOrCreateSequence(t *testing.T) {
	if FindSequence(nil, "key") != nil {
		t.Error("expected nil for nil mapNode")
	}
	if FindOrCreateSequence(nil, "key") != nil {
		t.Error("expected nil for nil mapNode")
	}

	mapNode := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "seq_key"},
			{Kind: yaml.SequenceNode, Content: []*yaml.Node{}},
			{Kind: yaml.ScalarNode, Value: "scalar_key"},
			{Kind: yaml.ScalarNode, Value: "val"},
		},
	}

	// Find existing sequence
	if FindSequence(mapNode, "seq_key") == nil {
		t.Error("expected to find seq_key")
	}
	// Key that is not a sequence
	if FindSequence(mapNode, "scalar_key") != nil {
		t.Error("expected nil for scalar_key in FindSequence")
	}
	// Non-existent key
	if FindSequence(mapNode, "missing") != nil {
		t.Error("expected nil for missing key in FindSequence")
	}

	// FindOrCreateSequence on scalar converts to sequence
	converted := FindOrCreateSequence(mapNode, "scalar_key")
	if converted.Kind != yaml.SequenceNode {
		t.Error("expected scalar_key converted to SequenceNode")
	}

	// FindOrCreateSequence on existing sequence returns it
	existing := FindOrCreateSequence(mapNode, "seq_key")
	if existing == nil || existing.Kind != yaml.SequenceNode {
		t.Error("expected existing sequence returned")
	}
}

func TestMutator_SetScalarValue_InferenceAndAppend(t *testing.T) {
	SetScalarValue(nil, "key", "val", "") // Nil check

	mapNode := &yaml.Node{Kind: yaml.MappingNode}

	// Inferred !!bool
	SetScalarValue(mapNode, "is_active", "true", "")
	// Inferred !!int
	SetScalarValue(mapNode, "count", "42", "")
	// Inferred !!str
	SetScalarValue(mapNode, "name", "devclean", "")

	// Check appended nodes
	if len(mapNode.Content) != 6 {
		t.Fatalf("expected 6 content items (3 pairs), got %d", len(mapNode.Content))
	}
	if mapNode.Content[1].Tag != "!!bool" {
		t.Errorf("expected tag !!bool, got %s", mapNode.Content[1].Tag)
	}
	if mapNode.Content[3].Tag != "!!int" {
		t.Errorf("expected tag !!int, got %s", mapNode.Content[3].Tag)
	}
	if mapNode.Content[5].Tag != "!!str" {
		t.Errorf("expected tag !!str, got %s", mapNode.Content[5].Tag)
	}

	// Update existing with false
	SetScalarValue(mapNode, "is_active", "false", "")
	if mapNode.Content[1].Value != "false" || mapNode.Content[1].Tag != "!!bool" {
		t.Errorf("expected value false and tag !!bool, got %s, %s", mapNode.Content[1].Value, mapNode.Content[1].Tag)
	}
}

func TestMutator_SetSequenceValues(t *testing.T) {
	mapNode := &yaml.Node{Kind: yaml.MappingNode}
	SetSequenceValues(mapNode, "items", []string{"a", "b", "c"})

	seq := FindSequence(mapNode, "items")
	if seq == nil || len(seq.Content) != 3 {
		t.Fatalf("expected 3 items in sequence, got %v", seq)
	}
	if seq.Content[0].Value != "a" || seq.Content[1].Value != "b" || seq.Content[2].Value != "c" {
		t.Errorf("unexpected content values: %v, %v, %v", seq.Content[0].Value, seq.Content[1].Value, seq.Content[2].Value)
	}

	// Overwrite sequence
	SetSequenceValues(mapNode, "items", []string{"x"})
	if len(seq.Content) != 1 || seq.Content[0].Value != "x" {
		t.Errorf("expected replaced sequence with 'x'")
	}
}

func TestMutator_SequenceHelpers_NilAndEmpty(t *testing.T) {
	if AppendUniqueToSequence(nil, "item") {
		t.Error("expected false for nil seqNode")
	}
	if RemoveFromSequence(nil, "item") != 0 {
		t.Error("expected 0 for nil seqNode")
	}

	seq := &yaml.Node{Kind: yaml.SequenceNode}
	AppendUniqueToSequence(seq, "foo")
	// Target with empty string ignored
	removed := RemoveFromSequence(seq, "", "bar")
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
}

func TestMutator_AtomicWriteAndBackup(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	bakFile := cfgFile + ".bak"
	tmpFile := cfgFile + ".tmp"

	initialContent := `# Initial config
scan_paths:
  - "/initial/project"
`
	if err := os.WriteFile(cfgFile, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	root, _, err := LoadWithAST(cfgFile)
	if err != nil {
		t.Fatalf("LoadWithAST failed: %v", err)
	}
	mapNode := GetMappingNode(root)
	SetScalarValue(mapNode, "retention_days", "60", "!!int")

	if err := SaveConfigNode(cfgFile, root); err != nil {
		t.Fatalf("SaveConfigNode failed: %v", err)
	}

	// 1. Verify cfgFile exists and has new value
	savedData, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("reading saved cfgFile: %v", err)
	}
	if !strings.Contains(string(savedData), "retention_days: 60") {
		t.Errorf("expected retention_days: 60 in saved file, got:\n%s", string(savedData))
	}
	if !strings.Contains(string(savedData), "# Initial config") {
		t.Errorf("lost initial header comment")
	}

	// 2. Verify bakFile exists and has initial content
	bakData, err := os.ReadFile(bakFile)
	if err != nil {
		t.Fatalf("reading backup file: %v", err)
	}
	if string(bakData) != initialContent {
		t.Errorf("expected backup to match initial content, got:\n%s", string(bakData))
	}

	// 3. Verify tmpFile does not exist
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Errorf("tmp file should not exist after atomic rename")
	}

	// 4. Second save updates the backup to the previous version
	SetScalarValue(mapNode, "retention_days", "90", "!!int")
	if err := SaveConfigNode(cfgFile, root); err != nil {
		t.Fatalf("second SaveConfigNode failed: %v", err)
	}
	bakData2, err := os.ReadFile(bakFile)
	if err != nil {
		t.Fatalf("reading updated backup file: %v", err)
	}
	if !strings.Contains(string(bakData2), "retention_days: 60") {
		t.Errorf("expected updated backup to contain intermediate retention_days: 60, got:\n%s", string(bakData2))
	}
}

func TestMutator_FirstRunDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "sub", "config.yaml")
	bakFile := cfgFile + ".bak"

	// File does not exist yet
	root, cfg, err := LoadWithAST(cfgFile)
	if err != nil {
		t.Fatalf("LoadWithAST failed on missing file: %v", err)
	}
	if root == nil {
		t.Fatal("expected non-nil root AST for first run")
	}
	if cfg == nil {
		t.Fatal("expected non-nil Config struct for first run")
	}
	if !cfg.Safety.TrashEnabled {
		t.Error("expected default TrashEnabled to be true")
	}
	if !cfg.Ecosystems.Docker.Enabled {
		t.Error("expected default Docker to be enabled")
	}

	// Save default AST
	if err := SaveConfigNode(cfgFile, root); err != nil {
		t.Fatalf("SaveConfigNode failed for first run: %v", err)
	}

	savedData, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("reading created first-run config: %v", err)
	}
	savedStr := string(savedData)
	if !strings.Contains(savedStr, "scan_paths:") {
		t.Errorf("expected scan_paths in default config, got:\n%s", savedStr)
	}
	if !strings.Contains(savedStr, "ecosystems:") {
		t.Errorf("expected ecosystems in default config, got:\n%s", savedStr)
	}

	// Ensure no .bak was created since there was no previous file
	if _, err := os.Stat(bakFile); !os.IsNotExist(err) {
		t.Errorf("expected no backup file created on first run")
	}
}

func TestMutator_CorruptedYAMLSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	bakFile := cfgFile + ".bak"

	corruptedYAML := `ecosystems:
  node:
    enabled: [invalid syntax { unbalanced
`
	if err := os.WriteFile(cfgFile, []byte(corruptedYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Without backup file
	_, _, err := LoadWithAST(cfgFile)
	if err == nil {
		t.Fatal("expected error on corrupted YAML syntax, got nil")
	}

	syntaxErr, ok := err.(*ConfigSyntaxError)
	if !ok {
		t.Fatalf("expected *ConfigSyntaxError, got %T: %v", err, err)
	}
	if syntaxErr.Path != cfgFile {
		t.Errorf("expected path %s, got %s", cfgFile, syntaxErr.Path)
	}
	if syntaxErr.HasBackup {
		t.Error("expected HasBackup to be false")
	}
	if strings.Contains(syntaxErr.Error(), "Dica: Foi detectado um arquivo de backup") {
		t.Errorf("unexpected backup hint in error message: %s", syntaxErr.Error())
	}

	// With backup file present
	if err := os.WriteFile(bakFile, []byte("scan_paths: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err = LoadWithAST(cfgFile)
	if err == nil {
		t.Fatal("expected error on corrupted YAML syntax with backup")
	}
	syntaxErr, ok = err.(*ConfigSyntaxError)
	if !ok {
		t.Fatalf("expected *ConfigSyntaxError, got %T: %v", err, err)
	}
	if !syntaxErr.HasBackup {
		t.Error("expected HasBackup to be true")
	}
	if !strings.Contains(syntaxErr.Error(), "Dica: Foi detectado um arquivo de backup em") {
		t.Errorf("expected backup hint in error message, got:\n%s", syntaxErr.Error())
	}
	if !strings.Contains(syntaxErr.Error(), "Para restaurá-lo, execute: cp") {
		t.Errorf("expected restore instruction in error message, got:\n%s", syntaxErr.Error())
	}
}

