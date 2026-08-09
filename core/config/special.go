package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// ValueTagGenerate is the YAML tag for dynamic value generation.
	ValueTagGenerate string = "!generate"

	// ValueTagSecret is the YAML tag referencing a secret.
	ValueTagSecret string = "!secret"

	// ValueTagRef is the YAML tag referencing another configuration key.
	ValueTagRef string = "!ref"
)

// GenerateSpec describes a !generate configuration value.
type GenerateSpec struct {
	// Length specifies the character length of the generated value.
	Length int
}

// SecretRef describes a !secret reference to a named secret.
type SecretRef struct {
	// KeyName is the name of the secret in the external secret provider.
	KeyName string
}

// RefSpec describes a reference to another configuration key.
//
// !ref is fully resolved as it is a pure reference to
// already-loaded configuration and requires neither
// persistent state nor an external provider.
type RefSpec struct {
	// TargetKey is the fully-qualified dotted key name of the referenced configuration.
	TargetKey string
}

// TaggedValue is a YAML-decoded configuration value.
//
// It exists to bridge YAML's tagged-node representation and the flat
// map[string]any representation used by ConfigLayer. Callers that decode
// YAML layer values should decode each value through TaggedValue so that
// !generate, !secret, and !ref become their corresponding sentinel types.
//
// Ordinary YAML values are preserved as their normal representations.
type TaggedValue struct {
	// Value holds the decoded underlying value, which may be a custom spec type
	// (like GenerateSpec, SecretRef, RefSpec) or a standard type.
	Value any
}

// UnmarshalYAML implements custom decoding of YAML nodes to recognize custom Kiln tags.
func (v *TaggedValue) UnmarshalYAML(node *yaml.Node) error {
	switch node.Tag {
	case ValueTagGenerate:
		return v.unmarshalGenerateTag(node)
	case ValueTagSecret:
		return v.unmarshalSecretTag(node)
	case ValueTagRef:
		return v.unmarshalRefTag(node)
	}

	// YAML's built-in tags use the "!!" prefix, for example !!str and !!int.
	// They are normal YAML types and must not be rejected as unknown Kiln
	// custom tags.
	if strings.HasPrefix(node.Tag, "!") && !strings.HasPrefix(node.Tag, "!!") {
		return fmt.Errorf("unsupported value tag %q", node.Tag)
	}

	var value any

	if err := node.Decode(&value); err != nil {
		return err
	}

	v.Value = value

	return nil
}

// unmarshalGenerateTag decodes and validates a YAML node with the !generate tag.
func (v *TaggedValue) unmarshalGenerateTag(node *yaml.Node) error {
	var spec struct {
		Length int `yaml:"length"`
	}

	if err := node.Decode(&spec); err != nil {
		return fmt.Errorf("invalid %s value: %w", ValueTagGenerate, err)
	}

	if spec.Length <= 0 {
		return fmt.Errorf(
			"invalid %s length %d: expected a positive integer",
			ValueTagGenerate,
			spec.Length,
		)
	}

	v.Value = GenerateSpec{
		Length: spec.Length,
	}

	return nil
}

// unmarshalSecretTag decodes and validates a YAML node with the !secret tag.
func (v *TaggedValue) unmarshalSecretTag(node *yaml.Node) error {
	keyName, err := v.unmarshalStringTag(node, ValueTagSecret, "key name")
	if err != nil {
		return err
	}

	v.Value = SecretRef{
		KeyName: keyName,
	}

	return nil
}

// unmarshalRefTag decodes and validates a YAML node with the !ref tag.
func (v *TaggedValue) unmarshalRefTag(node *yaml.Node) error {
	targetKey, err := v.unmarshalStringTag(node, ValueTagRef, "target key")
	if err != nil {
		return err
	}

	v.Value = RefSpec{
		TargetKey: targetKey,
	}

	return nil
}

// unmarshalStringTag is a helper that decodes a YAML node into a string,
// validating that the string is not empty.
func (v *TaggedValue) unmarshalStringTag(node *yaml.Node, tag string, fieldName string) (string, error) {
	var val string
	if err := node.Decode(&val); err != nil {
		return "", fmt.Errorf("invalid %s value: %w", tag, err)
	}

	if strings.TrimSpace(val) == "" {
		return "", fmt.Errorf("invalid %s value: %s must not be empty", tag, fieldName)
	}

	return val, nil
}
