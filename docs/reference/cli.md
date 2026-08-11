# CLI Reference

This document serves as the command-line interface reference for Kiln. As I build out more features and commands, I will document them here.

## Global Flags

The following global flags apply to all or most commands within Kiln:

| Flag | Applies to | Description |
|---|---|---|
| `--env=<name>` | all | Select the environment layer. |
| `--format=<yaml\|json>` | all | Output format, for scripting/CI consumption. |
| `--no-color` | all | Disable ANSI color output. |
| `-v, --verbose` | all | Increase log verbosity; repeatable (`-vv`). |
| `--version` | kiln (root only) | Print the version of kiln and exit. |

## Commands

### `kiln init`

Scaffold a new project in the current directory. This creates a `kiln.yaml` manifest file, a local overrides `kiln.config.yaml` file, and an empty `plugins/` directory.

#### Flags

* `--template=<name>`: Starter template to use. Currently supports only the `blank` template (default).
* `--name=<name>`: Explicit name for the project. If not provided, it defaults to the target directory name (sanitized to meet identifier requirements).
* `--force`: Overwrite `kiln.yaml` if it already exists.
