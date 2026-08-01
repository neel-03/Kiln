# Kiln

Kiln is a dynamic, language-agnostic project orchestration system designed to simplify building, deploying, and managing complex multi-service applications.

## What Kiln Does

Managing configuration, initialization tasks, and plugins for complex multi-service apps is usually a headache. Existing tools often lock you into a single platform (like Kubernetes) or require messy custom scripting to connect everything.

Kiln solves this by generalizing a highly extensible patch, hook, and job model to work with any application. Conceptually, it blends ideas from other familiar tools into a single orchestration workflow:

- **Helm:** Bringing the concept of a values-driven rendering step to generate clean deployment manifests.
- **Terraform:** Adopting a clear `plan` and `apply` lifecycle to map and execute initialization tasks safely.

You define your services and relationships once, and Kiln compiles and runs them identically across Docker Compose, Kubernetes, or systemd.

## Getting Started

### Development Commands

You can build and test Kiln using standard Go tools. Here are the core commands to get you started:

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
