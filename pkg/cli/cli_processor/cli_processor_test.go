package cli_processor

import (
	"testing"
)

func TestParseCLIArgs_CVEIDFlag(t *testing.T) {
	args := []string{"-cve", "CVE-2021-36159"}
	result := ParseCLIArgs(args)

	if result.CVEID != "CVE-2021-36159" {
		t.Errorf("Expected CVEID to be 'CVE-2021-36159', got '%s'", result.CVEID)
	}

	if result.CVEFilePath != "" {
		t.Errorf("Expected CVEFilePath to be empty, got '%s'", result.CVEFilePath)
	}
}

func TestParseCLIArgs_CVEFilePathFlag(t *testing.T) {
	args := []string{"-cve-file", "./test.txt"}
	result := ParseCLIArgs(args)

	if result.CVEFilePath != "./test.txt" {
		t.Errorf("Expected CVEFilePath to be './test.txt', got '%s'", result.CVEFilePath)
	}

	if result.CVEID != "" {
		t.Errorf("Expected CVEID to be empty, got '%s'", result.CVEID)
	}
}

func TestParseCLIArgs_SBOMPathFlag(t *testing.T) {
	args := []string{"-sbom", "./sbom.json"}
	result := ParseCLIArgs(args)

	if result.SBOMPath != "./sbom.json" {
		t.Errorf("Expected SBOMPath to be './sbom.json', got '%s'", result.SBOMPath)
	}
}

func TestParseCLIArgs_SBOMWithOutputFile(t *testing.T) {
	args := []string{"-sbom", "./sbom.json", "-output", "output.json"}
	result := ParseCLIArgs(args)

	if result.SBOMPath != "./sbom.json" {
		t.Errorf("Expected SBOMPath to be './sbom.json', got '%s'", result.SBOMPath)
	}

	if result.OutputPath != "output.json" {
		t.Errorf("Expected OutputPath to be 'output.json', got '%s'", result.OutputPath)
	}
}

func TestParseCLIArgs_DepPath(t *testing.T) {
	args := []string{"-sbom", "./sbom.json", "-dep-path", "pkg:npm/qs@4.0.0"}
	result := ParseCLIArgs(args)

	if result.SBOMPath != "./sbom.json" {
		t.Errorf("Expected SBOMPath to be './sbom.json', got '%s'", result.SBOMPath)
	}

	if result.DepPathPURL != "pkg:npm/qs@4.0.0" {
		t.Errorf("Expected DepPathPURL to be 'pkg:npm/qs@4.0.0', got '%s'", result.DepPathPURL)
	}
}

func TestParseCLIArgs_MultipleFlagsDoNotInterfere(t *testing.T) {
	args := []string{"-cve", "CVE-2021-36159", "-cve-file", "./test.txt"}
	result := ParseCLIArgs(args)

	if result.CVEID != "CVE-2021-36159" {
		t.Errorf("Expected CVEID to be 'CVE-2021-36159', got '%s'", result.CVEID)
	}

	if result.CVEFilePath != "./test.txt" {
		t.Errorf("Expected CVEFilePath to be './test.txt', got '%s'", result.CVEFilePath)
	}
}

func TestParseCLIArgs_EmptyArgs(t *testing.T) {
	args := []string{}
	result := ParseCLIArgs(args)

	if result.CVEID != "" {
		t.Errorf("Expected CVEID to be empty, got '%s'", result.CVEID)
	}

	if result.CVEFilePath != "" {
		t.Errorf("Expected CVEFilePath to be empty, got '%s'", result.CVEFilePath)
	}

	if result.SBOMPath != "" {
		t.Errorf("Expected SBOMPath to be empty, got '%s'", result.SBOMPath)
	}

	if result.DepPathPURL != "" {
		t.Errorf("Expected DepPathPURL to be empty, got '%s'", result.DepPathPURL)
	}
}

func TestParseCLIArgs_DefaultValues(t *testing.T) {
	args := []string{"-cve", "CVE-2021-36159"}
	result := ParseCLIArgs(args)

	if result.OutputPath != "" {
		t.Errorf("Expected OutputPath to be empty, got '%s'", result.OutputPath)
	}

	if result.DepPathPURL != "" {
		t.Errorf("Expected DepPathPURL to be empty, got '%s'", result.DepPathPURL)
	}

	if result.SBOMPath != "" {
		t.Errorf("Expected SBOMPath to be empty, got '%s'", result.SBOMPath)
	}
}

func TestCLIArgs_Struct(t *testing.T) {
	cliArgs := CLIArgs{
		CVEID:       "CVE-2021-36159",
		CVEFilePath: "./test.txt",
		SBOMPath:    "./sbom.json",
		OutputPath:  "output.json",
		DepPathPURL: "pkg:npm/qs@4.0.0",
	}

	if cliArgs.CVEID != "CVE-2021-36159" {
		t.Errorf("Expected CVEID to be 'CVE-2021-36159', got '%s'", cliArgs.CVEID)
	}

	if cliArgs.CVEFilePath != "./test.txt" {
		t.Errorf("Expected CVEFilePath to be './test.txt', got '%s'", cliArgs.CVEFilePath)
	}

	if cliArgs.SBOMPath != "./sbom.json" {
		t.Errorf("Expected SBOMPath to be './sbom.json', got '%s'", cliArgs.SBOMPath)
	}

	if cliArgs.OutputPath != "output.json" {
		t.Errorf("Expected OutputPath to be 'output.json', got '%s'", cliArgs.OutputPath)
	}

	if cliArgs.DepPathPURL != "pkg:npm/qs@4.0.0" {
		t.Errorf("Expected DepPathPURL to be 'pkg:npm/qs@4.0.0', got '%s'", cliArgs.DepPathPURL)
	}
}

func TestVersion_IsSet(t *testing.T) {
	// Verify that the Version constant is defined and not empty
	if Version == "" {
		t.Error("Expected Version constant to be set")
	}

	// Verify version is set to a valid value
	if Version != "1.0" {
		t.Errorf("Expected Version to be '1.0', got '%s'", Version)
	}

	t.Logf("CLI Version: %s", Version)
}

// Note: Testing -h, -help, and -version flags is done manually since they call os.Exit()
// Manual test commands:
//   go run cmd/cli/main.go -h
//   go run cmd/cli/main.go -help
//   go run cmd/cli/main.go -version
