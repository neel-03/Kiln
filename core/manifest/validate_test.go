package manifest

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		modify      func(*ProjectManifest)
		wantErr     bool
		errContains []string
	}{
		{
			name: "valid manifest",
		},

		{
			name: "invalid api version",
			modify: func(m *ProjectManifest) {
				m.APIVersion = "v2"
			},
			wantErr: true,
			errContains: []string{
				`apiVersion: unsupported value "v2", expected "kiln/v1"`,
			},
		},

		{
			name: "invalid kind",
			modify: func(m *ProjectManifest) {
				m.Kind = "Application"
			},
			wantErr: true,
			errContains: []string{
				`kind: unsupported value "Application", expected "Project"`,
			},
		},

		{
			name: "invalid project name",
			modify: func(m *ProjectManifest) {
				m.Metadata.Name = "My Project"
			},
			wantErr: true,
			errContains: []string{
				`metadata.name: must match pattern`,
			},
		},

		{
			name: "plugin path and module",
			modify: func(m *ProjectManifest) {
				m.Plugins = []PluginRef{
					{
						Path:   "./plugins/test",
						Module: "github.com/example/plugin",
					},
				}
			},
			wantErr: true,
			errContains: []string{
				`plugins[0]: exactly one of "path" or "module" must be set`,
			},
		},

		{
			name: "empty module version",
			modify: func(m *ProjectManifest) {
				m.Plugins = []PluginRef{
					{
						Module: "github.com/example/plugin@",
					},
				}
			},
			wantErr: true,
			errContains: []string{
				`plugins[0]: module "github.com/example/plugin@" has an empty version after "@"`,
			},
		},

		{
			name: "service source conflict",
			modify: func(m *ProjectManifest) {
				s := m.Services["web"]
				s.From = "base"
				m.Services["web"] = s
			},
			wantErr: true,
			errContains: []string{
				`services.web: exactly one of "from", "image", or "build" must be set`,
			},
		},

		{
			name: "healthcheck conflict",
			modify: func(m *ProjectManifest) {
				s := m.Services["web"]

				s.HealthCheck.Command = []string{"curl"}

				m.Services["web"] = s
			},
			wantErr: true,
			errContains: []string{
				`services.web.healthcheck: exactly one of "http" or "command" must be set`,
			},
		},

		{
			name: "invalid phase",
			modify: func(m *ProjectManifest) {
				task := m.Tasks["init-db"]
				task.Phase = "startup"
				m.Tasks["init-db"] = task
			},
			wantErr: true,
			errContains: []string{
				`tasks.init-db.phase: invalid value "startup"`,
			},
		},
	}

	for _, tc := range tests {

		tc := tc

		t.Run(tc.name, func(t *testing.T) {

			t.Parallel()

			m, err := LoadProjectManifest("testdata/valid.kiln.yaml")
			if err != nil {
				t.Fatal(err)
			}

			if tc.modify != nil {
				tc.modify(m)
			}

			err = Validate(m)

			if tc.wantErr {

				if err == nil {
					t.Fatal("expected validation error")
				}

				for _, expected := range tc.errContains {

					if !strings.Contains(err.Error(), expected) {
						t.Fatalf(
							"\nexpected:\n%s\n\nactual:\n%s",
							expected,
							err.Error(),
						)
					}
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected validation error:\n%s", err)
			}
		})
	}
}

func TestValidationAccumulatesErrors(t *testing.T) {

	m, err := LoadProjectManifest("testdata/valid.kiln.yaml")
	if err != nil {
		t.Fatal(err)
	}

	m.APIVersion = "wrong"

	m.Kind = "Wrong"

	m.Metadata.Name = "INVALID"

	err = Validate(m)

	if err == nil {
		t.Fatal("expected validation error")
	}

	msg := err.Error()

	expected := []string{
		`apiVersion: unsupported value`,
		`kind: unsupported value`,
		`metadata.name`,
	}

	for _, s := range expected {

		if !strings.Contains(msg, s) {
			t.Fatalf("missing %q", s)
		}
	}
}

func TestValidateNilManifest(t *testing.T) {

	if err := Validate(nil); err == nil {
		t.Fatal("expected error")
	}
}
