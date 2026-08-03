package manifest

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// ProjectManifest represents the contents of a kiln.yaml project manifest.
//
// Currently intentionally supports only parsing and structural decoding.
// TODO: Rendering, plugin loading, template expansion, and execution.
type ProjectManifest struct {
	// APIVersion is the version of the manifest schema (must be "kiln/v1").
	APIVersion string `yaml:"apiVersion"`
	// Kind is the type of the manifest resource (must be "Project").
	Kind string `yaml:"kind"`
	// Metadata contains metadata about the project, such as the name.
	Metadata ProjectMetadata `yaml:"metadata"`
	// Plugins is a list of references to plugins used by the project.
	Plugins []PluginRef `yaml:"plugins,omitempty"`
	// Services is a map of service specifications, keyed by service name.
	Services map[string]ServiceSpec `yaml:"services,omitempty"`
	// Tasks is a map of task specifications, keyed by task name.
	Tasks map[string]TaskSpec `yaml:"tasks,omitempty"`
}

// ProjectMetadata describes the project's metadata block.
type ProjectMetadata struct {
	// Name is the name of the project.
	Name string `yaml:"name"`
}

// PluginRef references either a local plugin directory or a remote module.
//
// Validation ensuring exactly one of Path or Module is set happens during
// manifest validation.
type PluginRef struct {
	// Path is the file system path to a local plugin directory.
	Path string `yaml:"path,omitempty"`
	// Module is the remote module path (e.g. github.com/user/plugin@v1.0.0).
	Module string `yaml:"module,omitempty"`
}

// ServiceSpec describes one service defined in kiln.yaml.
type ServiceSpec struct {
	// From indicates the plugin source this service is instantiated from.
	From string `yaml:"from,omitempty"`
	// Image is the container image to run (e.g. nginx:latest).
	Image string `yaml:"image,omitempty"`
	// Build details the container build configuration if building from source.
	Build *BuildSpecYAML `yaml:"build,omitempty"`
	// Replicas is the number of replicas to run.
	//
	// Note: Fields like replicas: ${web.replicas} interleave literal text with
	// ${...} config references. This field is typed as a plain string and stored raw;
	// interpolation is performed at render time.
	Replicas string `yaml:"replicas,omitempty"`
	// Env specifies the environment variables for the service.
	Env map[string]string `yaml:"env,omitempty"`
	// Ports defines the port mappings exposed by the service.
	Ports []string `yaml:"ports,omitempty"`
	// Volumes lists the volume specifications mounted by the service.
	Volumes []VolumeSpec `yaml:"volumes,omitempty"`
	// DependsOn specifies the services this service depends on.
	DependsOn []string `yaml:"depends_on,omitempty"`
	// HealthCheck describes the health check configuration for the service.
	HealthCheck *HealthCheckSpec `yaml:"healthcheck,omitempty"`
}

// TaskSpec describes one task declared in kiln.yaml.
type TaskSpec struct {
	// RunsOn is the service in whose environment/container context the task runs.
	RunsOn string `yaml:"runs_on"`
	// Command is the list of command line arguments to run.
	Command []string `yaml:"command,omitempty"`
	// DependsOn specifies other tasks or service readiness states this task depends on.
	DependsOn []string `yaml:"depends_on,omitempty"`
	// Phase is the execution phase for the task (e.g., build, pre-init, init, post-init, runtime).
	Phase string `yaml:"phase,omitempty"`
	// When is a CEL condition that controls whether the task runs.
	When string `yaml:"when,omitempty"`
	// Idempotent indicates whether this task can be safely re-run multiple times.
	Idempotent bool `yaml:"idempotent,omitempty"`
}

// BuildSpecYAML describes a container build.
type BuildSpecYAML struct {
	// Context is the build context directory.
	Context string `yaml:"context,omitempty"`
	// Dockerfile is the path to the Dockerfile relative to the context.
	Dockerfile string `yaml:"dockerfile,omitempty"`
	// Args specifies build-time arguments.
	Args map[string]string `yaml:"args,omitempty"`
}

// VolumeSpec describes a service volume.
type VolumeSpec struct {
	// Name is the name of the volume.
	Name string `yaml:"name"`
	// Mount is the target path where the volume is mounted inside the container.
	Mount string `yaml:"mount"`
	// Size is the storage capacity allocated for the volume.
	//
	// Note: Fields like size: ${postgres.storage_gb}Gi interleave literal text with
	// ${...} config references. This field is typed as a plain string and stored raw;
	// interpolation is performed at render time.
	Size string `yaml:"size,omitempty"`
}

// HealthCheckSpec defines a service health check.
type HealthCheckSpec struct {
	// HTTP is the HTTP endpoint path to query for health.
	HTTP string `yaml:"http,omitempty"`
	// Command is the shell command to run to assess health.
	Command []string `yaml:"command,omitempty"`
	// Interval is the duration between health checks (e.g., 30s).
	Interval string `yaml:"interval,omitempty"`
}

// LoadProjectManifest loads and decodes a kiln.yaml manifest.
//
// Decoding is intentionally strict:
//
//   - unknown fields are rejected
//   - malformed YAML is rejected
//
// Validation of semantic correctness is handled separately by Validate()
func LoadProjectManifest(path string) (*ProjectManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading manifest %s: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var manifest ProjectManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("loading manifest %s: %w", path, err)
	}

	// Reject trailing YAML documents or content to guarantee strict single-document loading.
	if err := decoder.Decode(new(yaml.Node)); err != io.EOF {
		return nil, fmt.Errorf("loading manifest %s: unexpected additional YAML document", path)
	}

	return &manifest, nil
}
