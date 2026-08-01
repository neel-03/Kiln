package main

import "testing"

func TestRootCommandMetadata(t *testing.T) {
	cmd := newRootCmd()

	if cmd.Use != "kiln" {
		t.Fatalf("expected Use=kiln, got %q", cmd.Use)
	}

	if !cmd.SilenceUsage {
		t.Fatal("SilenceUsage should be true")
	}

	if !cmd.SilenceErrors {
		t.Fatal("SilenceErrors should be true")
	}

	if cmd.Version != "dev" {
		t.Fatalf("expected version dev, got %q", cmd.Version)
	}
}

func TestPersistentFlagsExist(t *testing.T) {
	cmd := newRootCmd()

	flags := cmd.PersistentFlags()

	tests := []struct {
		name     string
		defValue string
	}{
		{"env", ""},
		{"format", "yaml"},
		{"no-color", "false"},
		{"verbose", "0"},
	}

	for _, tc := range tests {
		flag := flags.Lookup(tc.name)
		if flag == nil {
			t.Fatalf("flag %q not registered", tc.name)
		}
		if flag.DefValue != tc.defValue {
			t.Fatalf("expected flag %q default value to be %q, got %q", tc.name, tc.defValue, flag.DefValue)
		}
	}

	if flags.Lookup("target") != nil {
		t.Fatal("--target must not be registered globally")
	}
}

func TestFormatValidation(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{
			name:   "yaml",
			format: "yaml",
		},
		{
			name:   "json",
			format: "json",
		},
		{
			name:    "toml",
			format:  "toml",
			wantErr: true,
		},
		{
			name:    "xml",
			format:  "xml",
			wantErr: true,
		},
		{
			name:    "empty",
			format:  "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newRootCmd()

			// Parse flags to set the options struct values
			err := cmd.ParseFlags([]string{"--format", tc.format})
			if err != nil {
				t.Fatalf("unexpected error parsing flags: %v", err)
			}

			if cmd.PersistentPreRunE != nil {
				err = cmd.PersistentPreRunE(cmd, []string{})
			}

			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}

			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
