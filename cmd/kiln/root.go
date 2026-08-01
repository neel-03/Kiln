package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	formatYaml    string = "yaml"
	formatJson    string = "json"
	formatDefault string = formatYaml
)

var supportedFormats = map[string]struct{}{
	formatYaml: {},
	formatJson: {},
}

// globalOpts holds the global configuration options for the kiln CLI.
type globalOpts struct {
	Environment string
	Format      string
	NoColor     bool
	Verbosity   int
}

// newRootCmd constructs the root cobra.Command for the kiln CLI.
func newRootCmd() *cobra.Command {
	opts := &globalOpts{}

	rootCmd := &cobra.Command{
		Use:     "kiln",
		Short:   "A dynamic, language-agnostic project orchestration system.",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if _, ok := supportedFormats[opts.Format]; !ok {
				return fmt.Errorf(
					"unsupported --format %q, expected \"yaml\" or \"json\"",
					opts.Format,
				)
			}
			return nil
		},

		// silencing errors and usage messages as of now
		// since individual subcommands will control them on their own
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	flags := rootCmd.PersistentFlags()

	flags.StringVar(&opts.Environment, "env", "", "Environment to run in.")
	flags.StringVar(&opts.Format, "format", formatDefault, "Output format (Yaml/Json).")
	flags.BoolVar(&opts.NoColor, "no-color", false, "Disable ANSI color output.")
	flags.CountVarP(&opts.Verbosity, "verbose", "v", "Increase log verbosity (repeatable).")

	return rootCmd
}
