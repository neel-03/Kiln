package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neel-03/Kiln/core/manifest"
)

func TestMain(m *testing.M) {
	// Set NO_COLOR=1 to ensure that most tests run with clean plain text formatting by default
	_ = os.Setenv("NO_COLOR", "1")
	os.Exit(m.Run())
}

// TestInitSuccess verifies that the init command successfully scaffolds a new project,
// creates all required files and directories, populates the project name correctly,
// and appends entries to .gitignore.
func TestInitSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	cmd := newInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing kiln init: %v", err)
	}

	expectedOutput := strings.Join([]string{
		"Created kiln.yaml",
		"Created kiln.config.yaml",
		"Created plugins/",
		"Next: edit kiln.config.yaml, then run `kiln doctor`\n",
	}, "\n")
	if buf.String() != expectedOutput {
		t.Errorf("expected output:\n%q\ngot:\n%q", expectedOutput, buf.String())
	}

	manifestPath := filepath.Join(tmpDir, "kiln.yaml")
	configPath := filepath.Join(tmpDir, "kiln.config.yaml")
	pluginsPath := filepath.Join(tmpDir, "plugins")
	gitkeepPath := filepath.Join(pluginsPath, ".gitkeep")
	gitignorePath := filepath.Join(tmpDir, ".gitignore")

	if _, err := os.Stat(manifestPath); err != nil {
		t.Errorf("kiln.yaml not created: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("kiln.config.yaml not created: %v", err)
	}
	if _, err := os.Stat(gitkeepPath); err != nil {
		t.Errorf("plugins/.gitkeep not created: %v", err)
	}
	if _, err := os.Stat(gitignorePath); err != nil {
		t.Errorf(".gitignore not created: %v", err)
	}

	loaded, err := manifest.LoadProjectManifest(manifestPath)
	if err != nil {
		t.Errorf("failed to load generated manifest: %v", err)
	}

	expectedName := sanitizeProjectName(filepath.Base(tmpDir))
	if loaded.Metadata.Name != expectedName {
		t.Errorf("expected project name %q, got %q", expectedName, loaded.Metadata.Name)
	}

	gitignoreContent, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	if !strings.Contains(string(gitignoreContent), ".kiln/\n") {
		t.Errorf("expected .gitignore to ignore .kiln/")
	}
	if !strings.Contains(string(gitignoreContent), "kiln.config.yaml\n") {
		t.Errorf("expected .gitignore to ignore kiln.config.yaml")
	}
}

// TestInitAlreadyExistsWithoutForce verifies that trying to scaffold in a directory
// where kiln.yaml already exists returns an overwrite prevention error when --force is not passed.
func TestInitAlreadyExistsWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	manifestPath := filepath.Join(tmpDir, "kiln.yaml")
	if err := os.WriteFile(manifestPath, []byte("exists"), 0644); err != nil {
		t.Fatalf("failed to write dummy kiln.yaml: %v", err)
	}

	cmd := newInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error running init when kiln.yaml already exists, but got nil")
	}

	expectedErr := "kiln.yaml already exists in this directory (use --force to overwrite)"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
	}
}

// TestInitAlreadyExistsWithForce verifies that scaffolding in an existing project
// succeeds and overwrites the config files when the --force flag is passed.
func TestInitAlreadyExistsWithForce(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	manifestPath := filepath.Join(tmpDir, "kiln.yaml")
	if err := os.WriteFile(manifestPath, []byte("exists"), 0644); err != nil {
		t.Fatalf("failed to write dummy kiln.yaml: %v", err)
	}

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--force"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error with --force: %v", err)
	}

	loaded, err := manifest.LoadProjectManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if loaded.Metadata.Name == "" {
		t.Errorf("expected name to be populated")
	}
}

// TestInitWithNameOverride verifies that passing an explicit --name flag overrides
// the default project name derived from the directory.
func TestInitWithNameOverride(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--name", "custom-project-name"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	manifestPath := filepath.Join(tmpDir, "kiln.yaml")
	loaded, err := manifest.LoadProjectManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if loaded.Metadata.Name != "custom-project-name" {
		t.Errorf("expected project name to be 'custom-project-name', got %q", loaded.Metadata.Name)
	}
}

// TestInitWithInvalidName verifies that passing an invalid project name
// via --name returns a validation error.
func TestInitWithInvalidName(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--name", "Invalid_Name!"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error with invalid project name, got nil")
	}

	expectedErr := "invalid --name \"Invalid_Name!\": must match pattern"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
	}
}

// TestInitWithInvalidTemplate verifies that requesting an unsupported template name
// returns a descriptive template validation error.
func TestInitWithInvalidTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--template", "unsupported-template-name"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error with invalid template, got nil")
	}

	expectedErr := "unsupported template \"unsupported-template-name\""
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
	}
}

// TestInitGitignoreExistingHandling verifies that we append missing entries
// to .gitignore without duplicating already existing entries.
func TestInitGitignoreExistingHandling(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	initialContent := "node_modules/\n.kiln/\n"
	if err := os.WriteFile(gitignorePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write dummy .gitignore: %v", err)
	}

	cmd := newInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gitignoreContent, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	lines := strings.Split(string(gitignoreContent), "\n")
	stateCount := 0
	configCount := 0
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == ".kiln/" {
			stateCount++
		}
		if trimmed == "kiln.config.yaml" {
			configCount++
		}
	}

	if stateCount != 1 {
		t.Errorf("expected '.kiln/' to appear exactly once, got count %d", stateCount)
	}
	if configCount != 1 {
		t.Errorf("expected 'kiln.config.yaml' to appear exactly once, got count %d", configCount)
	}
}

// TestInitSuccessColor verifies that the success output formatting contains correct ANSI checkmarks
// and colors when NO_COLOR is not set.
func TestInitSuccessColor(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	// Temporarily clear NO_COLOR to enable colors
	oldNoColor := os.Getenv("NO_COLOR")
	_ = os.Unsetenv("NO_COLOR")
	defer func() {
		if oldNoColor != "" {
			_ = os.Setenv("NO_COLOR", oldNoColor)
		}
	}()

	cmd := newInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outStr := buf.String()
	if !strings.Contains(outStr, "\033[32m✔\033[0m") {
		t.Errorf("expected output to contain green checkmark, got:\n%q", outStr)
	}
	if !strings.Contains(outStr, "\033[1;36mkiln.yaml\033[0m") {
		t.Errorf("expected output to contain bold cyan 'kiln.yaml', got:\n%q", outStr)
	}
}
