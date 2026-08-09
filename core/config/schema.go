package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Type represents the supported configuration value types.
//
// Every configuration key declares exactly one Type.
// Secret values are represented by Key.Secret rather than as a
// distinct type because secrecy is an attribute of a value, not a value
// type itself.
type Type string

const (
	// TypeString represents a UTF-8 string.
	TypeString Type = "string"

	// TypeInt represents an integer.
	TypeInt Type = "int"

	// TypeFloat represents a floating-point value.
	TypeFloat Type = "float"

	// TypeBool represents a boolean value.
	TypeBool Type = "bool"

	// TypeDuration represents a Go time.Duration.
	TypeDuration Type = "duration"

	// TypeListString represents a list of strings.
	TypeListString Type = "list[string]"

	// TypeMapString represents a string-to-string map.
	TypeMapString Type = "map[string]string"
)

// Key describes one configuration value exposed by Kiln or a plugin.
type Key struct {
	// Namespace is the prefix/grouping for the configuration key (e.g., "postgres").
	Namespace string

	// Key is the name of the configuration option (e.g., "storage_gb").
	Key string

	// Type is the expected Type for the configuration option.
	Type Type

	// Default stores the default value for this configuration key.
	Default any

	// Description provides a human-readable explanation of the key's purpose.
	Description string

	// Secret indicates that the value should be redacted when displayed.
	Secret bool

	// Enum restricts string values to one of the listed strings.
	Enum []string

	// Pattern restricts string values using a regular expression.
	Pattern string

	// Min limits numeric values (int, float, duration) from below.
	Min *float64

	// Max limits numeric values (int, float, duration) from above.
	Max *float64

	// MinItems limits collection values (list[string]) length from below.
	MinItems *int

	// MaxItems limits collection values (list[string]) length from above.
	MaxItems *int
}

// FullName returns the fully-qualified configuration key.
func (k Key) FullName() string {
	return k.Namespace + "." + k.Key
}

// Validate validates a configuration value against this schema.
func (k Key) Validate(v any) error {
	switch k.Type {

	case TypeString:
		return k.validateString(v)

	case TypeInt:
		return k.validateInt(v)

	case TypeFloat:
		return k.validateFloat(v)

	case TypeBool:
		return k.validateBool(v)

	case TypeDuration:
		return k.validateDuration(v)

	case TypeListString:
		return k.validateStringList(v)

	case TypeMapString:
		return k.validateStringMap(v)

	default:
		return fmt.Errorf("%s: unsupported config type %q", k.FullName(), k.Type)
	}
}

// validateString validates that v is a string and satisfies any Enum/Pattern.
func (k Key) validateString(v any) error {
	value, ok := v.(string)
	if !ok {
		return k.typeError("string", v)
	}

	if len(k.Enum) > 0 {
		if !slices.Contains(k.Enum, value) {
			return fmt.Errorf(
				"%s: value %q must be one of [%s]",
				k.FullName(),
				k.formatErrorValue(value),
				strings.Join(k.Enum, ", "),
			)
		}
	}

	if k.Pattern != "" {
		re, err := regexp.Compile(k.Pattern)
		if err != nil {
			return fmt.Errorf(
				"%s: invalid schema pattern %q: %w",
				k.FullName(),
				k.Pattern,
				err,
			)
		}

		if !re.MatchString(value) {
			return fmt.Errorf(
				"%s: value %q does not match pattern %q",
				k.FullName(),
				k.formatErrorValue(value),
				k.Pattern,
			)
		}
	}

	return nil
}

// validateInt validates that v is an int and satisfies any min/max.
func (k Key) validateInt(v any) error {
	value, ok := v.(int)
	if !ok {
		return k.typeError("int", v)
	}

	return k.validateNumericLimits(float64(value), value)
}

// validateFloat validates that v is a float64 and satisfies any min/max.
func (k Key) validateFloat(v any) error {
	value, ok := v.(float64)
	if !ok {
		return k.typeError("float", v)
	}

	return k.validateNumericLimits(value, value)
}

// validateNumericLimits validates that v is within the given min/max bounds.
func (k Key) validateNumericLimits(val float64, original any) error {
	if k.Min != nil && val < *k.Min {
		return fmt.Errorf(
			"%s: value %s is below minimum %g",
			k.FullName(),
			k.formatErrorValue(original),
			*k.Min,
		)
	}

	if k.Max != nil && val > *k.Max {
		return fmt.Errorf(
			"%s: value %s exceeds maximum %g",
			k.FullName(),
			k.formatErrorValue(original),
			*k.Max,
		)
	}

	return nil
}

// validateBool validates that v is a boolean.
func (k Key) validateBool(v any) error {
	if _, ok := v.(bool); !ok {
		return k.typeError("bool", v)
	}

	return nil
}

// validateDuration validates that v is a time.Duration or a string that can be parsed as such, and satisfies any min/max limits.
func (k Key) validateDuration(v any) error {
	var dur time.Duration
	switch value := v.(type) {
	case time.Duration:
		dur = value
	case string:
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf(
				"%s: invalid duration %q: %v",
				k.FullName(),
				k.formatErrorValue(value),
				err,
			)
		}
		dur = d
	default:
		return k.typeError("duration", v)
	}

	return k.validateNumericLimits(dur.Seconds(), v)
}

// validateStringList validates that v is a slice of strings and satisfies any min/max items.
func (k Key) validateStringList(v any) error {
	value, ok := v.([]string)
	if !ok {
		return k.typeError("list[string]", v)
	}

	if k.MinItems != nil && len(value) < *k.MinItems {
		return fmt.Errorf(
			"%s: expected at least %d items, got %d",
			k.FullName(),
			*k.MinItems,
			len(value),
		)
	}

	if k.MaxItems != nil && len(value) > *k.MaxItems {
		return fmt.Errorf(
			"%s: expected at most %d items, got %d",
			k.FullName(),
			*k.MaxItems,
			len(value),
		)
	}

	return nil
}

// validateStringMap validates that v is a map of strings to strings.
func (k Key) validateStringMap(v any) error {
	if _, ok := v.(map[string]string); !ok {
		return k.typeError("map[string]string", v)
	}

	return nil
}

// typeError returns a formatted error message for a type mismatch.
func (k Key) typeError(expected string, value any) error {
	actual := typeName(value)

	return fmt.Errorf(
		`%s: expected %s, got %s %q`,
		k.FullName(),
		expected,
		actual,
		k.formatErrorValue(value),
	)
}

// formatErrorValue formats a value for display in error messages, redacting it if k.Secret is true.
func (k Key) formatErrorValue(v any) string {
	if k.Secret {
		return "<redacted>"
	}
	return formatValue(v)
}

// typeName returns the type name of a value.
func typeName(v any) string {
	if v == nil {
		return "nil"
	}

	return fmt.Sprintf("%T", v)
}

// formatValue formats a value for display in error messages.
func formatValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprint(v)
	}
}
