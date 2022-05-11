package Rpm

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
	Type    string  `xml:"type,attr"`
	Name    string  `xml:"name"`
	Arch    string  `xml:"arch"`
	Version Version `xml:"version"`
}

type Version struct {
	Version string `xml:"ver,attr"`
	Release string `xml:"rel,attr"`
	Epoch   string `xml:"epoch,attr"`
}

type Repomd struct {
	XMLName xml.Name `xml:"repomd"`
	Data    []Data   `xml:"data"`
}

type Data struct {
	Type     string   `xml:"type,attr"`
	Location Location `xml:"location"`
}
type Location struct {
	Href string `xml:"href,attr"`
}

// Returns an array of package metadata information when given an Rpm Repo.
func Extract(url string) ([]Package, error) {
	primaryUrl, err := GetPrimaryUrlFromRepomdUrl(url)

	if err != nil {
		return []Package{}, fmt.Errorf("GET error: %v", err)
	}

	return GetXmlDataFromPrimaryUrl(primaryUrl)
}

func GetPrimaryUrlFromRepomdUrl(url string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/repodata/repomd.xml", url))
	if err != nil {
		return "", fmt.Errorf("GET error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status error: %v", resp.StatusCode)
	}

	byteValue, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("read body: %v", err)
	}

	var result Repomd
	xml.Unmarshal(byteValue, &result)

	var primaryLocation string
	for _, data := range result.Data {
		if data.Type == "primary" {
			primaryLocation = data.Location.Href
		}
	}

	if primaryLocation == "" {
		return "", fmt.Errorf("GET error: Unable to parse 'primary' location in repomd.xml")
	}

	return fmt.Sprintf("%s/%s", url, primaryLocation), nil
}

func GetXmlDataFromPrimaryUrl(url string) ([]Package, error) {
	resp, err := http.Get(url)

	if err != nil {
		return []Package{}, fmt.Errorf("GET error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return []Package{}, fmt.Errorf("status error: %v", resp.StatusCode)
	}

	return ParseXMLData(resp.Body)
}

func ParseXMLData(body io.ReadCloser) ([]Package, error) {
	reader, err := gzip.NewReader(body)

	if err != nil {
		fmt.Println(err)
	}

	decoder := xml.NewDecoder(reader)
	reader.Close()

	result := []Package{}
	for {
		// Read tokens from the XML document in a stream.
		t, err := decoder.Token()

		// If we are at the end of the file, we are done
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Error decoding token: %s", err)
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

				if err = decoder.DecodeElement(&pkg, &elType); err != nil {
					log.Fatalf("Error decoding pkg: %s", err)
				}
				// Ensure that the type is "rpm" before pushing our array
				if pkg.Type != "rpm" {
					fmt.Printf("package found of tpye %v\n", pkg.Type)
					break
				}
				result = append(result, pkg)
			}
		}
	}

	return result, err
}
