# Manifest Reference

This reference details the schema, structural constraints, and validation rules for the Kiln project manifest (`kiln.yaml`) and plugin manifest (`plugin.yaml`).

---

## `kiln.yml` Structural Validation Rules

When you load a project manifest, the compiler enforces the following structural rules:

### 1. Document Identity
* **`apiVersion`**: Must be set exactly to `kiln/v1`. This ensures compatibility with the parser version.
* **`kind`**: Must be set exactly to `Project`. This distinguishes the project manifest from other resource kinds (like plugin manifests).

### 2. Naming Conventions
* **`metadata.name`**: Must be a non-empty string and match the pattern `^[a-z0-9][a-z0-9-]*$`.

  > **Why this format?** This restrictive naming pattern ensures that the project name is fully compatible with Kubernetes resource labels, Docker Compose project namespace requirements, and general DNS/URL naming limits.

### 3. Plugin Setup
* **`plugins`**: Each plugin entry in the array must specify **either** a local folder `path` **or** a remote `module` source—but never both.
* **`plugins.module`**: If the module references a remote package (e.g., `github.com/example/plugin@v1.2.3`), and you include an `@` character to specify a version, the version suffix following it cannot be empty.

### 4. Service Definitions
* **`services`**: Each service must declare exactly **one** primary container source:
  * `from`: Reference to a defined plugin.
  * `image`: A pre-built container image (e.g., `nginx:latest`).
  * `build`: A local directory context containing a Dockerfile definition.

  Declaring more than one (or none at all) is ambiguous and will fail validation.

* **`services.healthcheck`**: If a healthcheck block is provided, you must declare exactly **one** check mechanism: either `http` (for an HTTP GET path check) or `command` (for a custom shell command execution inside the container).

### 5. Task Execution Phases
* **`tasks.phase`**: Every task must be assigned to exactly one of the supported execution phases. These control *when* the task is scheduled to run during the deployment lifecycle:
  * `build`: Build-time orchestration.
  * `pre-init`: Bootstrapping infrastructure (e.g., creating storage buckets or DBs).
  * `init`: Database migrations, seeding, or basic app setup.
  * `post-init`: Final checks and post-start configuration.
  * `runtime`: Long-lived operations or runtime verification.
