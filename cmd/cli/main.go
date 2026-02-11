package cli

import (
	"bytes"
	"defender/pkg/cli/cli_processor"
	"defender/pkg/common/nvd"
	"defender/pkg/common/processor"
	"defender/pkg/common/trivy"
	"fmt"
	"io"
	"net/http"
	"os"

	"log"
)

func RunCLI(args []string) {
	cliArgs := cli_processor.ParseCLIArgs(args)

	if cliArgs.PURLTraversal {

		err := callPurlTraversalAPI(cliArgs.PURL, cliArgs.File)
		if err != nil {
			log.Printf("Error calling PurlTraversal: %v", err)
			os.Exit(1)
		}
		return

	}

	if cliArgs.PURLTraversal && (cliArgs.File == "" || cliArgs.PURL == "") {
		fmt.Println("Error: `-traversal` requires both a PURL and a file path.")
		fmt.Println("Usage: -traversal <purl> <file>")
		os.Exit(1)
	}

	if cliArgs.CVEID != "" {
		cli_processor.ProcessCVEData([]string{cliArgs.CVEID})
	} else if cliArgs.FilePath != "" {
		cvePurlMap, err := processor.ProcessFileInput(cliArgs.FilePath)
		if err != nil {
			log.Printf("No such file or directory: %v", err)
			return
		}
		pairs, err := nvd.FetchCVEDataWithPURLs(cvePurlMap)
		if err != nil {
			log.Printf("Failed to extract CVE IDs from the uploaded file: %v", err)
			return
		}
		cli_processor.ProcessCVEDataWithPURLs(pairs)
	} else if cliArgs.TrivyFilePath != "" {
		if !trivy.CheckTrivyInstalled() {
			log.Println("Please install Trivy before running this application.")
			return
		}

		var trivyResults string
		var err error

		if cliArgs.TrivyResultsFile != "" {
			trivyResults, err = trivy.RunTrivy(cliArgs.TrivyFilePath, cliArgs.TrivyResultsFile)

		} else {
			trivyResults, err = trivy.RunTrivy(cliArgs.TrivyFilePath)
		}

		if err != nil {
			log.Printf("Failed to run Trivy: %v", err)
			return
		}

		cvePurlMap, err := processor.ProcessFileInput(trivyResults)
		pairs, err := nvd.FetchCVEDataWithPURLs(cvePurlMap)
		if err != nil {
			log.Printf("Failed to extract CVE IDs from the uploaded file: %v", err)
			return
		}
		cli_processor.ProcessCVEDataWithPURLs(pairs)

	}
}

func callPurlTraversalAPI(pURL, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, file)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	url := fmt.Sprintf("http://localhost:3000/purl-traversal?pURL=%s", pURL)
	req, err := http.NewRequest("POST", url, &buffer)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("%s", string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK response: %s", resp.Status)
	}

	return nil
}
