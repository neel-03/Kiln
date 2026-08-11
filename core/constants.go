// Package core defines project-wide constants, shared types, and package-level configurations
// for Kiln.
package core

import "regexp"

const (
	// ExpectedAPIVersion is the exact apiVersion value required by Kiln.
	ExpectedAPIVersion string = "kiln/v1"
	// ExpectedKind is the exact kind value required by Kiln.
	ExpectedKind string = "Project"

	// BuildPhase represents the build stage of a project lifecycle.
	BuildPhase string = "build"
	// PreInitPhase represents tasks that run before initialization.
	PreInitPhase string = "pre-init"
	// InitPhase represents the main initialization tasks.
	InitPhase string = "init"
	// PostInitPhase represents tasks that run after initialization.
	PostInitPhase string = "post-init"
	// RuntimePhase represents long-running or runtime execution tasks.
	RuntimePhase string = "runtime"

	// ProjectNameRegex is the regex pattern for the project name.
	ProjectNameRegex string = "^[a-z0-9][a-z0-9-]*$"
)

// ValidPhases is the ordered list of all supported project phases.
var ValidPhases = []string{BuildPhase, PreInitPhase, InitPhase, PostInitPhase, RuntimePhase}

// ProjectNamePattern is the compiled regex for the project name.
var ProjectNamePattern = regexp.MustCompile(ProjectNameRegex)
