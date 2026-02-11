# Defender (CVE Data Extractor)

## Overview

This Go application is designed for security professionals and developers to extract CVE (Common Vulnerabilities and Exposures) data from the NVD (National Vulnerability Database) through various sources and formats. Its primary function is to analyze and report on vulnerabilities identified in software components, making it an invaluable tool for maintaining the security integrity of applications.

### Key Features

#### CLI

- **Trivy JSON Support:** The application can consume JSON files exported from [Trivy](https://github.com/aquasecurity/trivy), a comprehensive vulnerability scanner for containers. It adheres to the structure outputted by Trivy, enabling users to directly use scan results for further analysis.
- **Text File Processing:** It supports processing plain text files, where each line contains a CVE ID. Optionally, users can include a Package URL (PURL) for each CVE ID by separating them with a comma. This feature allows for the integration of CVE data from various textual sources.
- **Interactive CLI:** The application offers a command-line interface (CLI) for quick data extraction tasks, catering to users who prefer working in a terminal environment.
- **Path Traversal JSON Visualizer:** The application can output a JSON set of data containing a traversal path from a pURL to the root project when provided a CycloneDX SBOM and said pURL via an API call to localhost:3000/purl-traversal or by using the CLI below, which will also return a text based version of the JSON response to the CLI.

#### GUI

- **Trivy JSON Support:** The application can consume JSON files exported from [Trivy](https://github.com/aquasecurity/trivy), a comprehensive vulnerability scanner for containers. It adheres to the structure outputted by Trivy, enabling users to directly use scan results for further analysis.
- **Text File Processing:** It supports processing plain text files, where each line contains a CVE ID. Optionally, users can include a Package URL (PURL) for each CVE ID by separating them with a comma. This feature allows for the integration of CVE data from various textual sources.
- **CycloneDX SBOM Processing and Vulnerability Integration:** The GUI can process CycloneDX Software Bill of Materials (SBOMs), generating a graphical view of the components and their connected dependencies via Dgraph. It also integrates with Trivy to find CVEs connected to the components (PURLs) listed in the SBOM in order to generate a traversal from vulnerability to the root application.
- **Visual and Text-based Output of Reverse Vulnerability Traversal:** The GUI displays a visual reverse traversal from the vulnerable component back to the root of the project using Dgraph. Additionally, it provides a text-based output for detailed analysis.


## Installation

Before you begin, ensure you have Go and Trivy installed on your system if you plan to utilize the CLI.  

### Installing Go
[Download and install Go](https://golang.org/dl/)

### Installing Trivy
Trivy can be installed on various operating systems. Visit the [Trivy GitHub repository](https://github.com/aquasecurity/trivy) for detailed installation instructions. For most users, the following commands should suffice:

#### On macOS:
```bash
brew install trivy
```

#### On Ubuntu/Debian:
```bash
sudo apt-get install -y trivy
```

#### On Windows:
Download the latest release from the [Releases page](https://github.com/aquasecurity/trivy/releases) and add the executable to your PATH.

#### Verify Trivy Installed:
```
trivy --version
```

To set up Defender to run its CLI commands:

1. Clone the repository:
  ```bash
   git clone git@github.com:timoniersystems/defender.git
   cd defender
  ```

2. Install the binary using the Makefile:
  ```bash
   make build
   make install
   make clean
  ```

3. Alternatively, Build the application manually (optional):
  ```bash
   go build ./cmd/server/
  ```

4. Ensure defender has been installed by checking your path or running `defender` from the CLI.

To setup Defender to run via Docker for GUI interaction:

1. Clone the repository:
  ```bash
   git clone git@github.com:timoniersystems/defender.git
   cd defender
  ```

2. Run Defender via Docker with:
  ```bash
   docker-compose up
  ```


## Usage

### Running the CLI

- To extract CVE data for a specific ID and display the results on the CLI:
  ```bash
   defender -cve CVE-2021-36159
  ```

- To extract CVE data from a specified file (supports both Trivy JSON and plain text formats) and display results on the CLI:

  For Trivy JSON file input:
  ```bash
   defender -f ./examples/trivy-results.json
  ```
  Replace `./examples/trivy-results.json` with the path to your input file.

  For text file input:
  ```bash
   defender -f ./examples/text-file-example.txt
  ```
  Replace `./examples/text-file-example.txt` with the path to your input file.

- To pass an SBOM of any format into Trivy for processing:
  ```bash
   defender -sbom ./examples/cyclonedx-sbom-example.json
  ```
  Replace `./examples/cyclonedx-sbom-example.json` with the path to your input file.

- To pass an SBOM of any format into Trivy for processing as well as output the results of the scan to a file:
  ```bash
   defender -sbom ./examples/cyclonedx-sbom-example.json -o name-of-file.json
  ```
  Replace `./examples/cyclonedx-sbom-example.json` with the path to your input file and `name-of-file.json` to what you would like to name the results file. By default, the file is stored in `defender/outputs`, outputs being generated automatically if it doesn't already exist within the application's root dir.


- To pass an SBOM of CycloneDX format and a pURL for getting a traversal path from pURL to root project in JSON:

  ```bash
   docker-compose up
   defender -traversal -file ./examples/cyclonedx-sbom-example.json -purl pkg:composer/asm89/stack-cors@1.3.0
  ```

 - Example Output

  ```bash
    {“searched_purl”:“pkg:maven/com.sun.istack/istack-commons-runtime@4.0.1?type=jar”,“path_to_root_package”:[“pkg:maven/com.sun.istack/istack-commons-runtime@4.0.1?type=jar”,“pkg:maven/org.glassfish.jaxb/jaxb-core@3.0.2?type=jar”,“pkg:maven/org.glassfish.jaxb/jaxb-runtime@3.0.2?type=jar”,“pkg:maven/org.hibernate.orm/hibernate-core@6.1.3.Final?type=jar”,“pkg:maven/com.source.sample.datastore/satellite-position@2.3.12?type=jar”,“sample/java-project@root”]}
  ```

### Running the GUI

- To start the application with a graphical interface and the additional features: :
  ```bash
   docker-compose up
  ```
  After the container is up and running, navigate to `localhost:3000` in your web browser. This will launch the GUI, allowing you to input a CVE ID, select a CycloneDX SBOM file for vulnerability pathing and CVE Data Display, or provide a a text based file or SBOM of any type in JSON to process and display CVE data. The results will be displayed after submission.

  When working with CVE IDs, the NVD database can cause things to run slowly due to API calls. If you have a CycloneDX file with a found large amount of CVEs, the processing time can take a considerable amount of time at present.
  
  #### Individual CVE ID Field
  This Input Field allows you to pass in an individual CVE ID in order to display relevant data from the NVD regarding said CVE ID on a results page.

  #### Trivy (JSON format) or Listed Text File Containing CVE IDs and (Optionally) Their PURLs
  This input field allows you to pass in an already processed Trivy file in JSON format or a Listed Text File containing CVE IDs and (Optionally) their PURLs, the results of which being displayed on a results page the user is directed to. Examples can be found at: 

  `/examples/trivy-result-example.json` and `text-file-example.txt`

  #### SBOM (any type, JSON format) to Run Through Trivy and Process CVE Data
  This input field allows you to pass in an SBOM of any type in JSON format to then get passed into Trivy and eventually processed through the NVD to provide relevant vulnerabiltiy data for said SBOM on a results page. Example can be found at:

  `/examples/guac-file-example.spdx.json`

  #### SBOM (CycloneDX type, JSON format) to Process via Trivy. Produces Dependency Graph and Vulnerability Tracing if Provided in JSON
  This input field allows you to pass in a CycloneDX SBOM formatted in the component style to be processed by Trivy as well as inserted into dgraph for storage of dependency relevant information between components if any exists. This data is also passed into NVD to determine any vulnerabilites and, if found, will display a reverse traversal from the vulnerabiltiy package to the root (applicaiton). Example file can be found at:

  `/examples/cyclonedx-sbom-example.json`

  ##### Based Traversal Text Example:
    ```bash
      Traversal Path:
        pkg:npm/qs@4.0.0 (Vulnerable Package)
        pkg:npm/express@4.13.4
        Vulnerable Sample Project@root (Root Project)
    ```

  When passing in a CycloneDX SBOM file for vulnerability pathing, the results page will display a text based traversal of the found vulnerability based on the CVE ID in the header. This text based traversal works in reverse, starting at the source and displaying the shortest path to the root (the application itself.) An example is shown below.

  If you understand how to make GraphQL queries and are uploading a CycloneDX SBOM file for vulnerability pathing, there is also a visual display of the data via dgraph at `localhost:8000`. For a specified vulnerability's pURL and CVE ID, you can implement the following query and expand on/shorten it in order to view the path from the source vulnerability to the root project. 

  query:
  ```bash
  {component(func: eq(purl, "name-of-purl")) @filter(eq(cveID, "cve-id-attached-to-purl")) {
        uid
        name
        purl
        cveID
        version
        vulnerable
        bomRef
        reference
        root
        dependsOn {
          uid
          name
          purl
          cveID
          version
          vulnerable
          bomRef
          reference
          root
          dependsOn {
            uid
            name
            purl
            cveID
            version
            vulnerable
            bomRef
            reference
            root
            dependsOn {
              uid
              name
              purl
              cveID
              version
              vulnerable
              bomRef
              reference
              root
              dependsOn {
                uid
                name
                purl
                cveID
                version
                vulnerable
                bomRef
                reference
                root
              }
            }
          }
        }
        ~dependsOn {
          uid
          name
          purl
          cveID
          version
          vulnerable
          bomRef
          reference
          root
          ~dependsOn {
            uid
            name
            purl
            cveID
            version
            vulnerable
            bomRef
            reference
            root
            ~dependsOn {
              uid
              name
              purl
              cveID
              version
              vulnerable
              bomRef
              reference
              root
              ~dependsOn {
                uid
                name
                purl
                cveID
                version
                vulnerable
                bomRef
                reference
                root
            }
          }
        }
      }
    }
  }
  ```

### Input File Formats

The application supports two types of input files:

1. **Trivy JSON Format:** JSON files generated by Trivy scans.
2. **Plain Text Files:** Each line contains a single CVE ID, optionally followed by a PURL, separated by a comma.
3. **SBOM (any type, JSON format):** SBOM file of any type in JSON format that Trivy supports processing of. 
4. **CycloneDX JSON Format:** JSON files generated by CycloneDX in the component structure format for the GUI.

Examples of file formats can be found in the `examples` directory.


## Findings from Testing of Defender v1.0

1. Some CycloneDX Files have a different configuration than what is expected. As an example, most of the files contain a hierarchy of: `components, dependencies -> dependsOn`. Some examples had a different hierarchy of: `services, dependencies -> dependsOn`. A `solution` for this would be to modify Defender so that it can handle both hierarchies.

2. Very large SBOMs, such as tools like KeyCloak's entire application, caused Defender to lag heavily and fail at certain processes. In one scenario, it took Defender over an hour to process all of the CVEs that were found to be connected to the PURLs. That high amount of components and dependencies also causes a resource strain on the Docker container as well as causing the dependsOn connections between components via dependencies to fail as well. A `solution` to this would be to optimize Defender so that it can more efficiently utilize its resources while running, as well as the `solution` found for the next finding as they are related.

3. The more CVEs there are to process, the longer it will take for Defender to complete primarily due to NVD's API. Calls to the API fail often, requiring a retry from Defender that always requires an incremental time increase from the last API call to prevent NVD from fully blocking out Defender from making API calls. A `solution` to this would be to have a local DB on hand containing all of the CVEs and their relevant data. When Defender is run, it can grab all of the latest CVEs, bringing them into the local DB. Increases in the resource allocation to the containers would help with this as well. 

4. Something else that would be potentially useful for v2.0 is a customized query path based on the shortest path traversal found.

5. Also need to implement a purge tool that purges the dgraph db of data when rerunning against files.

## Contributing

Contributions to the CVE Data Extractor are welcome! Please refer to the CONTRIBUTING.md file for guidelines on how to make a contribution.

## License

This project is licensed under the MIT License - see the LICENSE file for details.