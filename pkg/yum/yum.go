package yum

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

type Package struct {
	Type     string   `xml:"type,attr"`
	Name     string   `xml:"name"`
	Arch     string   `xml:"arch"`
	Version  Version  `xml:"version"`
	Checksum Checksum `xml:"checksum"`
	Summary  string   `xml:"summary"`
}

type Version struct {
	Version string `xml:"ver,attr"`
	Release string `xml:"rel,attr"`
	Epoch   int32  `xml:"epoch,attr"`
}

type Checksum struct {
	Value string `xml:",chardata"`
	Type  string `xml:"type,attr"`
}

type Repomd struct {
	XMLName  xml.Name `xml:"repomd"`
	Data     []Data   `xml:"data"`
	Revision string   `xml:"revision"`
}

type Data struct {
	Type     string   `xml:"type,attr"`
	Location Location `xml:"location"`
}
type Location struct {
	Href string `xml:"href,attr"`
}

// ExtractPackageData returns array of package information given URL to a yum repo.
func ExtractPackageData(client http.Client, url string) ([]Package, error) {
	var repomd Repomd
	var primaryURL string
	var err error

	if repomd, err = GetRepomdXML(client, url); err != nil {
		return []Package{}, fmt.Errorf("Error parsing repomd.xml: %v", err)
	}

	if primaryURL, err = GetPrimaryURL(repomd, url); err != nil {
		return []Package{}, fmt.Errorf("Error getting primary URL: %v", err)
	}

	if err != nil {
		return []Package{}, err
	}

	packages, err := GetPackagesArrayWithPrimaryURL(client, primaryURL)

	return packages, err
}

func GetPrimaryURL(repomd Repomd, url string) (string, error) {
	var primaryLocation string
	for _, data := range repomd.Data {
		if data.Type == "primary" {
			primaryLocation = data.Location.Href
		}
	}

	if primaryLocation == "" {
		return "", fmt.Errorf("GET error: Unable to parse 'primary' location in repomd.xml")
	}

	return fmt.Sprintf("%s/%s", url, primaryLocation), nil
}

// GetRepomdXML returns Repomd struct given http client and URL to yum repository
func GetRepomdXML(client http.Client, url string) (Repomd, error) {
	var result Repomd

	resp, err := client.Get(fmt.Sprintf("%s/repodata/repomd.xml", url))
	if err != nil {
		return Repomd{}, fmt.Errorf("GET error: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return Repomd{}, fmt.Errorf("Status error: %v", resp.StatusCode)
	}

	if result, err = parseRepomdXML(resp.Body); err != nil {
		resp.Body.Close()
		return Repomd{}, fmt.Errorf("error parsing repomd.xml: %w", err)
	}
	resp.Body.Close()

	return result, err
}

func parseRepomdXML(body io.ReadCloser) (Repomd, error) {
	var result Repomd

	byteValue, err := ioutil.ReadAll(body)
	if err != nil {
		return Repomd{}, fmt.Errorf("io.reader read failure: %w", err)
	}

	err = xml.Unmarshal(byteValue, &result)
	if err != nil {
		return Repomd{}, fmt.Errorf("xml.Unmarshal failure: %w", err)
	}

	return result, err
}

// Returns an array of package information when given the primary repo URL.
func GetPackagesArrayWithPrimaryURL(client http.Client, url string) ([]Package, error) {
	resp, err := client.Get(url)

	if err != nil {
		return []Package{}, fmt.Errorf("GET error: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return []Package{}, fmt.Errorf("status error: %v", resp.StatusCode)
	}

	packages, err := ParseCompressedXMLData(resp.Body)

	resp.Body.Close()

	return packages, err
}

// Unzips a gzipped body response, then parses the contained XML for package information
// Returns an array of package data
func ParseCompressedXMLData(body io.ReadCloser) ([]Package, error) {
	reader, err := gzip.NewReader(body)

	if err != nil {
		return []Package{}, fmt.Errorf("Error unzipping response body: %w", err)
	}

	decoder := xml.NewDecoder(reader)
	reader.Close()

	result := []Package{}

	for {
		// Read tokens from the XML document in a stream.
		t, decodeError := decoder.Token()

		// If we are at the end of the file, we are done
		if decodeError == io.EOF {
			break
		} else if decodeError != nil {
			return []Package{}, fmt.Errorf("Error decoding token: %w", decodeError)
		} else if t == nil {
			break
		}

		// Here, we inspect the token
		switch elType := t.(type) {
		case xml.StartElement:
			switch elType.Name.Local {
			// Found an item, so we process it
			case "package":
				var pkg Package
				if decodeElementError := decoder.DecodeElement(&pkg, &elType); decodeElementError != nil {
					log.Fatalf("Error decoding pkg: %s", decodeElementError)
					break
				}
				// Ensure that the type is "rpm" before pushing our array
				if pkg.Type != "rpm" {
					fmt.Printf("package found of type %v\n", pkg.Type)
					break
				}
				result = append(result, pkg)
			}
		}
	}

	return result, nil
}
