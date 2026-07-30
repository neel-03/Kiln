---
name: kiln-conventions
description: >
  Target architecture and design rules for the Kiln orchestration tool.
  Load this whenever writing, reviewing, or reasoning about code in this
  repository — it encodes the design rules the codebase is being built
  toward, which are not discoverable from any single file in isolation.
  This repo is early-stage: most paths referenced below do not exist yet.
  Treat them as the intended structure to create, not as an existing
  layout to inspect.
---

# Kiln project conventions

Kiln is a Go-based, language-agnostic project-orchestration tool. Its value
depends on a small number of architectural rules holding across the entire
codebase. This file describes those rules ahead of the code that will
enforce them, so the repo is built in the right shape from the first
commit instead of needing a later restructure.

## Current state of this repo

As of now it likely contains few or none of the directories named below.
**Do not assume `core/`, `target/`, `pluginsdk/`, or `.kiln/` exist —
check before referencing them, and create them
following the layout below when the relevant work actually starts.** If
asked to scaffold the project, use this file as the target structure, not
as a description of what's already there.

Suggested build order (see the project's own phased roadmap if present):
config/manifest parsing first, with no plugins or targets yet; then a
single render pipeline and one target driver (Compose); then the task
DAG and first-party plugins; then hooks and remote plugin distribution;
then a second target driver and out-of-process plugins. Don't scaffold
every directory below in one pass just because this file lists them —
create each one when there's real code to put in it.

## Target directory layout

```
core/            # config, manifest, hooks, template/patch engine, graph,
                 # plugin loader, secrets, state, pipeline orchestration
target/          # deployment drivers: compose/, k8s/, systemd/
pluginsdk/       # the public Go SDK third-party plugins import
plugins/         # first-party plugins, or a
                 # separate kiln-plugins repo — either is fine, see that
                 # repo's own README if it exists
cmd/kiln/        # CLI entry point
.kiln/           # generated at runtime (env/, state.json) — never
                 # hand-authored, never committed
```

## Hard rules (treat violations as blocking, not stylistic)

These apply the moment the relevant directory/package is created — from its
first file, not retroactively once it's grown large.

1. **`core/`, once created, must have zero built-in application
   knowledge.** Nothing under it may reference a specific framework,
   language ecosystem, cloud provider, or SaaS by name. If a change needs
   that, it belongs in a plugin, not in core. This applies from the very
   first file added to `core/`, not just once the package is established.

2. **`target/` drivers only consume target-agnostic `Service`/`Task` types
   defined in `core/graph`.** Never add a field to those shared types that's
   shaped like one specific target's config format (e.g. don't add a
   Kubernetes-only field to `Service` just because one driver needs it —
   generalize the field or push the translation into that driver). If
   `core/graph` doesn't exist yet and a target driver is being built first
   for some reason, flag that ordering as a risk rather than proceeding
   silently — the shared type should come first.

3. **Every plugin-facing method takes `context.Context` as its first
   parameter and returns `error` as its last return value** — this applies
   uniformly across all three plugin tiers (manifest-only, scripted,
   compiled/RPC) from the first interface defined, so the pipeline code
   never needs a special case per tier.

4. **Nothing is read from `.kiln/state.json` during config resolution —
   only during planning.** This file does not exist until the project has
   a working apply stage; when it's introduced, keep it write-only from
   config resolution's point of view from day one.

5. **Any change to the `Plugin`, `Target`, or `SecretProvider` interfaces,
   once they exist, is a breaking change** and needs a major version bump
   plus a migration note, not a silent signature change. Before these
   interfaces are first defined, get the shape right deliberately — this
   is the most expensive thing to redesign once any plugin depends on it.

## Style conventions

- Table-driven tests preferred over repeated near-identical test functions.
- Integration tests (anything touching Docker or a network call) must be
  behind the `integration` build tag and named `Test...Integration`, so
  `go test -short ./...` never accidentally shells out. Set this
  convention up in the very first test file, not after integration tests
  already exist without it.
- Errors are wrapped with `fmt.Errorf("...: %w", err)`, never dropped or
  logged-and-swallowed in core packages.
- Exported types and functions in `core/` and `pluginsdk/` require a doc
  comment from the first commit that exports them — these are the packages
  third parties will import.

## When creating a plugin manifest (`plugin.yaml`)

This format applies once plugin support exists; skip this section entirely
until then.

- `metadata.name` must match the containing directory name.
- Config keys are namespaced automatically under the plugin name — don't
  prefix them again inside the key itself (`version`, not `postgres_version`,
  inside the `postgres` plugin's own manifest).
- Any `secret: true` config key must never appear in a `patches[].content`
  block in plaintext — reference it via `${plugin.key}` and let the
  resolver substitute it at apply time.
- Prefer `content_file:` over inline `content:` for any patch longer than a
  couple of lines.

## Anti-patterns to flag (documented failure modes, not hypotheticals)

These are worth watching for even in a young codebase — the earlier a
pattern like this is caught, the cheaper it is to redirect.

- **"Just add it to core."** Any PR that adds an `if` branch to `core/`
  keyed on a specific framework, language ecosystem, cloud provider, or SaaS
  name violates rule 1, no matter how small or reasonable it looks in
  isolation — including the very first such branch in a brand-new package.
  Redirect to a plugin instead, even when the plugin path is slower for
  the requester.
- **Tier creep.** If, once plugins exist, most of them end up Tier 3
  (compiled/RPC) instead of Tier 1 (manifest-only), something is wrong with
  either the config schema's expressiveness or the docs steering authors
  there too early. Tier 1 should cover the large majority of real plugins.
- **Parallel target-specific template trees.** A target driver
  (`target/compose`, `target/k8s`, `target/systemd`, ...) must never gain
  plugin-specific or application-specific knowledge. If a target needs a new
  capability, it needs a new generic field on `Service` or `Task`, reviewed
  as carefully as any other core type change — never a special case inside
  one driver. Watch for this from the second target driver onward, since a
  single driver alone can't yet show the divergence.
- **State file as a second source of truth.** Once `.kiln/state.json`
  exists, nothing should ever be read from it during config resolution —
  only during planning (rule 4). If it starts accumulating fields that are
  actually configuration, that's a regression toward a much heavier state
  model this project deliberately avoids.
- **Hidden global flags that change pipeline semantics.** A flag like
  `--legacy-mode` or `--skip-validation` that quietly changes what a stage
  computes (rather than how much runs or how results are displayed) breaks
  the promise that the CLI teaches the system. Anything that changes *what*
  the pipeline computes belongs in config (visible via `kiln config
  resolve`), not an invisible flag. Watch for this from the first CLI flag
  added, not just once the CLI has grown large.

## When reviewing a PR

- Check rules 1–5 above before anything else, for whichever of them are
  currently applicable given what exists in the repo today — a
  stylistically clean PR that violates an applicable one is still a
  request for changes.
- Check the anti-patterns above next — they're the concrete shapes rule
  violations tend to take under real deadline pressure, so they're worth
  checking explicitly rather than trusting the five rules alone to catch
  them.
- If a PR adds a new core-level patch point, hook name, or CLI command,
  confirm the same PR also updates the reference documentation in the same
  pull request — undocumented extension points are treated as a missing
  requirement, not a follow-up.
- Any change to the `Plugin`, `Target`, or `SecretProvider` interface
  signatures, once they're defined, needs a major version bump and a
  documented migration note — additive changes (new optional field, new
  method with a default) are fine at any time; anything else is breaking
  regardless of how small the diff looks. Before these interfaces are
  first defined, this rule doesn't apply yet — but flag if a PR is trying
  to define them hastily without considering what plugins will need.