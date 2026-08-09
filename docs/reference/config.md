# Configuration Reference

Kiln's configuration layer supports a structured type and validation system to ensure environment and plugin configurations are statically verified before execution.

## Configuration Types & Constraints

Every configuration key defines a namespace, key, and type, along with optional constraints. When Kiln resolves your configuration, it validates the inputs against these rules.

---

### `string`
Represents a UTF-8 string value. This is the default type for most configuration inputs.

* **Constraints:**
  * `enum`: Restricts the value to a predefined list of allowed strings.
  * `pattern`: A regular expression (regex) that the string must match.
* **Example Schema Definition:**
  ```yaml
  - key: environment
    type: string
    default: "development"
    enum: ["development", "staging", "production"]

  - key: database_url
    type: string
    pattern: "^postgres://.*$"
  ```

---

### `int`
Represents whole numbers.

* **Constraints:**
  * `min`: The minimum allowed value (inclusive).
  * `max`: The maximum allowed value (inclusive).
* **Example Schema Definition:**
  ```yaml
  - key: storage_gb
    type: int
    default: 10
    min: 1
    max: 1000
  ```

---

### `float`
Represents floating-point/decimal numbers. Useful for ratios, CPU limits, or thresholds.

* **Constraints:**
  * `min`: The minimum allowed value (inclusive).
  * `max`: The maximum allowed value (inclusive).
* **Example Schema Definition:**
  ```yaml
  - key: cpu_limit
    type: float
    default: 0.5
    min: 0.1
    max: 8.0
  ```

---

### `bool`
Represents a boolean value (`true` or `false`).

* **Example Schema Definition:**
  ```yaml
  - key: enable_cache
    type: bool
    default: false
  ```

---

### `duration`
Represents a time duration. String values are parsed using Go's standard `time.ParseDuration` format (e.g., `"30s"`, `"5m"`, `"2h"`).

* **Constraints:**
  * `min`: The minimum duration allowed, specified in seconds (e.g., `10` for 10 seconds).
  * `max`: The maximum duration allowed, specified in seconds (e.g., `300` for 5 minutes).
* **Example Schema Definition:**
  ```yaml
  - key: request_timeout
    type: duration
    default: "30s"
    min: 5
    max: 60
  ```

---

### `list<string>`
Represents a list/array of string values.

* **Constraints:**
  * `min_items`: The minimum number of items required in the list.
  * `max_items`: The maximum number of items allowed in the list.
* **Example Schema Definition:**
  ```yaml
  - key: allowed_origins
    type: list[string]
    default: ["http://localhost:3000"]
    min_items: 1
  ```

---

### `map<string,string>`
Represents an associative map of key-value string pairs. This is typically used to configure environment variables.

* **Example Schema Definition:**
  ```yaml
  - key: custom_env
    type: map[string]string
    default:
      DEBUG: "true"
      LOG_LEVEL: "info"
  ```

---

### `secret`
A configuration key can mark its values as sensitive by setting `secret: true`. This is a modifier on top of any of the above types rather than a distinct type itself.

* When a key is marked as a secret, its value is automatically redacted in logs, console output, and printed configuration resolutions (showing `<redacted>`).
* **Example Schema Definition:**
  ```yaml
  - key: api_key
    type: string
    secret: true
  ```

---

## Validation Timing

Every type and constraint is checked during `Resolve` time—before any template renders. If a configured value does not match the expected type or violates a constraint, Kiln will immediately abort and report a clear validation error.

---

## Configuration Layers & Precedence

Kiln resolves configuration dynamically by merging multiple layers in order of precedence. If the same configuration key is specified in multiple layers, the layer with the higher precedence wins.

Here are the five configuration layers, ordered by precedence from highest (Layer 1) to lowest (Layer 5):

* **Layer 1: CLI Overrides** (`LayerCLIOverride`)
  - The highest precedence values. These are supplied at runtime via CLI flags or matching `KILN_*` environment variables.
* **Layer 2: User Configuration** (`LayerUserConfig`)
  - Values defined in the user's primary project configuration file, `kiln.config.yaml`.
* **Layer 3: Environment Defaults** (`LayerEnvironmentDefaults`)
  - Values loaded from files specific to the active environment under `environments/<env>.yaml` (e.g., `environments/development.yaml`).
* **Layer 4: Plugin-declared Defaults** (`LayerPluginDefaults`)
  - Default configuration values declared by plugins.
  - *Note:* Since plugin loading is not yet implemented, this layer is currently empty. It will be active once plugin loading is introduced.
* **Layer 5: Core Defaults** (`LayerCoreDefaults`)
  - Built-in defaults defined by Kiln itself (e.g., `kiln.target = "compose"`).
  - This layer is always present to provide base fallback values.

---

## The SchemaProvider Mechanism

To ensure Kiln knows how to validate configuration keys before merging layers, it compiles a schema registry. This is managed by the `SchemaProvider` interface:

```go
type SchemaProvider interface {
	ConfigSchema() []Key
}
```

- Any component (such as core Kiln or a plugin) can implement `SchemaProvider` to expose the configuration keys it expects.
- During resolution, Kiln gathers schemas from all registered providers, merges them, and validates incoming layer values against these schemas.
- If two different sources try to declare the same configuration key, Kiln will immediately report a duplicate key declaration error to prevent conflicting definitions.

---

## Special Value Forms

Kiln supports three special value forms when decoding values from configuration layers: `!generate`, `!secret`, and `!ref`. These tags allow for dynamic, secret, and referenced configuration patterns.

### `!ref <target.key>`

References another configuration key.
- **Resolution**: Fully resolved during configuration resolution. The target key's resolved value, type, and source are copied to the referencing key.
- **Errors**:
  - Resolving a reference to a nonexistent key is a hard error.
  - Cyclic references (e.g., key `A` referencing key `B`, which references key `A`) are automatically detected and result in a resolution failure, reporting the full cycle path.

### `!generate {length: N}`

Instructs Kiln to generate a random string of length `N` (where `N` is a positive integer).
- **Resolution**: Deliberately deferred. The generated value is not computed or persisted during this resolution phase. Stable generation requires state persistence, which will be introduced later.
- **State**: The key is marked as `Pending: true` with a reason indicating that state persistence is required, and its value remains empty.

### `!secret <secret_name>`

References a secret retrieved from a secret manager or provider.
- **Resolution**: Deliberately deferred. Resolving secret values requires a configured secret provider, which will be introduced later.
- **State**: The key is marked as `Pending: true` with a reason indicating that a secret provider is required, its value remains empty, and its metadata is marked as `Secret: true` so that it is redacted in displays.
