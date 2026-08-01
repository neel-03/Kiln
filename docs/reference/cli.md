# CLI Reference

This document serves as the command-line interface reference for Kiln. As I build out more features and commands, I will document them here.

## Global Flags

The following global flags apply to all or most commands within Kiln:

| Flag | Applies to | Description |
|---|---|---|
| `--env=<name>` | all | Select the environment layer. |
| `--format=<yaml\|json>` | most read commands | Output format, for scripting/CI consumption. |
| `--no-color` | all | Disable ANSI color output. |
| `-v, --verbose` | all | Increase log verbosity; repeatable (`-vv`). |
| `--version` | all | Print the version of kiln and exit. |
