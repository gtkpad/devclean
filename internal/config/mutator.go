package config

import (
	"strconv"

	"gopkg.in/yaml.v3"
)

// GetMappingNode returns the root mapping node from a document or mapping node.
// If the document node has no children, an empty mapping node is initialized and attached.
func GetMappingNode(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			mapNode := &yaml.Node{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
			}
			root.Content = []*yaml.Node{mapNode}
			return mapNode
		}
		return root.Content[0]
	}
	if root.Kind == yaml.MappingNode {
		return root
	}
	return nil
}

// FindOrCreateMapping searches for a mapping key in mapNode.
// If the key exists, its child mapping node is returned (converting it to a mapping if necessary).
// If the key does not exist, a new key and mapping node are appended and returned.
func FindOrCreateMapping(mapNode *yaml.Node, key string) *yaml.Node {
	if mapNode == nil {
		return nil
	}
	for i := 0; i < len(mapNode.Content); i += 2 {
		k := mapNode.Content[i]
		if k.Value == key {
			val := mapNode.Content[i+1]
			if val.Kind != yaml.MappingNode {
				val.Kind = yaml.MappingNode
				val.Tag = "!!map"
				val.Content = nil
			}
			return val
		}
	}
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}
	childMap := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
	mapNode.Content = append(mapNode.Content, keyNode, childMap)
	return childMap
}

// FindSequence searches for a sequence node associated with the specified key in mapNode.
// Returns nil if key is not found or is not a sequence.
func FindSequence(mapNode *yaml.Node, key string) *yaml.Node {
	if mapNode == nil {
		return nil
	}
	for i := 0; i < len(mapNode.Content); i += 2 {
		k := mapNode.Content[i]
		if k.Value == key {
			val := mapNode.Content[i+1]
			if val.Kind == yaml.SequenceNode {
				return val
			}
			return nil
		}
	}
	return nil
}

// FindOrCreateSequence searches for a sequence associated with key in mapNode.
// If not found, a new SequenceNode is created, appended, and returned.
func FindOrCreateSequence(mapNode *yaml.Node, key string) *yaml.Node {
	if mapNode == nil {
		return nil
	}
	for i := 0; i < len(mapNode.Content); i += 2 {
		k := mapNode.Content[i]
		if k.Value == key {
			val := mapNode.Content[i+1]
			if val.Kind != yaml.SequenceNode {
				val.Kind = yaml.SequenceNode
				val.Tag = "!!seq"
				val.Content = nil
			}
			val.Style = 0
			return val
		}
	}
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}
	seqNode := &yaml.Node{
		Kind:  yaml.SequenceNode,
		Tag:   "!!seq",
		Style: 0,
	}
	mapNode.Content = append(mapNode.Content, keyNode, seqNode)
	return seqNode
}

// SetScalarValue updates or inserts a scalar value in mapNode for the given key.
// It preserves existing comments (HeadComment, LineComment, FootComment) on both key and value.
// If tag is empty, the appropriate tag (!!bool, !!int, !!str) is inferred from value.
func SetScalarValue(mapNode *yaml.Node, key, value, tag string) {
	if mapNode == nil {
		return
	}
	if tag == "" {
		if value == "true" || value == "false" {
			tag = "!!bool"
		} else if _, err := strconv.Atoi(value); err == nil {
			tag = "!!int"
		} else {
			tag = "!!str"
		}
	}

	for i := 0; i < len(mapNode.Content); i += 2 {
		if mapNode.Content[i].Value == key {
			val := mapNode.Content[i+1]
			val.Kind = yaml.ScalarNode
			val.Value = value
			val.Tag = tag
			return
		}
	}

	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}
	valNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   tag,
		Value: value,
	}
	mapNode.Content = append(mapNode.Content, keyNode, valNode)
}

// SetSequenceValues sets or replaces all string values in a sequence node under key,
// preserving the sequence node's own comments.
func SetSequenceValues(mapNode *yaml.Node, key string, values []string) {
	seqNode := FindOrCreateSequence(mapNode, key)
	seqNode.Kind = yaml.SequenceNode
	seqNode.Tag = "!!seq"
	seqNode.Style = 0

	seqNode.Content = make([]*yaml.Node, 0, len(values))
	for _, v := range values {
		seqNode.Content = append(seqNode.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: v,
		})
	}
}

// AppendUniqueToSequence appends val to seqNode if it is not already present.
// Returns true if val was appended, false if it was already in the sequence.
func AppendUniqueToSequence(seqNode *yaml.Node, val string) bool {
	if seqNode == nil {
		return false
	}
	for _, item := range seqNode.Content {
		if item.Value == val {
			return false
		}
	}
	seqNode.Content = append(seqNode.Content, &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: val,
	})
	return true
}

// RemoveFromSequence removes all occurrences of targets from seqNode.
// Returns the count of removed items.
func RemoveFromSequence(seqNode *yaml.Node, targets ...string) int {
	if seqNode == nil {
		return 0
	}
	targetSet := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		if t != "" {
			targetSet[t] = struct{}{}
		}
	}

	removed := 0
	newContent := make([]*yaml.Node, 0, len(seqNode.Content))
	for _, item := range seqNode.Content {
		if _, match := targetSet[item.Value]; match {
			removed++
		} else {
			newContent = append(newContent, item)
		}
	}
	seqNode.Content = newContent
	return removed
}
