package yum

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
)

// Package metadata of a given package
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

// Repomd metadata of the repomd of a repository
type Repomd struct {
	XMLName      xml.Name `xml:"repomd"`
	Data         []Data   `xml:"data"`
	Revision     string   `xml:"revision"`
	RepomdString *string  `xml:"-"`
}

type Data struct {
	Type     string   `xml:"type,attr"`
	Location Location `xml:"location"`
}
type Location struct {
	Href string `xml:"href,attr"`
}

type YummySettings struct {
	Client *http.Client
	URL    *string
}

type YumRepository interface {
	Configure(settings YummySettings)
	Packages() (packages []Package, statusCode int, err error)
	Repomd() (repomd *Repomd, statusCode int, err error)
	Signature() (repomdSignature *string, statusCode int, err error)
	Clear()
}

type Repository struct {
	settings        YummySettings
	packages        []Package // Packages repository contains
	repomdSignature *string   // Signature of the repository
	repomd          *Repomd   // Repomd of the repository
}

func NewRepository(settings YummySettings) (Repository, error) {
	if settings.Client == nil {
		settings.Client = http.DefaultClient
	}
	if settings.URL == nil {
		return Repository{}, fmt.Errorf("url cannot be nil")
	}
	return Repository{settings: settings}, nil
}

func (r *Repository) Configure(settings YummySettings) {
	if settings.Client != nil {
		r.settings.Client = settings.Client
	}
	if r.settings.Client == nil {
		r.settings.Client = http.DefaultClient
	}
	if settings.URL != nil {
		r.settings.URL = settings.URL
	}
	r.Clear()
}

// Clear resets cached data to nil
func (r *Repository) Clear() {
	r.repomd = nil
	r.packages = nil
	r.repomdSignature = nil
}

// Repomd populates r.Repomd with repository's repomd.xml metadata. Returns Repomd, response code, and error.
// If the repomd was successfully fetched previously, will return cached repomd.
func (r *Repository) Repomd() (*Repomd, int, error) {
	var result Repomd
	var err error
	var resp *http.Response
	var repomdURL string

	if r.repomd != nil {
		return r.repomd, 0, nil
	}
	if repomdURL, err = r.getRepomdURL(); err != nil {
		return nil, 0, fmt.Errorf("error parsing Repomd URL: %w", err)
	}
	if resp, err = r.settings.Client.Get(repomdURL); err != nil {
		return nil, erroredStatusCode(resp), fmt.Errorf("GET error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("Status error: %v", resp.StatusCode)
	}
	if result, err = ParseRepomdXML(resp.Body); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("error parsing repomd.xml: %w", err)
	}

	r.repomd = &result
	return r.repomd, resp.StatusCode, nil
}

func erroredStatusCode(response *http.Response) int {
	if response == nil {
		return 0
	} else {
		return response.StatusCode
	}
}

// Packages populates r.Packages with metadata of each package in repository. Returns response code and error.
// If the packages were successfully fetched previously, will return cached packages.
func (r *Repository) Packages() ([]Package, int, error) {
	var err error
	var primaryURL string
	var resp *http.Response
	var packages []Package

	if r.packages != nil {
		return r.packages, 0, nil
	}

	if _, _, err = r.Repomd(); err != nil {
		return nil, 0, fmt.Errorf("error parsing repomd.xml: %w", err)
	}

	if primaryURL, err = r.getPrimaryURL(); err != nil {
		return nil, 0, fmt.Errorf("error getting primary URL: %w", err)
	}

	if resp, err = r.settings.Client.Get(primaryURL); err != nil {
		return nil, erroredStatusCode(resp), fmt.Errorf("GET error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("status error: %d", resp.StatusCode)
	}

	if packages, err = ParseCompressedXMLData(resp.Body); err != nil {
		return nil, resp.StatusCode, err
	}
	r.packages = packages

	return packages, resp.StatusCode, nil
}

// Signature fetches the yum metadata signature and returns any error and HTTP code encountered.
// If the signature was successfully fetched previously, will return cached signature.
func (r *Repository) Signature() (*string, int, error) {
	var sig *string

	if r.repomdSignature != nil {
		return r.repomdSignature, 0, nil
	}

	sigUrl, err := r.getSignatureURL()
	if err != nil {
		return nil, 0, err
	}

	resp, err := r.settings.Client.Get(sigUrl)
	if err != nil {
		return nil, erroredStatusCode(resp), err
	} else if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, resp.StatusCode, fmt.Errorf("received http %d", resp.StatusCode)
	}

	if sig, err = responseBodyToString(resp.Body); err != nil {
		return nil, resp.StatusCode, err
	}
	resp.Body.Close()

	r.repomdSignature = sig
	return sig, resp.StatusCode, err
}

func (r *Repository) getRepomdURL() (string, error) {
	u, err := url.Parse(*r.settings.URL)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, "/repodata/repomd.xml")
	return u.String(), nil
}

func (r *Repository) getSignatureURL() (string, error) {
	url, err := r.getRepomdURL()
	if err == nil {
		return url + ".asc", nil
	} else {
		return "", err
	}
}

func (r *Repository) getPrimaryURL() (string, error) {
	var primaryLocation string

	if _, _, err := r.Repomd(); err != nil {
		return "", fmt.Errorf("error fetching Repomd: %w", err)
	}

	for _, data := range r.repomd.Data {
		if data.Type == "primary" {
			primaryLocation = data.Location.Href
		}
	}

	if primaryLocation == "" {
		return "", fmt.Errorf("GET error: Unable to parse 'primary' location in repomd.xml")
	}
	url, err := url.Parse(*r.settings.URL)
	if err != nil {
		return "", err
	}
	url.Path = path.Join(url.Path, primaryLocation)
	return url.String(), nil
}

func responseBodyToString(body io.ReadCloser) (*string, error) {
	byteValue, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	asString := string(byteValue)
	return &asString, nil
}

// ParseRepomdXML creates Repomd from repomd.xml body response
func ParseRepomdXML(body io.ReadCloser) (Repomd, error) {
	var result Repomd

	byteValue, err := io.ReadAll(body)
	if err != nil {
		return Repomd{}, fmt.Errorf("io.reader read failure: %w", err)
	}

	err = xml.Unmarshal(byteValue, &result)
	if err != nil {
		return Repomd{}, fmt.Errorf("xml.Unmarshal failure: %w", err)
	}
	repomdString := string(byteValue)
	result.RepomdString = &repomdString

	return result, err
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
					break
				}
				result = append(result, pkg)
			}
		}
	}

	return result, nil
}
