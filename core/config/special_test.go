package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestTaggedValue_UnmarshalYAML verifies that TaggedValue decodes custom
// tags (!generate, !secret, !ref) and native types correctly, failing
// on unrecognized custom tags or invalid spec definitions.
func TestTaggedValue_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yamlStr string
		wantVal any
		wantErr string
	}{
		{
			name:    "plain string",
			yamlStr: `"hello"`,
			wantVal: "hello",
		},
		{
			name:    "plain integer",
			yamlStr: `123`,
			wantVal: 123,
		},
		{
			name:    "plain boolean",
			yamlStr: `true`,
			wantVal: true,
		},
		{
			name:    "!!str tag",
			yamlStr: `!!str hello`,
			wantVal: "hello",
		},
		{
			name:    "valid !generate tag",
			yamlStr: `!generate { length: 16 }`,
			wantVal: GenerateSpec{Length: 16},
		},
		{
			name:    "invalid !generate length zero",
			yamlStr: `!generate { length: 0 }`,
			wantErr: "invalid !generate length 0: expected a positive integer",
		},
		{
			name:    "invalid !generate length negative",
			yamlStr: `!generate { length: -5 }`,
			wantErr: "invalid !generate length -5: expected a positive integer",
		},
		{
			name:    "invalid !generate value type",
			yamlStr: `!generate { length: "foo" }`,
			wantErr: "invalid !generate value",
		},
		{
			name:    "valid !secret tag",
			yamlStr: `!secret my-secret-key`,
			wantVal: SecretRef{KeyName: "my-secret-key"},
		},
		{
			name:    "invalid !secret empty value",
			yamlStr: `!secret ""`,
			wantErr: "invalid !secret value: key name must not be empty",
		},
		{
			name:    "invalid !secret spaces only",
			yamlStr: `!secret "   "`,
			wantErr: "invalid !secret value: key name must not be empty",
		},
		{
			name:    "valid !ref tag",
			yamlStr: `!ref other.key`,
			wantVal: RefSpec{TargetKey: "other.key"},
		},
		{
			name:    "invalid !ref empty value",
			yamlStr: `!ref ""`,
			wantErr: "invalid !ref value: target key must not be empty",
		},
		{
			name:    "invalid !ref spaces only",
			yamlStr: `!ref "   "`,
			wantErr: "invalid !ref value: target key must not be empty",
		},
		{
			name:    "valid !secret tag with spaces to be trimmed",
			yamlStr: `!secret "  my-secret-key   "`,
			wantVal: SecretRef{KeyName: "my-secret-key"},
		},
		{
			name:    "valid !ref tag with spaces to be trimmed",
			yamlStr: `!ref "  other.key   "`,
			wantVal: RefSpec{TargetKey: "other.key"},
		},
		{
			name:    "unsupported custom tag",
			yamlStr: `!bogus value`,
			wantErr: `unsupported value tag "!bogus"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var tv TaggedValue
			err := yaml.Unmarshal([]byte(tc.yamlStr), &tv)

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tv.Value != tc.wantVal {
					t.Fatalf("expected value %+v, got %+v", tc.wantVal, tv.Value)
				}
				return
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}
