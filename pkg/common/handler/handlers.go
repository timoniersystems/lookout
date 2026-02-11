package handler

import (
	"defender/pkg/common/cyclonedx"
	"defender/pkg/common/nvd"
	"defender/pkg/common/processor"
	"defender/pkg/common/trivy"
	"defender/pkg/gui/dgraph"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

type TemplateData struct {
	CVEs []nvd.CVEPURLPair
}

type ResultsTemplateData struct {
	CVEIDs      []string
	SelectedCVE *nvd.CVEData
}

func CVES(c echo.Context) error {
	cveID := c.FormValue("cveID")

	if cveID != "" {
		data, err := nvd.FetchCVEData(cveID)
		if err != nil {
			fmt.Printf("Error with the CVE Data.")

		}
		return c.Render(http.StatusOK, "cve_details.html", data)
	}

	file, err := c.FormFile("file")
	if err == nil && file != nil {
		src, err := file.Open()
		if err != nil {
			log.Printf("Failed to open the uploaded file: %v", err)
			return err
		}
		defer src.Close()

		tempFile, err := os.CreateTemp("", "upload-*"+filepath.Ext(file.Filename))
		if err != nil {
			log.Printf("Failed to create a temporary file: %v", err)
			return err
		}
		defer os.Remove(tempFile.Name())

		_, err = io.Copy(tempFile, src)
		if err != nil {
			log.Printf("Failed to copy the uploaded file to the temporary file: %v", err)
			return err
		}

		if err := tempFile.Close(); err != nil {
			log.Printf("Failed to close the temporary file: %v", err)
			return err
		}

		cveIDs, err := processor.ProcessFileInputForCVEs(tempFile.Name())
		if err != nil {
			log.Printf("Failed to extract CVE IDs from the uploaded file: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "Failed to extract CVE IDs",
			})
		}

		return c.Render(http.StatusOK, "cve_list.html", map[string]interface{}{
			"CVEs": cveIDs,
		})
	}

	return c.JSON(http.StatusBadRequest, map[string]interface{}{
		"error": "No CVE ID or file provided",
	})
}

func ProcessCVE(c echo.Context) error {
	cveID := c.FormValue("cveID")

	data, err := nvd.FetchCVEData(cveID)

	if err != nil {
		fmt.Printf(err.Error())

	}

	return c.Render(http.StatusOK, "results.html", data)
}

func UploadAndProcess(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("Failed to retrieve the file from request: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Failed to retrieve the file",
		})
	}

	src, err := file.Open()
	if err != nil {
		log.Printf("Failed to open the uploaded file: %v", err)
		return err
	}
	defer src.Close()

	ext := filepath.Ext(file.Filename)
	tempFile, err := os.CreateTemp("", "upload-*"+ext)
	if err != nil {
		log.Printf("Failed to create a temporary file: %v", err)
		return err
	}
	tempFilePath := tempFile.Name()
	log.Printf("Uploaded file saved to: %s", tempFilePath)
	defer os.Remove(tempFilePath)

	_, err = io.Copy(tempFile, src)
	if err != nil {
		log.Printf("Failed to copy the uploaded file to the temporary file: %v", err)
		return err
	}

	if err := tempFile.Close(); err != nil {
		log.Printf("Failed to close the temporary file: %v", err)
		return err
	}

	cvePurlMap, err := processor.ProcessFileInput(tempFilePath)
	if err != nil {
		log.Printf("Failed to process the uploaded file at %s: %v", tempFilePath, err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to process uploaded file",
		})
	}

	aggregatedData := nvd.AggregateCVEData(cvePurlMap)

	var pageData nvd.ResultsPageData
	for _, data := range aggregatedData {
		pageData.CVEPURLPairs = append(pageData.CVEPURLPairs, data)
	}

	return c.Render(http.StatusOK, "cve_results.html", pageData)
}

func RunTrivyAndProcess(c echo.Context) error {
	if !trivy.CheckTrivyInstalled() {
		log.Println("Please install Trivy before running this application.")
		return c.JSON(http.StatusBadRequest, "error: Please install Trivy before running the application.")
	}

	file, err := c.FormFile("sbom-file")
	if err != nil {
		log.Printf("Failed to retrieve the file from request: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Failed to retrieve the file",
		})
	}

	src, err := file.Open()
	if err != nil {
		log.Printf("Failed to open the uploaded file: %v", err)
		return err
	}
	defer src.Close()

	ext := filepath.Ext(file.Filename)
	tempFile, err := os.CreateTemp("", "upload-*"+ext)
	if err != nil {
		log.Printf("Failed to create a temporary file: %v", err)
		return err
	}
	tempFilePath := tempFile.Name()
	log.Printf("Uploaded file saved to: %s", tempFilePath)
	defer os.Remove(tempFilePath)

	_, err = io.Copy(tempFile, src)
	if err != nil {
		log.Printf("Failed to copy the uploaded file to the temporary file: %v", err)
		return err
	}

	if err := tempFile.Close(); err != nil {
		log.Printf("Failed to close the temporary file: %v", err)
		return err
	}

	trivyResults, err := trivy.RunTrivy(tempFilePath)

	cvePurlMap, err := processor.ProcessFileInput(trivyResults)
	if err != nil {
		log.Printf("Failed to process the uploaded file at %s: %v", tempFilePath, err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to process uploaded file",
		})
	}

	aggregatedData := nvd.AggregateCVEData(cvePurlMap)

	var pageData nvd.ResultsPageData
	for _, data := range aggregatedData {
		pageData.CVEPURLPairs = append(pageData.CVEPURLPairs, data)
	}

	return c.Render(http.StatusOK, "cve_results.html", pageData)

}

func UploadBOMAndInsertData(c echo.Context) error {
	client := dgraph.DgraphClient()
	err := dgraph.DropAllData(client)

	file, err := c.FormFile("cyclonedx-bom-file")
	if err != nil {
		log.Printf("Failed to retrieve the BOM file from request: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Failed to retrieve the BOM file",
		})
	}

	src, err := file.Open()
	if err != nil {
		log.Printf("Failed to open the uploaded BOM file: %v", err)
		return err
	}
	defer src.Close()

	tempFile, err := os.CreateTemp("", "bom-*"+filepath.Ext(file.Filename))
	if err != nil {
		log.Printf("Failed to create a temporary file for the BOM: %v", err)
		return err
	}
	defer os.Remove(tempFile.Name())

	_, err = io.Copy(tempFile, src)
	if err != nil {
		log.Printf("Failed to copy the BOM file to the temporary file: %v", err)
		return err
	}

	if err := tempFile.Close(); err != nil {
		log.Printf("Failed to close the temporary BOM file: %v", err)
		return err
	}

	bom, err := cyclonedx.ParseBOM(tempFile.Name())
	if err != nil {
		log.Printf("Failed to parse BOM file: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to parse BOM file",
		})
	}

	if err := dgraph.InsertComponentsAndDependencies(dgraph.DgraphClient(), bom); err != nil {
		log.Printf("Failed to insert BOM data into Dgraph: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to insert BOM data into Dgraph",
		})
	}

	trivyResults, err := trivy.RunTrivy(tempFile.Name())

	cvePurlMap, err := processor.ProcessFileInput(trivyResults)
	if err != nil {
		log.Printf("Failed to process the uploaded file at %s: %v", tempFile.Name(), err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to process uploaded file",
		})
	}

	dgraph.QueryAndUpdatePurl(cvePurlMap)
	aggregatedData := nvd.AggregateCVEData(cvePurlMap)

	resultMap, err := dgraph.RetrieveVulnerablePURLs(cvePurlMap)
	if err != nil {
		log.Fatal(err)
	}

	var pageData nvd.ResultsPageData

	for _, data := range aggregatedData {

		purl := strings.TrimSpace(strings.ToLower(data.PURL))

		for _, component := range resultMap {
			componentPurl := strings.TrimSpace(strings.ToLower(component.Purl))
			if componentPurl == purl {
				data.DgraphData = component
				break
			}
		}
		pageData.CVEPURLPairs = append(pageData.CVEPURLPairs, data)

	}

	return c.Render(http.StatusOK, "cve_vulnerability_results.html", pageData)
}

func PurlTraversal(c echo.Context) error {
	client := dgraph.DgraphClient()
	err := dgraph.DropAllData(client)
	if err != nil {
		log.Fatalf("Error clearing data: %v", err)
	}

	pURL := c.QueryParam("pURL")
	if pURL == "" {
		log.Println("Error: pURL is required")
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "pURL is required",
		})
	}
	log.Printf("Received pURL: %s", pURL)

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid request body",
		})
	}

	tempFile, err := os.CreateTemp("", "upload-*.json")
	if err != nil {
		log.Printf("Failed to create temporary file: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to create temporary file",
		})
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write(body); err != nil {
		log.Printf("Failed to write to temporary file: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to write JSON to file",
		})
	}

	if err := tempFile.Close(); err != nil {
		log.Printf("Failed to close temporary file: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to close temporary file",
		})
	}
	bom, err := cyclonedx.ParseBOM(tempFile.Name())
	if err != nil {
		log.Printf("Failed to parse BOM file: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to parse BOM file",
		})
	}

	if err := dgraph.InsertComponentsAndDependencies(dgraph.DgraphClient(), bom); err != nil {
		log.Printf("Failed to insert BOM data into Dgraph: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to insert BOM data into Dgraph",
		})
	}

	resultMap, err := dgraph.RetrievePURL(pURL)
	if err != nil {
		log.Printf("Error retrieving component for pURL %s: %v", pURL, err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve component data",
		})
	}

	log.Printf("Final retrieved data for pURL %s: %v", pURL, resultMap)

	return c.JSON(http.StatusOK, resultMap)
}
