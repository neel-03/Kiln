# Kiln

**Kiln** is a **language-agnostic project orchestration system** designed to simplify building, deploying, and managing multi-service applications.

## What Kiln Does

Managing configuration, initialization tasks, and plugins for complex multi-service apps can be difficult. Existing tools often lock you into a single platform (like Kubernetes) or require fragile custom scripting to connect everything.

Kiln solves this by generalizing a highly extensible patch, hook, and job model to work with any application. Conceptually, it blends ideas from familiar tools into a single orchestration workflow:

- **Helm-style rendering:** Generates clean, environment-specific deployment manifests from templates.
- **Terraform-style planning:** Employs a clear `plan` and `apply` lifecycle to map and execute initialization tasks safely in dependency order.

You define your services once, and Kiln runs them identically across Docker Compose, Kubernetes, or systemd.

## Getting Started

### Development Commands

You can build and test Kiln using standard Go tools:

- **Build the project:**
  ```bash
  go build ./...
  ```

- **Run all unit tests:**
  ```bash
  go test -short -race ./...
  ```

## Documentation

Kiln's architecture, design decisions, and reference specifications will be documented shortly.
