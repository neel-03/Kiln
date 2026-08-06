// Package config implements Kiln's configuration schema definition, layer loading,
// and resolution (Brain of Kiln).

package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// SchemaProvider exposes configuration schema definitions.
type SchemaProvider interface {
	ConfigSchema() []Key
}

// coreSchemaProvider is the built-in schema provider.
type coreSchemaProvider struct{}

// ConfigSchema returns Kiln's built-in schema provider.
// It defines the default configuration schema for Kiln.
// Default config keys are:
//   - project.name
//   - kiln.target
//   - kiln.env
func (coreSchemaProvider) ConfigSchema() []Key {
	return []Key{
		{
			Namespace:   "project",
			Key:         "name",
			Type:        TypeString,
			Description: "Project name.",
		},
		{
			Namespace:   "kiln",
			Key:         "target",
			Type:        TypeString,
			Default:     "compose",
			Description: "Deployment target.",
			Enum: []string{
				"compose",
				"k8s",
				"systemd",
			},
		},
		{
			Namespace:   "kiln",
			Key:         "env",
			Type:        TypeString,
			Default:     "development",
			Description: "Active environment.",
		},
	}
}

// coreSchema returns Kiln's built-in schema provider.
func coreSchema() []Key {
	return coreSchemaProvider{}.ConfigSchema()
}

// ResolvedValue represents one resolved configuration value.
// It tracks not only the final value but also metadata about where the value came
// from (e.g. from user config vs CLI overrides) and whether it is secret or pending.
type ResolvedValue struct {
	Key           string
	Type          Type
	Value         any
	Source        LayerSource
	Secret        bool
	Pending       bool
	PendingReason string
}

// ResolvedConfig contains resolved configuration values.
type ResolvedConfig struct {
	values map[string]ResolvedValue
}

// Get returns a resolved value by key.
func (c ResolvedConfig) Get(key string) (ResolvedValue, bool) {
	v, ok := c.values[key]
	return v, ok
}

// Keys returns all resolved keys in sorted order.
func (c ResolvedConfig) Keys() []string {
	keys := slices.Collect(maps.Keys(c.values))
	slices.Sort(keys)
	return keys
}

// ResolveError aggregates configuration resolution failures.
type ResolveError struct {
	Errors []error
}

// Error returns a human-readable description of all errors
// encountered during configuration resolution.
func (e *ResolveError) Error() string {
	if len(e.Errors) == 0 {
		return ""
	}

	var b strings.Builder

	for i, err := range e.Errors {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(err.Error())
	}

	return b.String()
}

// Add adds a new error to the list of resolution failures.
func (e *ResolveError) Add(err error) {
	e.Errors = append(e.Errors, err)
}

// Append adds a new formatted configuration error to the list.
func (e *ResolveError) Append(format string, args ...any) {
	e.Errors = append(e.Errors, fmt.Errorf(format, args...))
}

// ErrorOrNil returns nil if no errors were recorded, otherwise it returns
// the aggregated error.
func (e *ResolveError) ErrorOrNil() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e
}

// Resolve merges configuration layers into one resolved configuration.
//
// Note: This Resolve function must remain side-effect-free forever — `.kiln/state.json`
// must never be read here, only during planning.
func Resolve(
	providers []SchemaProvider,
	layers []Layer,
) (ResolvedConfig, error) {
	errs := &ResolveError{}

	schema, errs := gatherSchemas(providers, errs)
	if err := errs.ErrorOrNil(); err != nil {
		return ResolvedConfig{}, err
	}

	resolved := seedDefaults(schema)
	resolved, errs = mergeLayers(schema, resolved, layers, errs)
	errs = checkCompleteness(schema, resolved, errs)

	if err := errs.ErrorOrNil(); err != nil {
		return ResolvedConfig{}, err
	}

	return resolved, nil
}

// gatherSchemas compiles a master registry of all configuration keys by merging
// the core schemas and all provider schemas, checking for duplicate keys.
func gatherSchemas(
	providers []SchemaProvider,
	errs *ResolveError,
) (map[string]Key, *ResolveError) {
	schema := make(map[string]Key)

	// Add built-in schema. it will always be present.
	for _, key := range coreSchema() {
		schema[key.FullName()] = key
	}

	// Add provider schemas.
	for _, provider := range providers {
		for _, key := range provider.ConfigSchema() {
			name := key.FullName()
			if _, exists := schema[name]; exists {
				errs.Append(
					`config key %q declared by more than one source`,
					name,
				)
				continue
			}
			schema[name] = key
		}
	}

	return schema, errs
}

// seedDefaults initializes a ResolvedConfig with default values for all known keys.
func seedDefaults(schema map[string]Key) ResolvedConfig {
	resolved := ResolvedConfig{
		values: make(map[string]ResolvedValue, len(schema)),
	}

	for _, key := range schema {
		resolved.values[key.FullName()] = ResolvedValue{
			Key:    key.FullName(),
			Type:   key.Type,
			Value:  key.Default,
			Secret: key.Secret,
			Source: LayerCoreDefaults, // Initially marked as originating from core defaults.
		}
	}

	return resolved
}

// mergeLayers sorts the configuration layers by precedence and merges them into
// the resolved config, performing validation checks on each layer's values.
func mergeLayers(
	schema map[string]Key,
	resolved ResolvedConfig,
	layers []Layer,
	errs *ResolveError,
) (ResolvedConfig, *ResolveError) {

	// Clone before sorting to preserve purity.
	sortedLayers := slices.Clone(layers)

	slices.SortFunc(sortedLayers, func(a, b Layer) int {
		// Sorting in ascending order of precedence
		// lower value = higher precedence
		return int(a.Source - b.Source)
	})

	for _, layer := range sortedLayers {
		for name, value := range layer.Values {
			key, ok := schema[name]
			if !ok {
				errs.Append(
					`config key %q is not declared by any loaded schema`,
					name,
				)
				continue
			}

			if err := key.Validate(value); err != nil {
				errs.Add(err)
				continue
			}

			current := resolved.values[name]
			current.Value = value
			current.Source = layer.Source

			resolved.values[name] = current
		}
	}

	return resolved, errs
}

// checkCompleteness verifies that every schema key has a non-nil value or is marked as pending.
func checkCompleteness(
	schema map[string]Key,
	resolved ResolvedConfig,
	errs *ResolveError,
) *ResolveError {
	for _, key := range schema {
		value := resolved.values[key.FullName()]

		if value.Pending {
			continue
		}

		if value.Value == nil && key.Default == nil {
			errs.Append(
				`config key %q has no default and was not set in any layer`,
				key.FullName(),
			)
		}
	}

	return errs
}
