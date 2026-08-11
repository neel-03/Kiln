package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neel-03/Kiln/core"
	"github.com/neel-03/Kiln/core/manifest"
	"github.com/spf13/cobra"
)

//go:embed templates/blank/kiln.yaml templates/blank/kiln.config.yaml
var initTemplates embed.FS

const (
	defaultInitTemplate = "blank"
	kilnManifestFile    = "kiln.yaml"
	kilnConfigFile      = "kiln.config.yaml"

	manifestTemplatePath = "templates/" + defaultInitTemplate + "/" + kilnManifestFile
	configTemplatePath   = "templates/" + defaultInitTemplate + "/" + kilnConfigFile

	projectNamePlaceholder = "__KILN_PROJECT_NAME__"

	pluginsDir      = "plugins"
	pluginsKeepFile = ".gitkeep"

	gitignoreFile = ".gitignore"
	kilnStateDir  = ".kiln/"
)

// initOptions holds the configuration for the "kiln init" command.
type initOptions struct {
	// The name of the template to use for initialization.
	templateName string

	// The name of the project. Defaults to the current directory name.
	name string

	// Whether to overwrite an existing kiln.yaml.
	force bool
}

// newInitCmd constructs the "kiln init" command.
func newInitCmd() *cobra.Command {
	opts := &initOptions{
		templateName: defaultInitTemplate,
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new Kiln project.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVar(
		&opts.templateName,
		"template",
		defaultInitTemplate,
		"Starter template to use.",
	)

	flags.StringVar(
		&opts.name,
		"name",
		"",
		"Project name. Defaults to the current directory name.",
	)

	flags.BoolVar(
		&opts.force,
		"force",
		false,
		"Overwrite an existing kiln.yaml.",
	)

	return cmd
}

// runInit implements the "kiln init" command.
func runInit(cmd *cobra.Command, opts *initOptions) error {
	// validate the template name
	if err := validateInitTemplate(opts.templateName); err != nil {
		return err
	}

	// get the current working directory
	targetDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determining current directory: %w", err)
	}

	// resolve the project name from the options or the current directory
	// if --name is not provided, then the project name is the current directory name (after sanitization)
	projectName, err := resolveProjectName(targetDir, opts.name)
	if err != nil {
		return err
	}

	// path to the kiln.yaml file
	manifestPath := filepath.Join(targetDir, kilnManifestFile)

	// if kiln.yaml already exists, return error unless --force is specified
	if _, err := os.Stat(manifestPath); err == nil {
		if !opts.force {
			return fmt.Errorf(
				"kiln.yaml already exists in this directory (use --force to overwrite)",
			)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking for existing kiln.yaml: %w", err)
	}

	// now we create the kiln.yaml file by rendering the template
	// this replaces the __KILN_PROJECT_NAME__ placeholder with the actual project name
	manifestData, err := renderInitManifest(opts.templateName, projectName)
	if err != nil {
		return err
	}

	// read the kiln.config.yaml template
	configData, err := readInitTemplate(opts.templateName, configTemplatePath)
	if err != nil {
		return err
	}

	// write the kiln.yaml file with the given data
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", kilnManifestFile, err)
	}

	// always validate the actual file that was written. This ensures the
	// embedded template, placeholder substitution, YAML parser, and manifest
	// validator all agree before init reports success.
	if err := validateWrittenManifest(targetDir, manifestPath); err != nil {
		return fmt.Errorf("validating generated %s: %w", kilnManifestFile, err)
	}

	configPath := filepath.Join(targetDir, kilnConfigFile)

	// kiln.config.yaml holds local overrides and is gitignored, so never
	// replace it silently.
	configExists := false
	if _, err := os.Stat(configPath); err == nil {
		configExists = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking for existing %s: %w", kilnConfigFile, err)
	}

	if !configExists || opts.force {
		if err := os.WriteFile(configPath, configData, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", kilnConfigFile, err)
		}
	}

	if err := ensurePluginsDirectory(targetDir); err != nil {
		return err
	}

	if err := ensureGitignore(targetDir); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	noColor := getNoColor(cmd)

	if noColor {
		_, _ = fmt.Fprintln(out, "Created kiln.yaml")
		_, _ = fmt.Fprintln(out, "Created kiln.config.yaml")
		_, _ = fmt.Fprintln(out, "Created plugins/")
		_, _ = fmt.Fprintln(out, "Next: edit kiln.config.yaml, then run `kiln doctor`")
	} else {
		_, _ = fmt.Fprintln(out, "\033[32m✔\033[0m Created \033[1;36mkiln.yaml\033[0m")
		_, _ = fmt.Fprintln(out, "\033[32m✔\033[0m Created \033[1;36mkiln.config.yaml\033[0m")
		_, _ = fmt.Fprintln(out, "\033[32m✔\033[0m Created \033[1;36mplugins/\033[0m")
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, "\033[1;33mNext Steps:\033[0m")
		_, _ = fmt.Fprintln(out, "  1. Edit \033[1;36mkiln.config.yaml\033[0m to configure your environment.")
		_, _ = fmt.Fprintln(out, "  2. Run \033[1;32mkiln doctor\033[0m to verify your setup.")
	}

	return nil
}

// validateInitTemplate validates that the template name is supported.
func validateInitTemplate(templateName string) error {
	if templateName == defaultInitTemplate {
		return nil
	}

	return fmt.Errorf(
		"unsupported template %q; available templates: %s",
		templateName,
		defaultInitTemplate,
	)
}

// readInitTemplate reads an embedded template file for the given template.
func readInitTemplate(templateName, path string) ([]byte, error) {
	if err := validateInitTemplate(templateName); err != nil {
		return nil, err
	}

	data, err := initTemplates.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"reading embedded template %s: %w",
			path,
			err,
		)
	}

	return data, nil
}

// renderInitManifest reads the template manifest file, replaces the project name placeholder,
// and returns the raw template manifest content.
func renderInitManifest(templateName, projectName string) ([]byte, error) {
	data, err := readInitTemplate(templateName, manifestTemplatePath)
	if err != nil {
		return nil, err
	}

	content := string(data)

	if strings.Count(content, projectNamePlaceholder) != 1 {
		return nil, fmt.Errorf(
			"embedded manifest template must contain exactly one %q placeholder",
			projectNamePlaceholder,
		)
	}

	content = strings.Replace(
		content,
		projectNamePlaceholder,
		projectName,
		1,
	)

	return []byte(content), nil
}

// resolveProjectName determines the project name. If explicitName is provided, it validates it.
// If not, it derives and sanitizes the name from the target directory basename.
func resolveProjectName(targetDir, explicitName string) (string, error) {
	if explicitName != "" {
		if !core.ProjectNamePattern.MatchString(explicitName) {
			return "", fmt.Errorf(
				"invalid --name %q: must match pattern %s",
				explicitName,
				core.ProjectNameRegex,
			)
		}

		return explicitName, nil
	}

	base := filepath.Base(filepath.Clean(targetDir))
	name := sanitizeProjectName(base)

	if !core.ProjectNamePattern.MatchString(name) {
		return "", fmt.Errorf(
			"could not derive a valid project name from directory %q",
			base,
		)
	}

	return name, nil
}

// sanitizeProjectName sanitizes a string to be used as a project name.
// It converts the string to lowercase, removes any leading or trailing whitespace,
// replaces any sequence of non-alphanumeric characters with a single hyphen,
// and removes any leading or trailing hyphens.
//
// Example:
//
//	"My Project" -> "my-project"
//	"  My Project  " -> "my-project"
//	"_my_project!" -> "my-project"
func sanitizeProjectName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))

	var b strings.Builder
	b.Grow(len(name))

	previousHyphen := false

	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			previousHyphen = false

		case r >= '0' && r <= '9':
			b.WriteRune(r)
			previousHyphen = false

		case r == '-':
			if b.Len() > 0 && !previousHyphen {
				b.WriteByte('-')
				previousHyphen = true
			}

		default:
			if b.Len() > 0 && !previousHyphen {
				b.WriteByte('-')
				previousHyphen = true
			}
		}
	}

	result := strings.Trim(b.String(), "-")

	if result == "" {
		return "kiln"
	}

	return result
}

// validateWrittenManifest loads and validates the written manifest to ensure it is structurally valid.
func validateWrittenManifest(targetDir, manifestPath string) error {
	loaded, err := manifest.LoadProjectManifest(manifestPath)
	if err != nil {
		return err
	}

	if err := manifest.Validate(loaded, targetDir); err != nil {
		return err
	}

	return nil
}

// ensurePluginsDirectory creates the plugins directory and a .gitkeep file if they do not exist.
func ensurePluginsDirectory(targetDir string) error {
	dir := filepath.Join(targetDir, pluginsDir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s directory: %w", pluginsDir, err)
	}

	keepPath := filepath.Join(dir, pluginsKeepFile)

	file, err := os.OpenFile(
		keepPath,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0o644,
	)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}

		return fmt.Errorf(
			"creating %s/%s: %w",
			pluginsDir,
			pluginsKeepFile,
			err,
		)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf(
			"closing %s/%s: %w",
			pluginsDir,
			pluginsKeepFile,
			err,
		)
	}

	return nil
}

// ensureGitignore makes sure the .gitignore file exists and ignores the .kiln/ folder and kiln.config.yaml.
func ensureGitignore(targetDir string) error {
	path := filepath.Join(targetDir, gitignoreFile)

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading .gitignore: %w", err)
	}

	content := string(data)

	entries := []string{
		kilnStateDir,
		kilnConfigFile,
	}

	lines := strings.Split(content, "\n")

	present := make(map[string]struct{}, len(lines))

	for _, line := range lines {
		present[strings.TrimSpace(line)] = struct{}{}
	}

	var missing []string

	for _, entry := range entries {
		if _, ok := present[entry]; !ok {
			missing = append(missing, entry)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	content += strings.Join(missing, "\n")
	content += "\n"

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing .gitignore: %w", err)
	}

	return nil
}
