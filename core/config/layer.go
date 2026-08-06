package config

// LayerSource identifies where a configuration value originated.
//
// Layer precedence is ordered from lowest to highest.
//
// Later layers override values from earlier layers.
//
// The zero value intentionally represents the lowest-precedence layer so
// that an uninitialized Layer never accidentally wins during
// resolution.
type LayerSource int

const (
	// LayerCoreDefaults contains Kiln's built-in configuration defaults.
	LayerCoreDefaults LayerSource = iota

	// LayerPluginDefaults contains default values declared by loaded plugins.
	LayerPluginDefaults

	// LayerEnvironmentDefaults contains values loaded from an
	// environments/<env>.yaml file.
	LayerEnvironmentDefaults

	// LayerUserConfig contains values loaded from kiln.config.yaml.
	LayerUserConfig

	// LayerCLIOverride contains the highest-precedence values supplied via
	// CLI flags or KILN_* environment variables.
	LayerCLIOverride
)

// Layer represents one immutable configuration layer.
//
// Values are stored using fully-qualified dotted keys rather than nested
// maps.
//
// Example:
//
//	project.name
//	kiln.target
//
// Flattening nested YAML into dotted keys is intentionally performed by
// the caller (cmd/kiln). Resolve only operates on flat key/value pairs.
type Layer struct {
	Source LayerSource

	// Values contains configuration values indexed by their fully-qualified
	// dotted key.
	Values map[string]any
}
