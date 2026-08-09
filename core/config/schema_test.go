package config

import (
	"strings"
	"testing"
	"time"
)

func floatPtr(v float64) *float64 { return &v }
func intPtr(v int) *int           { return &v }

func TestKeyValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     Key
		value   any
		wantErr string
	}{
		{
			name: "string valid",
			key: Key{
				Namespace: "project",
				Key:       "name",
				Type:      TypeString,
			},
			value: "kiln",
		},

		{
			name: "string wrong type",
			key: Key{
				Namespace: "project",
				Key:       "name",
				Type:      TypeString,
			},
			value:   123,
			wantErr: `project.name: expected string`,
		},

		{
			name: "string enum valid",
			key: Key{
				Namespace: "project",
				Key:       "mode",
				Type:      TypeString,
				Enum:      []string{"dev", "prod"},
			},
			value: "prod",
		},

		{
			name: "string enum invalid",
			key: Key{
				Namespace: "project",
				Key:       "mode",
				Type:      TypeString,
				Enum:      []string{"dev", "prod"},
			},
			value:   "stage",
			wantErr: `must be one of`,
		},

		{
			name: "pattern valid",
			key: Key{
				Namespace: "project",
				Key:       "slug",
				Type:      TypeString,
				Pattern:   `^[a-z-]+$`,
			},
			value: "acme-storefront",
		},

		{
			name: "pattern invalid",
			key: Key{
				Namespace: "project",
				Key:       "slug",
				Type:      TypeString,
				Pattern:   `^[a-z-]+$`,
			},
			value:   "Acme123",
			wantErr: "does not match pattern",
		},

		{
			name: "int valid",
			key: Key{
				Namespace: "postgres",
				Key:       "storage_gb",
				Type:      TypeInt,
				Min:       floatPtr(10),
				Max:       floatPtr(100),
			},
			value: 50,
		},

		{
			name: "int below minimum",
			key: Key{
				Namespace: "postgres",
				Key:       "storage_gb",
				Type:      TypeInt,
				Min:       floatPtr(10),
			},
			value:   5,
			wantErr: "below minimum",
		},

		{
			name: "int above maximum",
			key: Key{
				Namespace: "postgres",
				Key:       "storage_gb",
				Type:      TypeInt,
				Max:       floatPtr(20),
			},
			value:   50,
			wantErr: "exceeds maximum",
		},

		{
			name: "int wrong type",
			key: Key{
				Namespace: "postgres",
				Key:       "storage_gb",
				Type:      TypeInt,
			},
			value:   "50",
			wantErr: "expected int",
		},

		{
			name: "float valid",
			key: Key{
				Namespace: "limits",
				Key:       "cpu",
				Type:      TypeFloat,
			},
			value: 2.5,
		},

		{
			name: "float wrong type",
			key: Key{
				Namespace: "limits",
				Key:       "cpu",
				Type:      TypeFloat,
			},
			value:   2,
			wantErr: "expected float",
		},

		{
			name: "bool valid",
			key: Key{
				Namespace: "feature",
				Key:       "enabled",
				Type:      TypeBool,
			},
			value: true,
		},

		{
			name: "bool wrong type",
			key: Key{
				Namespace: "feature",
				Key:       "enabled",
				Type:      TypeBool,
			},
			value:   "true",
			wantErr: "expected bool",
		},

		{
			name: "duration valid",
			key: Key{
				Namespace: "jobs",
				Key:       "timeout",
				Type:      TypeDuration,
			},
			value: "30s",
		},

		{
			name: "duration valid as time.Duration",
			key: Key{
				Namespace: "jobs",
				Key:       "timeout",
				Type:      TypeDuration,
			},
			value: 30 * time.Second,
		},

		{
			name: "duration below minimum",
			key: Key{
				Namespace: "jobs",
				Key:       "timeout",
				Type:      TypeDuration,
				Min:       floatPtr(10),
			},
			value:   "5s",
			wantErr: "below minimum",
		},

		{
			name: "duration above maximum",
			key: Key{
				Namespace: "jobs",
				Key:       "timeout",
				Type:      TypeDuration,
				Max:       floatPtr(60),
			},
			value:   "2m",
			wantErr: "exceeds maximum",
		},

		{
			name: "duration invalid",
			key: Key{
				Namespace: "jobs",
				Key:       "timeout",
				Type:      TypeDuration,
			},
			value:   "forever",
			wantErr: "invalid duration",
		},

		{
			name: "duration wrong type",
			key: Key{
				Namespace: "jobs",
				Key:       "timeout",
				Type:      TypeDuration,
			},
			value:   30,
			wantErr: "expected duration",
		},

		{
			name: "list valid",
			key: Key{
				Namespace: "web",
				Key:       "hosts",
				Type:      TypeListString,
				MinItems:  intPtr(1),
				MaxItems:  intPtr(3),
			},
			value: []string{"a", "b"},
		},

		{
			name: "list too short",
			key: Key{
				Namespace: "web",
				Key:       "hosts",
				Type:      TypeListString,
				MinItems:  intPtr(2),
			},
			value:   []string{"a"},
			wantErr: "at least",
		},

		{
			name: "list too long",
			key: Key{
				Namespace: "web",
				Key:       "hosts",
				Type:      TypeListString,
				MaxItems:  intPtr(1),
			},
			value:   []string{"a", "b"},
			wantErr: "at most",
		},

		{
			name: "map valid",
			key: Key{
				Namespace: "env",
				Key:       "vars",
				Type:      TypeMapString,
			},
			value: map[string]string{
				"PORT": "8080",
			},
		},

		{
			name: "map wrong type",
			key: Key{
				Namespace: "env",
				Key:       "vars",
				Type:      TypeMapString,
			},
			value:   map[string]int{"PORT": 8080},
			wantErr: "expected map[string]string",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.key.Validate(tc.value)

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected error")
			}

			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected %q in %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestKeyFullName(t *testing.T) {
	t.Parallel()

	key := Key{
		Namespace: "postgres",
		Key:       "storage_gb",
	}

	if got := key.FullName(); got != "postgres.storage_gb" {
		t.Fatalf("expected postgres.storage_gb, got %q", got)
	}
}
