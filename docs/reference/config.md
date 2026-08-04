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
