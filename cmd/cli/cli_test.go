package main

import (
	"testing"
)

// resetFlags restores all package-level flag variables to their defaults before each test.
func resetFlags() {
	severity = "high"
	debug = false
	outputPath = ""
	depPathPURL = ""
}

// --- Persistent flags (root command) ---

func TestRootCmd_DefaultSeverity(t *testing.T) {
	resetFlags()
	if err := rootCmd.ParseFlags([]string{}); err != nil {
		t.Fatal(err)
	}
	if severity != "high" {
		t.Errorf("expected default severity 'high', got %q", severity)
	}
}

func TestRootCmd_DefaultDebug(t *testing.T) {
	resetFlags()
	if err := rootCmd.ParseFlags([]string{}); err != nil {
		t.Fatal(err)
	}
	if debug != false {
		t.Error("expected debug to default to false")
	}
}

func TestRootCmd_SeverityFlag(t *testing.T) {
	levels := []string{"all", "critical", "high", "medium", "low"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			resetFlags()
			if err := rootCmd.ParseFlags([]string{"--severity", level}); err != nil {
				t.Fatal(err)
			}
			if severity != level {
				t.Errorf("expected severity %q, got %q", level, severity)
			}
		})
	}
}

func TestRootCmd_DebugFlag(t *testing.T) {
	resetFlags()
	if err := rootCmd.ParseFlags([]string{"--debug"}); err != nil {
		t.Fatal(err)
	}
	if !debug {
		t.Error("expected debug to be true")
	}
}

func TestRootCmd_SeverityAndDebugTogether(t *testing.T) {
	resetFlags()
	if err := rootCmd.ParseFlags([]string{"--severity", "critical", "--debug"}); err != nil {
		t.Fatal(err)
	}
	if severity != "critical" {
		t.Errorf("expected severity 'critical', got %q", severity)
	}
	if !debug {
		t.Error("expected debug to be true")
	}
}

// --- Subcommand registration ---

func TestSubcommands_AllRegistered(t *testing.T) {
	want := map[string]bool{
		"cve":      false,
		"cve-file": false,
		"sbom":     false,
		"version":  false,
	}
	for _, cmd := range rootCmd.Commands() {
		want[cmd.Name()] = true
	}
	for name, found := range want {
		if !found {
			t.Errorf("subcommand %q not registered on root", name)
		}
	}
}

// --- Args validation (ExactArgs(1)) ---

func TestCveCmd_ArgsValidation(t *testing.T) {
	if err := cveCmd.Args(cveCmd, []string{}); err == nil {
		t.Error("expected error for 0 args, got nil")
	}
	if err := cveCmd.Args(cveCmd, []string{"CVE-2021-44228", "CVE-2021-0001"}); err == nil {
		t.Error("expected error for 2 args, got nil")
	}
	if err := cveCmd.Args(cveCmd, []string{"CVE-2021-44228"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

func TestCveFileCmd_ArgsValidation(t *testing.T) {
	if err := cveFileCmd.Args(cveFileCmd, []string{}); err == nil {
		t.Error("expected error for 0 args, got nil")
	}
	if err := cveFileCmd.Args(cveFileCmd, []string{"a.txt", "b.txt"}); err == nil {
		t.Error("expected error for 2 args, got nil")
	}
	if err := cveFileCmd.Args(cveFileCmd, []string{"cve-list.txt"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

func TestSbomCmd_ArgsValidation(t *testing.T) {
	if err := sbomCmd.Args(sbomCmd, []string{}); err == nil {
		t.Error("expected error for 0 args, got nil")
	}
	if err := sbomCmd.Args(sbomCmd, []string{"a.json", "b.json"}); err == nil {
		t.Error("expected error for 2 args, got nil")
	}
	if err := sbomCmd.Args(sbomCmd, []string{"sbom.json"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

// --- sbom subcommand flags ---

func TestSbomCmd_DefaultFlags(t *testing.T) {
	resetFlags()
	if err := sbomCmd.Flags().Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	if outputPath != "" {
		t.Errorf("expected empty outputPath, got %q", outputPath)
	}
	if depPathPURL != "" {
		t.Errorf("expected empty depPathPURL, got %q", depPathPURL)
	}
}

func TestSbomCmd_OutputFlag(t *testing.T) {
	resetFlags()
	if err := sbomCmd.Flags().Parse([]string{"--output", "results.json"}); err != nil {
		t.Fatal(err)
	}
	if outputPath != "results.json" {
		t.Errorf("expected outputPath 'results.json', got %q", outputPath)
	}
}

func TestSbomCmd_DepPathFlag(t *testing.T) {
	resetFlags()
	if err := sbomCmd.Flags().Parse([]string{"--dep-path", "pkg:npm/express@4.17.1"}); err != nil {
		t.Fatal(err)
	}
	if depPathPURL != "pkg:npm/express@4.17.1" {
		t.Errorf("expected depPathPURL 'pkg:npm/express@4.17.1', got %q", depPathPURL)
	}
}

func TestSbomCmd_OutputAndDepPathTogether(t *testing.T) {
	resetFlags()
	if err := sbomCmd.Flags().Parse([]string{"--output", "out.json", "--dep-path", "pkg:npm/qs@4.0.0"}); err != nil {
		t.Fatal(err)
	}
	if outputPath != "out.json" {
		t.Errorf("expected outputPath 'out.json', got %q", outputPath)
	}
	if depPathPURL != "pkg:npm/qs@4.0.0" {
		t.Errorf("expected depPathPURL 'pkg:npm/qs@4.0.0', got %q", depPathPURL)
	}
}

// --- Command routing via Find ---

func TestCommandRouting_CveFound(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"cve", "CVE-2021-44228"})
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if cmd.Name() != "cve" {
		t.Errorf("expected command 'cve', got %q", cmd.Name())
	}
}

func TestCommandRouting_CveFileFound(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"cve-file", "cve-list.txt"})
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if cmd.Name() != "cve-file" {
		t.Errorf("expected command 'cve-file', got %q", cmd.Name())
	}
}

func TestCommandRouting_SbomFound(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"sbom", "sbom.json"})
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if cmd.Name() != "sbom" {
		t.Errorf("expected command 'sbom', got %q", cmd.Name())
	}
}

func TestCommandRouting_VersionFound(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"version"})
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if cmd.Name() != "version" {
		t.Errorf("expected command 'version', got %q", cmd.Name())
	}
}

// --- Version variable ---

func TestVersion_IsSet(t *testing.T) {
	if Version == "" {
		t.Error("Version must not be empty")
	}
}
