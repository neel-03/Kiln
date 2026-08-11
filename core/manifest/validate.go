// Package manifest handles parsing, representation, and structural validation
// of project manifests (kiln.yaml) and plugin manifests (plugin.yaml).
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/neel-03/Kiln/core"
)

// ValidationError aggregates all manifest validation failures.
//
// Validation intentionally accumulates every structural problem instead
// of failing fast so users receive a complete report in a single run.
type ValidationError struct {
	Errors []error
}

// Error returns a string representation of the validation errors, with one error message per line.
func (ve *ValidationError) Error() string {
	if len(ve.Errors) == 0 {
		return ""
	}

	var b strings.Builder

	for i, err := range ve.Errors {
		if i > 0 {
			b.WriteByte('\n')
		}

		b.WriteString(err.Error())
	}

	return b.String()
}

// Append adds a new formatted validation error to the list.
func (ve *ValidationError) Append(format string, args ...any) {
	ve.Errors = append(ve.Errors, fmt.Errorf(format, args...))
}

// Validate performs structural and cross-reference validation of a parsed manifest.
// The dir argument is the directory containing the manifest file, used to resolve local plugin paths.
// An empty dir skips filesystem checks.
func Validate(m *ProjectManifest, dir string) error {
	if m == nil {
		return fmt.Errorf("manifest cannot be nil")
	}

	validation := &ValidationError{}

	validateHeader(validation, m)
	validatePlugins(validation, m)
	validatePluginPaths(validation, m, dir)
	validateServices(validation, m)
	validateTasks(validation, m)

	return validation.ErrorOrNil()
}

// ErrorOrNil returns nil if there are no validation errors, or ve if there are.
func (ve *ValidationError) ErrorOrNil() error {
	if len(ve.Errors) == 0 {
		return nil
	}
	return ve
}

// validateHeader checks the top-level manifest metadata to make sure we're working
// with a supported schema version and that the project name matches our naming conventions.
// We expect apiVersion "kiln/v1", kind "Project", and metadata.name to match ^[a-z0-9][a-z0-9-]*$.
func validateHeader(ve *ValidationError, m *ProjectManifest) {
	if m.APIVersion != core.ExpectedAPIVersion {
		ve.Append(
			`apiVersion: unsupported value %q, expected %q`,
			m.APIVersion,
			core.ExpectedAPIVersion,
		)
	}

	if m.Kind != core.ExpectedKind {
		ve.Append(
			`kind: unsupported value %q, expected %q`,
			m.Kind,
			core.ExpectedKind,
		)
	}

	if !core.ProjectNamePattern.MatchString(m.Metadata.Name) {
		ve.Append(
			`metadata.name: must match pattern %s, got %q`,
			core.ProjectNameRegex,
			m.Metadata.Name,
		)
	}
}

// validatePlugins ensures that plugin references are structurally sound.
// A plugin reference must declare either a local directory path OR a remote module import,
// but not both (that would be ambiguous!). For remote modules, we also ensure they don't
// have an empty version string trailing after a '@'.
func validatePlugins(ve *ValidationError, m *ProjectManifest) {
	for i, plugin := range m.Plugins {

		if countSet(plugin.Path != "", plugin.Module != "") != 1 {
			ve.Append(
				`plugins[%d]: exactly one of "path" or "module" must be set`,
				i,
			)
			continue
		}

		if plugin.Module != "" {

			if idx := strings.LastIndex(plugin.Module, "@"); idx >= 0 {

				version := plugin.Module[idx+1:]

				if version == "" {
					ve.Append(
						`plugins[%d]: module %q has an empty version after "@"`,
						i,
						plugin.Module,
					)
				}
			}
		}
	}
}

// validatePluginPaths checks that local plugin directory paths exist on the filesystem.
// If dir is empty, the check is skipped.
func validatePluginPaths(ve *ValidationError, m *ProjectManifest, dir string) {
	if dir == "" {
		return
	}
	for i, plugin := range m.Plugins {
		if plugin.Path != "" && plugin.Module == "" {
			path := filepath.Join(dir, plugin.Path)
			info, err := os.Stat(path)
			if err != nil {
				ve.Append("plugins[%d]: path %q does not exist", i, plugin.Path)
			} else if !info.IsDir() {
				ve.Append("plugins[%d]: path %q is not a directory", i, plugin.Path)
			}
		}
	}
}

// validateServices verifies that service definitions are configured correctly.
// Each service must define exactly one way to build or fetch the service container:
// either from a local plugin reference ('from'), a container image ('image'), or a
// docker build configuration ('build').
// Additionally, if a healthcheck is specified, it must be either an HTTP path OR a shell command.
// It also verifies service dependencies.
func validateServices(ve *ValidationError, m *ProjectManifest) {
	for name, service := range m.Services {

		if countSet(service.From != "", service.Image != "", service.Build != nil) != 1 {
			ve.Append(
				`services.%s: exactly one of "from", "image", or "build" must be set`,
				name,
			)
		}

		if service.HealthCheck != nil {

			if countSet(service.HealthCheck.HTTP != "", len(service.HealthCheck.Command) > 0) != 1 {
				ve.Append(
					`services.%s.healthcheck: exactly one of "http" or "command" must be set`,
					name,
				)
			}
		}

		for _, dep := range service.DependsOn {
			if dep == name {
				ve.Append("services.%s.depends_on: service cannot depend on itself", name)
			} else if _, ok := m.Services[dep]; !ok {
				ve.Append("services.%s.depends_on: references unknown service %q", name, dep)
			}
		}
	}
}

// validateTasks checks that tasks are declared with a recognized execution phase
// (build, pre-init, init, post-init, runtime) so Kiln knows when to run them in the lifecycle.
// It also verifies runs_on service reference and depends_on dependencies.
func validateTasks(ve *ValidationError, m *ProjectManifest) {
	for name, task := range m.Tasks {

		if !slices.Contains(core.ValidPhases, task.Phase) {

			ve.Append(
				`tasks.%s.phase: invalid value %q, must be one of %s`,
				name,
				task.Phase,
				strings.Join(core.ValidPhases, ", "),
			)
		}

		if _, ok := m.Services[task.RunsOn]; !ok {
			ve.Append("tasks.%s.runs_on: references unknown service %q", name, task.RunsOn)
		}

		for _, dep := range task.DependsOn {
			// Note: This validation only checks that the referenced
			// task or service exists.
			idx := strings.LastIndex(dep, ".")
			isValid := false
			if idx >= 0 && dep[idx+1:] == "ready" {
				prefix := dep[:idx]
				_, isValid = m.Services[prefix]
			} else {
				_, isValid = m.Tasks[dep]
			}

			if !isValid {
				ve.Append("tasks.%s.depends_on: references unknown task or service %q", name, dep)
			}
		}
	}
}

// countSet returns the number of true values among the given conditions.
// Used to enforce "exactly one of N fields must be set" rules.
func countSet(conditions ...bool) int {
	n := 0
	for _, c := range conditions {
		if c {
			n++
		}
	}
	return n
}
