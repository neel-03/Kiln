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

	resolved, errs := seedDefaults(schema, errs)
	resolved, errs = mergeLayers(schema, resolved, layers, errs)
	resolved, errs = resolveSpecialForms(schema, resolved, errs)

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
func seedDefaults(schema map[string]Key, errs *ResolveError) (ResolvedConfig, *ResolveError) {
	resolved := ResolvedConfig{
		values: make(map[string]ResolvedValue, len(schema)),
	}

	for _, key := range schema {
		if key.Default != nil {
			if !isSpecialForm(key.Default) {
				if err := key.Validate(key.Default); err != nil {
					errs.Add(fmt.Errorf("config default for %q: %w", key.FullName(), err))
				}
			}
		}
		resolved.values[key.FullName()] = ResolvedValue{
			Key:    key.FullName(),
			Type:   key.Type,
			Value:  key.Default,
			Secret: key.Secret,
			Source: LayerCoreDefaults, // Initially marked as originating from core defaults.
		}
	}

	return resolved, errs
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
		// higher value = higher precedence
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

			if !isSpecialForm(value) {
				if err := key.Validate(value); err != nil {
					errs.Add(err)
					continue
				}
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

// refResolver resolves cyclic or nested !ref configuration references recursively.
type refResolver struct {
	// resolved holds the configuration state with resolved values.
	resolved ResolvedConfig

	// schema contains the master configuration key definitions.
	schema map[string]Key

	// errs accumulates any cycle or unknown target errors.
	errs *ResolveError

	// cache stores fully resolved values to avoid redundant resolutions.
	cache map[string]ResolvedValue
}

// resolveKey recursively resolves the configuration key keyName,
// tracking the traversal path to detect reference cycles.
func (r *refResolver) resolveKey(keyName string, path []string) (ResolvedValue, bool) {
	if cv, ok := r.cache[keyName]; ok {
		return cv, true
	}

	val, exists := r.resolved.values[keyName]
	if !exists {
		return ResolvedValue{}, false
	}

	ref := val.Value.(RefSpec)

	if r.detectCycle(keyName, path) {
		return ResolvedValue{}, false
	}

	nextPath := make([]string, len(path)+1)
	copy(nextPath, path)
	nextPath[len(path)] = keyName
	var targetVal ResolvedValue

	tVal, targetExists := r.resolved.values[ref.TargetKey]
	if !targetExists {
		if _, schemaExists := r.schema[ref.TargetKey]; !schemaExists {
			r.errs.Append(
				"config key %q: !ref target %q does not exist",
				keyName,
				ref.TargetKey,
			)
		}
		return ResolvedValue{}, false
	}

	if _, isTargetRef := tVal.Value.(RefSpec); isTargetRef {
		var ok bool
		targetVal, ok = r.resolveKey(ref.TargetKey, nextPath)
		if !ok {
			return ResolvedValue{}, false
		}
	} else {
		targetVal = tVal
	}

	val = r.copyTargetValue(val, targetVal)
	r.cache[keyName] = val
	return val, true
}

// detectCycle checks if keyName exists in the current traversal path, recording a cycle error if found.
func (r *refResolver) detectCycle(keyName string, path []string) bool {
	for i, p := range path {
		if p == keyName {
			cyclePath := append([]string(nil), path[i:]...)
			cyclePath = append(cyclePath, keyName)
			r.errs.Append(
				"config key %q: !ref cycle detected (%s)",
				path[0],
				strings.Join(cyclePath, " -> "),
			)
			return true
		}
	}
	return false
}

// copyTargetValue copies Value, Type, Source, Secret, and Pending state from targetVal to val.
func (r *refResolver) copyTargetValue(val ResolvedValue, targetVal ResolvedValue) ResolvedValue {
	val.Value = targetVal.Value
	val.Type = targetVal.Type
	val.Source = targetVal.Source
	if targetVal.Secret {
		val.Secret = true
	}
	val.Pending = targetVal.Pending
	val.PendingReason = targetVal.PendingReason
	return val
}

// resolveSpecialForms resolves references (!ref) and marks
// generators (!generate) and secrets (!secret) as pending.
func resolveSpecialForms(
	schema map[string]Key,
	resolved ResolvedConfig,
	errs *ResolveError,
) (ResolvedConfig, *ResolveError) {
	r := &refResolver{
		resolved: resolved,
		schema:   schema,
		errs:     errs,
		cache:    make(map[string]ResolvedValue),
	}

	for keyName, val := range resolved.values {
		if _, isRef := val.Value.(RefSpec); isRef {
			r.resolveKey(keyName, nil)
		}
	}

	for keyName, val := range r.cache {
		resolved.values[keyName] = val
	}

	// this part of the code handles !generate and !secret special forms.
	for keyName, val := range resolved.values {
		switch val.Value.(type) {
		case GenerateSpec:
			val.Value = nil
			val.Pending = true
			val.PendingReason = "value is !generate; requires .kiln/state.json, which is not yet available"
			resolved.values[keyName] = val
		case SecretRef:
			val.Value = nil
			val.Pending = true
			val.PendingReason = "value is !secret; requires a SecretProvider, which is not yet configured"
			val.Secret = true
			resolved.values[keyName] = val
		}
	}

	return resolved, errs
}

// isSpecialForm returns true if the value matches one of Kiln's custom spec types.
func isSpecialForm(v any) bool {
	switch v.(type) {
	case GenerateSpec, SecretRef, RefSpec:
		return true
	}
	return false
}
