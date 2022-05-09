package Rpm

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Primary struct {
	XMLName  xml.Name  `xml:"metadata"`
	Packages []Package `xml:"package"`
}

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
func Extract(url string) (Primary, error) {
	primaryPath, err := getDataFromRepomd(url)
	if err != nil {
		return Primary{}, fmt.Errorf("GET error: %v", err)
	}
	var primaryUrl string = fmt.Sprintf("%s/%s", url, primaryPath)

	return getDataFromPrimary(primaryUrl)
}

func getDataFromPrimary(url string) (Primary, error) {
	resp, err := http.Get(url)

	if err != nil {
		return Primary{}, fmt.Errorf("GET error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Primary{}, fmt.Errorf("status error: %v", resp.StatusCode)
	}

	reader, err := gzip.NewReader(resp.Body)

	if err != nil {
		fmt.Println(err)
	}

	var result Primary
	xml.NewDecoder(reader).Decode(&result)

	reader.Close()
	return result, nil
}

func getDataFromRepomd(url string) (string, error) {
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
	return primaryLocation, nil
}
