package manifest

import (
	"strings"
	"testing"
)

func TestLoadProjectManifest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		file        string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid manifest",
			file: "testdata/valid.kiln.yaml",
		},
		{
			name:        "unknown top level field",
			file:        "testdata/unknown-field.kiln.yaml",
			wantErr:     true,
			errContains: "field unknownField",
		},
		{
			name:        "unknown field nested inside service",
			file:        "testdata/unknown-service-field.kiln.yaml",
			wantErr:     true,
			errContains: "field depnds_on",
		},
		{
			name:        "malformed yaml",
			file:        "testdata/malformed.kiln.yaml",
			wantErr:     true,
			errContains: "testdata/malformed.kiln.yaml",
		},
		{
			name:        "multiple documents",
			file:        "testdata/multiple-documents.kiln.yaml",
			wantErr:     true,
			errContains: "unexpected additional YAML document",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manifest, err := LoadProjectManifest(tc.file)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}

				if !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf(
						"expected %q in error\nactual: %v",
						tc.errContains,
						err,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if manifest == nil {
				t.Fatal("manifest is nil")
			}

			assertValidManifest(t, manifest)
		})
	}
}

func assertValidManifest(t *testing.T, m *ProjectManifest) {
	t.Helper()

	if m.APIVersion != "kiln/v1" {
		t.Fatalf("unexpected apiVersion: %q", m.APIVersion)
	}

	if m.Kind != "Project" {
		t.Fatalf("unexpected kind: %q", m.Kind)
	}

	if m.Metadata.Name != "acme-storefront" {
		t.Fatalf("unexpected project name: %q", m.Metadata.Name)
	}

	if len(m.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(m.Plugins))
	}
	if m.Plugins[0].Path != "./plugins/postgres" {
		t.Fatalf("unexpected plugin[0].path: %q", m.Plugins[0].Path)
	}
	if m.Plugins[0].Module != "" {
		t.Fatalf("unexpected plugin[0].module: %q", m.Plugins[0].Module)
	}
	if m.Plugins[1].Path != "" {
		t.Fatalf("unexpected plugin[1].path: %q", m.Plugins[1].Path)
	}
	if m.Plugins[1].Module != "github.com/example/auth-plugin@v1.2.3" {
		t.Fatalf("unexpected plugin[1].module: %q", m.Plugins[1].Module)
	}

	if len(m.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(m.Services))
	}

	web, ok := m.Services["web"]
	if !ok {
		t.Fatal("service web missing")
	}

	if web.Image != "nginx:latest" {
		t.Fatalf("unexpected image: %q", web.Image)
	}

	if web.Replicas != "${web.replicas}" {
		t.Fatalf("unexpected replicas: %q", web.Replicas)
	}

	if len(web.Ports) != 2 {
		t.Fatalf("unexpected ports length: %d", len(web.Ports))
	}
	if web.Ports[0] != "80:80" || web.Ports[1] != "443:443" {
		t.Fatalf("unexpected ports: %v", web.Ports)
	}

	if len(web.Env) != 2 {
		t.Fatalf("unexpected env length: %d", len(web.Env))
	}
	if web.Env["APP_ENV"] != "production" || web.Env["LOG_LEVEL"] != "info" {
		t.Fatalf("unexpected env: %v", web.Env)
	}

	if len(web.Volumes) != 1 {
		t.Fatalf("unexpected volumes length: %d", len(web.Volumes))
	}
	if web.Volumes[0].Name != "web-data" || web.Volumes[0].Mount != "/var/www" || web.Volumes[0].Size != "${web.storage_gb}Gi" {
		t.Fatalf("unexpected volumes[0]: %+v", web.Volumes[0])
	}

	if len(web.DependsOn) != 1 {
		t.Fatalf("unexpected depends_on length: %d", len(web.DependsOn))
	}
	if web.DependsOn[0] != "postgres" {
		t.Fatalf("unexpected depends_on[0]: %q", web.DependsOn[0])
	}

	if web.HealthCheck == nil {
		t.Fatal("healthcheck missing")
	}
	if web.HealthCheck.HTTP != "http://localhost/health" || web.HealthCheck.Interval != "30s" || len(web.HealthCheck.Command) != 0 {
		t.Fatalf("unexpected healthcheck: %+v", web.HealthCheck)
	}

	postgres, ok := m.Services["postgres"]
	if !ok {
		t.Fatal("service postgres missing")
	}
	if postgres.Build == nil {
		t.Fatal("postgres build spec missing")
	}
	if postgres.Build.Context != "." || postgres.Build.Dockerfile != "Dockerfile" {
		t.Fatalf("unexpected postgres build context/dockerfile: %+v", postgres.Build)
	}
	if len(postgres.Build.Args) != 1 || postgres.Build.Args["VERSION"] != "16" {
		t.Fatalf("unexpected postgres build args: %v", postgres.Build.Args)
	}

	if len(m.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(m.Tasks))
	}

	initTask, ok := m.Tasks["init-db"]
	if !ok {
		t.Fatal("task init-db missing")
	}

	if initTask.RunsOn != "postgres" {
		t.Fatalf("unexpected runs_on: %q", initTask.RunsOn)
	}

	if len(initTask.Command) != 2 {
		t.Fatalf("unexpected command length: %d", len(initTask.Command))
	}
	if initTask.Command[0] != "migrate" || initTask.Command[1] != "up" {
		t.Fatalf("unexpected command: %v", initTask.Command)
	}

	if len(initTask.DependsOn) != 1 {
		t.Fatalf("unexpected task depends_on length: %d", len(initTask.DependsOn))
	}
	if initTask.DependsOn[0] != "postgres" {
		t.Fatalf("unexpected task depends_on: %q", initTask.DependsOn[0])
	}

	if initTask.Phase != "pre-init" {
		t.Fatalf("unexpected task phase: %q", initTask.Phase)
	}

	if initTask.When != "project.enabled" {
		t.Fatalf("unexpected task when: %q", initTask.When)
	}

	if !initTask.Idempotent {
		t.Fatal("expected task to be idempotent")
	}
}
