/*
Copyright © 2025 Katie Mulliken <katie@mulliken.net>
*/
package cmd

import (
	"bytes"
	"runtime"
	"testing"
	"time"
)

func TestArchiveCmd_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		defaultValue interface{}
		flagType     string
	}{
		{
			name:         "id flag has correct default",
			flagName:     "id",
			defaultValue: int64(0),
			flagType:     "int64",
		},
		{
			name:         "limit flag has correct default",
			flagName:     "limit",
			defaultValue: 0,
			flagType:     "int",
		},
		{
			name:         "timeout flag has correct default",
			flagName:     "timeout",
			defaultValue: 40 * time.Second,
			flagType:     "duration",
		},
		{
			name:         "wait-selector flag has correct default",
			flagName:     "wait-selector",
			defaultValue: "",
			flagType:     "string",
		},
		{
			name:         "chrome-path flag has correct default",
			flagName:     "chrome-path",
			defaultValue: "",
			flagType:     "string",
		},
		{
			name:         "headful flag has correct default",
			flagName:     "headful",
			defaultValue: false,
			flagType:     "bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var flag interface{}
			var err error

			switch tt.flagType {
			case "string":
				flag, err = archiveCmd.Flags().GetString(tt.flagName)
			case "int":
				flag, err = archiveCmd.Flags().GetInt(tt.flagName)
			case "int64":
				flag, err = archiveCmd.Flags().GetInt64(tt.flagName)
			case "bool":
				flag, err = archiveCmd.Flags().GetBool(tt.flagName)
			case "duration":
				flag, err = archiveCmd.Flags().GetDuration(tt.flagName)
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

func TestArchiveCmd_CommandMetadata(t *testing.T) {
	if archiveCmd.Use != "archive" {
		t.Errorf("Expected Use to be 'archive', got %s", archiveCmd.Use)
	}

	if archiveCmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestArchiveCmd_UsageOutput(t *testing.T) {
	var buf bytes.Buffer
	archiveCmd.SetOut(&buf)
	archiveCmd.SetErr(&buf)

	err := archiveCmd.Usage()
	if err != nil {
		t.Errorf("Usage() returned error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("Expected usage output, got empty string")
	}

	// Check that key flags are mentioned in usage
	expectedFlags := []string{"--id", "--limit", "--timeout", "--chrome-path", "--headful"}
	for _, flag := range expectedFlags {
		if !bytes.Contains([]byte(output), []byte(flag)) {
			t.Errorf("Expected usage to mention %s", flag)
		}
	}
}

func TestArchiveCmd_InheritsDBFlag(t *testing.T) {
	// The archive command should have access to the persistent --db flag from root
	flag := archiveCmd.InheritedFlags().Lookup("db")
	if flag == nil {
		t.Error("Expected archive command to inherit --db flag from root")
	}
}

func TestArchiveCmd_ChromePathDefault_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	// On macOS, when chrome-path is empty, it should default to the standard Chrome location
	// This is tested indirectly through the runArchive logic
	chromePath, _ := archiveCmd.Flags().GetString("chrome-path")
	if chromePath != "" {
		t.Errorf("Expected default chrome-path to be empty (runtime detection), got %s", chromePath)
	}
}

func TestArchiveCmd_HeadlessDefault(t *testing.T) {
	// By default, headful should be false (meaning headless mode is enabled)
	headful, err := archiveCmd.Flags().GetBool("headful")
	if err != nil {
		t.Fatalf("Failed to get headful flag: %v", err)
	}

	if headful {
		t.Error("Expected headful to default to false (headless mode)")
	}
}

func TestArchiveCmd_FlagShortcuts(t *testing.T) {
	// Verify that expected flag shortcuts exist
	// Note: archive command doesn't define shortcuts, but we verify the flags exist
	flags := archiveCmd.Flags()

	requiredFlags := []string{"id", "limit", "timeout", "wait-selector", "chrome-path", "headful"}
	for _, name := range requiredFlags {
		if flags.Lookup(name) == nil {
			t.Errorf("Expected flag %s to be defined", name)
		}
	}
}
