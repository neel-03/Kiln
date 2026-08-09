package config

import (
	"errors"
	"strings"
	"testing"
)

type mockSchemaProvider struct {
	keys []Key
}

func (m mockSchemaProvider) ConfigSchema() []Key {
	return m.keys
}

func TestResolve_Success(t *testing.T) {
	t.Parallel()

	// 1. Zero providers beyond the core one, resolves core keys correctly
	providers := []SchemaProvider{}
	layers := []Layer{
		{
			Source: LayerUserConfig,
			Values: map[string]any{
				"project.name": "kiln-test",
			},
		},
	}

	cfg, err := Resolve(providers, layers)
	if err != nil {
		t.Fatalf("unexpected error resolving with core schema: %v", err)
	}

	// Verify core defaults
	if name, ok := cfg.Get("project.name"); !ok || name.Value != "kiln-test" || name.Source != LayerUserConfig {
		t.Errorf("project.name: expected 'kiln-test' from LayerUserConfig, got %+v (ok=%t)", name, ok)
	}
	if target, ok := cfg.Get("kiln.target"); !ok || target.Value != "compose" || target.Source != LayerCoreDefaults {
		t.Errorf("kiln.target: expected 'compose' from LayerCoreDefaults, got %+v (ok=%t)", target, ok)
	}
	if env, ok := cfg.Get("kiln.env"); !ok || env.Value != "development" || env.Source != LayerCoreDefaults {
		t.Errorf("kiln.env: expected 'development' from LayerCoreDefaults, got %+v (ok=%t)", env, ok)
	}

	// 2. Custom schema provider and multiple layers overriding keys
	customProvider := mockSchemaProvider{
		keys: []Key{
			{
				Namespace: "db",
				Key:       "host",
				Type:      TypeString,
			},
			{
				Namespace: "db",
				Key:       "port",
				Type:      TypeInt,
				Default:   5432,
			},
		},
	}

	providers = []SchemaProvider{customProvider}
	layers = []Layer{
		{
			Source: LayerUserConfig,
			Values: map[string]any{
				"project.name": "custom-proj",
				"db.host":      "prod-db",
			},
		},
		{
			Source: LayerEnvironmentDefaults,
			Values: map[string]any{
				"db.host": "dev-db",
				"db.port": 5433,
			},
		},
	}

	cfg, err = Resolve(providers, layers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check precedence: LayerUserConfig (precedence 3) overrides LayerEnvironmentDefaults (precedence 2)
	if host, ok := cfg.Get("db.host"); !ok || host.Value != "prod-db" || host.Source != LayerUserConfig {
		t.Errorf("db.host: expected 'prod-db' from LayerUserConfig, got %+v (ok=%t)", host, ok)
	}
	// Check that LayerEnvironmentDefaults overrides defaults
	if port, ok := cfg.Get("db.port"); !ok || port.Value != 5433 || port.Source != LayerEnvironmentDefaults {
		t.Errorf("db.port: expected 5433 from LayerEnvironmentDefaults, got %+v (ok=%t)", port, ok)
	}
}

func TestResolve_PrecedenceFiveLayers(t *testing.T) {
	t.Parallel()

	// Test 5 layers overriding the same key, asserting the winning layer is the highest precedence one.
	customProvider := mockSchemaProvider{
		keys: []Key{
			{
				Namespace: "app",
				Key:       "val",
				Type:      TypeString,
				Default:   "default-val",
			},
		},
	}

	providers := []SchemaProvider{customProvider}

	tests := []struct {
		name           string
		layers         []Layer
		expectedVal    any
		expectedSource LayerSource
	}{
		{
			name: "core defaults wins if no layers",
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
					},
				},
			},
			expectedVal:    "default-val",
			expectedSource: LayerCoreDefaults,
		},
		{
			name: "plugin defaults overrides core defaults",
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
					},
				},
				{
					Source: LayerPluginDefaults,
					Values: map[string]any{
						"app.val": "plugin-val",
					},
				},
			},
			expectedVal:    "plugin-val",
			expectedSource: LayerPluginDefaults,
		},
		{
			name: "env defaults overrides plugin defaults",
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
					},
				},
				{
					Source: LayerPluginDefaults,
					Values: map[string]any{
						"app.val": "plugin-val",
					},
				},
				{
					Source: LayerEnvironmentDefaults,
					Values: map[string]any{
						"app.val": "env-val",
					},
				},
			},
			expectedVal:    "env-val",
			expectedSource: LayerEnvironmentDefaults,
		},
		{
			name: "user config overrides env defaults",
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
						"app.val":      "user-val",
					},
				},
				{
					Source: LayerPluginDefaults,
					Values: map[string]any{
						"app.val": "plugin-val",
					},
				},
				{
					Source: LayerEnvironmentDefaults,
					Values: map[string]any{
						"app.val": "env-val",
					},
				},
			},
			expectedVal:    "user-val",
			expectedSource: LayerUserConfig,
		},
		{
			name: "CLI override overrides user config (all 5 layers)",
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
						"app.val":      "user-val",
					},
				},
				{
					Source: LayerPluginDefaults,
					Values: map[string]any{
						"app.val": "plugin-val",
					},
				},
				{
					Source: LayerEnvironmentDefaults,
					Values: map[string]any{
						"app.val": "env-val",
					},
				},
				{
					Source: LayerCLIOverride,
					Values: map[string]any{
						"app.val": "cli-val",
					},
				},
			},
			expectedVal:    "cli-val",
			expectedSource: LayerCLIOverride,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Resolve(providers, tc.layers)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			val, ok := cfg.Get("app.val")
			if !ok {
				t.Fatalf("key app.val not found in resolved config")
			}
			if val.Value != tc.expectedVal {
				t.Errorf("expected value %v, got %v", tc.expectedVal, val.Value)
			}
			if val.Source != tc.expectedSource {
				t.Errorf("expected source %v, got %v", tc.expectedSource, val.Source)
			}
		})
	}
}

func TestResolve_SecretPreservation(t *testing.T) {
	t.Parallel()

	customProvider := mockSchemaProvider{
		keys: []Key{
			{
				Namespace: "db",
				Key:       "password",
				Type:      TypeString,
				Secret:    true,
			},
		},
	}

	providers := []SchemaProvider{customProvider}
	layers := []Layer{
		{
			Source: LayerUserConfig,
			Values: map[string]any{
				"project.name": "test",
				"db.password":  "super-secret",
			},
		},
	}

	cfg, err := Resolve(providers, layers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, ok := cfg.Get("db.password")
	if !ok {
		t.Fatalf("db.password not found in resolved config")
	}

	if !val.Secret {
		t.Errorf("expected db.password to be marked as secret, got %+v", val)
	}
}

func TestCheckCompleteness_PendingIgnored(t *testing.T) {
	t.Parallel()

	schema := map[string]Key{
		"db.host": {
			Namespace: "db",
			Key:       "host",
			Type:      TypeString, // no default, required
		},
	}

	// 1. When it's not pending and has no value/default -> should error
	resolvedWithoutPending := ResolvedConfig{
		values: map[string]ResolvedValue{
			"db.host": {
				Key:     "db.host",
				Type:    TypeString,
				Value:   nil,
				Pending: false,
			},
		},
	}
	errs := &ResolveError{}
	errs = checkCompleteness(schema, resolvedWithoutPending, errs)
	if errs.ErrorOrNil() == nil {
		t.Errorf("expected completeness check to fail for non-pending key with nil value")
	}

	// 2. When it is pending -> should NOT error
	resolvedWithPending := ResolvedConfig{
		values: map[string]ResolvedValue{
			"db.host": {
				Key:     "db.host",
				Type:    TypeString,
				Value:   nil,
				Pending: true,
			},
		},
	}
	errs = &ResolveError{}
	errs = checkCompleteness(schema, resolvedWithPending, errs)
	if err := errs.ErrorOrNil(); err != nil {
		t.Errorf("expected completeness check to pass for pending key: %v", err)
	}
}

func TestResolve_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		providers []SchemaProvider
		layers    []Layer
		wantErr   string
	}{
		{
			name: "duplicate schema keys",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{Namespace: "custom", Key: "key", Type: TypeString},
					},
				},
				mockSchemaProvider{
					keys: []Key{
						{Namespace: "custom", Key: "key", Type: TypeInt},
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
					},
				},
			},
			wantErr: `config key "custom.key" declared by more than one source`,
		},
		{
			name:      "undeclared key in layer",
			providers: []SchemaProvider{},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name":   "test",
						"undeclared.key": "val",
					},
				},
			},
			wantErr: `config key "undeclared.key" is not declared by any loaded schema`,
		},
		{
			name: "validation failure wrong type",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{Namespace: "db", Key: "port", Type: TypeInt},
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
						"db.port":      "not-an-int",
					},
				},
			},
			wantErr: `db.port: expected int, got string "not-an-int"`,
		},
		{
			name: "required key missing",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{Namespace: "db", Key: "host", Type: TypeString}, // no default
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
					},
				},
			},
			wantErr: `config key "db.host" has no default and was not set in any layer`,
		},
		{
			name: "invalid default type",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{Namespace: "custom", Key: "key", Type: TypeInt, Default: "not-an-int"},
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
					},
				},
			},
			wantErr: `config default for "custom.key": custom.key: expected int, got string "not-an-int"`,
		},
		{
			name: "invalid default constraint",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{
							Namespace: "custom",
							Key:       "key",
							Type:      TypeInt,
							Min:       floatPtr(10),
							Default:   5,
						},
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
					},
				},
			},
			wantErr: `config default for "custom.key": custom.key: value 5 is below minimum 10`,
		},
		{
			name: "secret enum value redaction",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{
							Namespace: "db",
							Key:       "mode",
							Type:      TypeString,
							Enum:      []string{"dev", "prod"},
							Secret:    true,
						},
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
						"db.mode":      "sensitive-stage",
					},
				},
			},
			wantErr: `db.mode: value "<redacted>" must be one of [dev, prod]`,
		},
		{
			name: "secret pattern value redaction",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{
							Namespace: "db",
							Key:       "slug",
							Type:      TypeString,
							Pattern:   `^[a-z]+$`,
							Secret:    true,
						},
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
						"db.slug":      "Secret123",
					},
				},
			},
			wantErr: `db.slug: value "<redacted>" does not match pattern "^[a-z]+$"`,
		},
		{
			name: "secret numeric limit value redaction",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{
							Namespace: "db",
							Key:       "port",
							Type:      TypeInt,
							Min:       floatPtr(1024),
							Secret:    true,
						},
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
						"db.port":      80,
					},
				},
			},
			wantErr: `db.port: value <redacted> is below minimum 1024`,
		},
		{
			name: "secret type mismatch value redaction",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{
							Namespace: "db",
							Key:       "port",
							Type:      TypeInt,
							Secret:    true,
						},
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
						"db.port":      "eighty",
					},
				},
			},
			wantErr: `db.port: expected int, got string "<redacted>"`,
		},
		{
			name: "secret duration parse failure redaction",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{
							Namespace: "db",
							Key:       "timeout",
							Type:      TypeDuration,
							Secret:    true,
						},
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
						"db.timeout":   "s3cr3t-token",
					},
				},
			},
			wantErr: `db.timeout: invalid duration "<redacted>"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Resolve(tc.providers, tc.layers)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestResolvedConfig_GetAndKeys(t *testing.T) {
	t.Parallel()

	values := map[string]ResolvedValue{
		"z.key": {Key: "z.key", Value: "z"},
		"a.key": {Key: "a.key", Value: "a"},
		"m.key": {Key: "m.key", Value: "m"},
	}
	cfg := ResolvedConfig{values: values}

	// Test Get
	val, ok := cfg.Get("a.key")
	if !ok || val.Value != "a" {
		t.Errorf("Get('a.key') expected ('a', true), got (%v, %t)", val.Value, ok)
	}

	_, ok = cfg.Get("missing.key")
	if ok {
		t.Errorf("Get('missing.key') expected false, got true")
	}

	// Test Keys (should be sorted)
	keys := cfg.Keys()
	expected := []string{"a.key", "m.key", "z.key"}
	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("at index %d: expected %q, got %q", i, expected[i], k)
		}
	}
}

func TestResolveError(t *testing.T) {
	t.Parallel()

	errs := &ResolveError{}
	if err := errs.ErrorOrNil(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if msg := errs.Error(); msg != "" {
		t.Fatalf("expected empty error message, got %q", msg)
	}

	errs.Add(errors.New("error 1"))
	errs.Append("error %d", 2)

	if err := errs.ErrorOrNil(); err == nil {
		t.Fatal("expected non-nil error")
	}

	expectedMsg := "error 1\nerror 2"
	if msg := errs.Error(); msg != expectedMsg {
		t.Errorf("expected %q, got %q", expectedMsg, msg)
	}
}

func TestResolve_SpecialForms(t *testing.T) {
	t.Parallel()

	customProvider := mockSchemaProvider{
		keys: []Key{
			{
				Namespace: "db",
				Key:       "host",
				Type:      TypeString,
			},
			{
				Namespace: "db",
				Key:       "alias",
				Type:      TypeString,
			},
			{
				Namespace: "db",
				Key:       "password",
				Type:      TypeString,
			},
			{
				Namespace: "db",
				Key:       "token",
				Type:      TypeString,
			},
		},
	}

	providers := []SchemaProvider{customProvider}
	layers := []Layer{
		{
			Source: LayerUserConfig,
			Values: map[string]any{
				"project.name": "test",
				"db.host":      "localhost",
				"db.alias":     RefSpec{TargetKey: "db.host"},
				"db.password":  GenerateSpec{Length: 16},
				"db.token":     SecretRef{KeyName: "my-token"},
			},
		},
	}

	cfg, err := Resolve(providers, layers)
	if err != nil {
		t.Fatalf("unexpected error resolving special forms: %v", err)
	}

	// Verify !ref resolves
	alias, ok := cfg.Get("db.alias")
	if !ok {
		t.Fatalf("db.alias not found")
	}
	if alias.Value != "localhost" || alias.Source != LayerUserConfig || alias.Type != TypeString {
		t.Errorf("expected db.alias to resolve to localhost, got %+v", alias)
	}

	// Verify !generate is Pending
	pwd, ok := cfg.Get("db.password")
	if !ok {
		t.Fatalf("db.password not found")
	}
	if pwd.Value != nil || !pwd.Pending || pwd.PendingReason != "value is !generate; requires .kiln/state.json, which is not yet available" {
		t.Errorf("expected db.password to be pending generate, got %+v", pwd)
	}

	// Verify !secret is Pending and Secret is true
	tok, ok := cfg.Get("db.token")
	if !ok {
		t.Fatalf("db.token not found")
	}
	if tok.Value != nil || !tok.Pending || !tok.Secret || tok.PendingReason != "value is !secret; requires a SecretProvider, which is not yet configured" {
		t.Errorf("expected db.token to be pending secret, got %+v", tok)
	}
}

func TestResolve_SpecialFormsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		providers []SchemaProvider
		layers    []Layer
		wantErr   string
	}{
		{
			name: "ref to unknown key",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{Namespace: "db", Key: "host", Type: TypeString},
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
						"db.host":      RefSpec{TargetKey: "db.nonexistent"},
					},
				},
			},
			wantErr: `config key "db.host": !ref target "db.nonexistent" does not exist`,
		},
		{
			name: "ref cycle of length 2",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{Namespace: "db", Key: "a", Type: TypeString},
						{Namespace: "db", Key: "b", Type: TypeString},
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
						"db.a":         RefSpec{TargetKey: "db.b"},
						"db.b":         RefSpec{TargetKey: "db.a"},
					},
				},
			},
			wantErr: `config key "db.a": !ref cycle detected (db.a -> db.b -> db.a)`,
		},
		{
			name: "ref cycle of length 3",
			providers: []SchemaProvider{
				mockSchemaProvider{
					keys: []Key{
						{Namespace: "db", Key: "a", Type: TypeString},
						{Namespace: "db", Key: "b", Type: TypeString},
						{Namespace: "db", Key: "c", Type: TypeString},
					},
				},
			},
			layers: []Layer{
				{
					Source: LayerUserConfig,
					Values: map[string]any{
						"project.name": "test",
						"db.a":         RefSpec{TargetKey: "db.b"},
						"db.b":         RefSpec{TargetKey: "db.c"},
						"db.c":         RefSpec{TargetKey: "db.a"},
					},
				},
			},
			wantErr: `config key "db.a": !ref cycle detected (db.a -> db.b -> db.c -> db.a)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Resolve(tc.providers, tc.layers)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}
