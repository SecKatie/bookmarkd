/*
Copyright © 2025 Katie Mulliken <katie@mulliken.net>
*/
package cmd

import (
	"bytes"
	"testing"
)

func TestRootCmd_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		defaultValue interface{}
		flagType     string
	}{
		{
			name:         "db flag has correct default",
			flagName:     "db",
			defaultValue: "bookmarkd.db",
			flagType:     "string",
		},
		{
			name:         "port flag has correct default",
			flagName:     "port",
			defaultValue: 8080,
			flagType:     "int",
		},
		{
			name:         "host flag has correct default",
			flagName:     "host",
			defaultValue: "localhost",
			flagType:     "string",
		},
		{
			name:         "archive-workers flag has correct default",
			flagName:     "archive-workers",
			defaultValue: 1,
			flagType:     "int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var flag interface{}
			var err error

			switch tt.flagType {
			case "string":
				if tt.flagName == "db" {
					flag, err = rootCmd.PersistentFlags().GetString(tt.flagName)
				} else {
					flag, err = rootCmd.Flags().GetString(tt.flagName)
				}
			case "int":
				flag, err = rootCmd.Flags().GetInt(tt.flagName)
			}

			if err != nil {
				t.Fatalf("Failed to get flag %s: %v", tt.flagName, err)
			}

			if flag != tt.defaultValue {
				t.Errorf("Flag %s: got %v, want %v", tt.flagName, flag, tt.defaultValue)
			}
		})
	}
}

func TestRootCmd_HasArchiveSubcommand(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "archive" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected archive subcommand to be registered")
	}
}

func TestRootCmd_UsageOutput(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	// Test that usage doesn't error
	err := rootCmd.Usage()
	if err != nil {
		t.Errorf("Usage() returned error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("Expected usage output, got empty string")
	}
}

func TestRootCmd_CommandMetadata(t *testing.T) {
	if rootCmd.Use != "bookmarkd" {
		t.Errorf("Expected Use to be 'bookmarkd', got %s", rootCmd.Use)
	}

	if rootCmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	if rootCmd.Long == "" {
		t.Error("Expected Long description to be set")
	}
}
