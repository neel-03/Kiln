# Manifest Reference

This reference details the schema, structural constraints, and validation rules for the Kiln project manifest (`kiln.yaml`) and plugin manifest (`plugin.yaml`).

---

## `kiln.yaml` Structural Validation Rules

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

### 6. Cross-Reference Validation
To ensure logical consistency and prevent deployment failures due to misconfigured service or task dependencies, the manifest validates references between services, tasks, and plugin paths:

* **Service Dependencies (`depends_on`)**:
  - Every service listed under a service's `depends_on` array must exist within the `services` block.
  - A service cannot list itself as a dependency (self-dependency is prohibited).
* **Task Execution Target (`runs_on`)**:
  - The service name specified in a task's `runs_on` field must exist within the `services` block.
* **Task Dependencies (`depends_on`)**:
  - Every entry in a task's `depends_on` array must resolve to a valid target.
  - Suffixes of `.ready` are checked against the names of services in the `services` block (representing a dependency on that service being healthy).
  - All other dependency targets without a `.ready` suffix are checked against the names of other tasks in the `tasks` block.
* **Plugin Paths (`path`)**:
  - When referencing local plugins with a `path` property, the target directory must exist relative to the directory containing the project manifest file. (Pure in-memory manifest validations may skip this check).
