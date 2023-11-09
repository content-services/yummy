package yum

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/klauspost/compress/zstd"
	"github.com/openlyinc/pointy"
	"github.com/ulikunitz/xz"
)

// Max uncompressed XML file supported
const DefaultMaxXmlSize = int64(512 * 1024 * 1024) // 512 MB

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
	Client     *http.Client
	URL        *string
	MaxXmlSize *int64
}

type PackageGroup struct {
	ID          string                  `xml:"id"`
	Name        PackageGroupName        `xml:"name"`
	Description PackageGroupDescription `xml:"description"`
	PackageList []string                `xml:"packagelist>packagereq"`
}

type PackageGroupName string

type PackageGroupDescription string

type Environment struct {
	ID          string                 `xml:"id"`
	Name        EnvironmentName        `xml:"name"`
	Description EnvironmentDescription `xml:"description"`
}

type EnvironmentName string

type EnvironmentDescription string

type Comps struct {
	PackageGroups []PackageGroup
	Environments  []Environment
}

type YumRepository interface {
	Configure(settings YummySettings)
	Packages() (packages []Package, statusCode int, err error)
	Repomd() (repomd *Repomd, statusCode int, err error)
	Signature() (repomdSignature *string, statusCode int, err error)
	Comps() (comps *Comps, statusCode int, err error)
	PackageGroups() (packageGroups []PackageGroup, statusCode int, err error)
	Environments() (environments []Environment, statusCode int, err error)
	Clear()
}

type Repository struct {
	settings        YummySettings
	packages        []Package // Packages repository contains
	repomdSignature *string   // Signature of the repository
	repomd          *Repomd   // Repomd of the repository
	comps           *Comps    // Comps of the repository
}

func NewRepository(settings YummySettings) (Repository, error) {
	if settings.Client == nil {
		settings.Client = http.DefaultClient
	}
	if settings.URL == nil {
		return Repository{}, fmt.Errorf("url cannot be nil")
	}
	if settings.MaxXmlSize == nil {
		settings.MaxXmlSize = pointy.Pointer(DefaultMaxXmlSize)
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
	r.comps = nil
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
		return nil, 0, fmt.Errorf("Error parsing Repomd URL: %w", err)
	}
	if resp, err = r.settings.Client.Get(repomdURL); err != nil {
		return nil, erroredStatusCode(resp), fmt.Errorf("GET error for file %v: %w", repomdURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("Cannot fetch %v: %v", repomdURL, resp.StatusCode)
	}
	if result, err = ParseRepomdXML(resp.Body); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("Error parsing repomd.xml: %w", err)
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

func (r *Repository) Comps() (*Comps, int, error) {
	var err error
	var compsURL *string
	var resp *http.Response
	var comps Comps

	if r.comps != nil {
		return r.comps, 200, nil
	}

	if _, _, err = r.Repomd(); err != nil {
		return nil, 0, fmt.Errorf("error parsing repomd.xml: %w", err)
	}

	if compsURL, err = r.getCompsURL(); err != nil {
		return nil, 0, fmt.Errorf("error parsing Comps URL: %w", err)
	}

	if compsURL != nil {
		if resp, err = r.settings.Client.Get(*compsURL); err != nil {
			return nil, erroredStatusCode(resp), fmt.Errorf("GET error for file %v: %w", compsURL, err)
		}

		defer resp.Body.Close()

		if comps, err = ParseCompsXML(resp.Body); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("error parsing comps.xml: %w", err)
		}

		r.comps = &comps

		return r.comps, resp.StatusCode, nil
	}

	return nil, 200, nil
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
		return nil, 0, fmt.Errorf("Error getting primary URL: %w", err)
	}

	if resp, err = r.settings.Client.Get(primaryURL); err != nil {
		return nil, erroredStatusCode(resp), fmt.Errorf("GET error for file %v: %w", primaryURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("Cannot fetch %v: %d", primaryURL, resp.StatusCode)
	}

	if packages, err = ParseCompressedXMLData(io.NopCloser(resp.Body), *r.settings.MaxXmlSize); err != nil {
		return nil, resp.StatusCode, err
	}
	r.packages = packages

	return packages, resp.StatusCode, nil
}

// PackageGroups populates r.PackageGroups with the package groups of a repository. Returns response code and error.
func (r *Repository) PackageGroups() ([]PackageGroup, int, error) {
	var err error
	var status int
	var comps *Comps

	if r.comps != nil && r.comps.PackageGroups != nil {
		return r.comps.PackageGroups, 200, nil
	}

	if comps, status, err = r.Comps(); err != nil {
		return nil, 0, fmt.Errorf("error getting comps: %w", err)
	}

	if compsURL, _ := r.getCompsURL(); compsURL != nil {
		r.comps.PackageGroups = comps.PackageGroups
		return r.comps.PackageGroups, status, nil
	}

	return nil, status, nil

}

// Environments populates r.Environments with the environments of a repository. Returns response code and error.
func (r *Repository) Environments() ([]Environment, int, error) {
	var err error
	var status int
	var comps *Comps

	if r.comps != nil && r.comps.Environments != nil {
		return r.comps.Environments, 200, nil
	}

	if comps, status, err = r.Comps(); err != nil {
		return nil, 0, fmt.Errorf("error getting comps: %w", err)
	}

	if compsURL, _ := r.getCompsURL(); compsURL != nil {
		r.comps.Environments = comps.Environments
		return r.comps.Environments, status, nil
	}

	return nil, status, nil
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

func (r *Repository) getCompsURL() (*string, error) {
	var compsLocation string

	for _, data := range r.repomd.Data {
		if data.Type == "group" {
			compsLocation = data.Location.Href
		}
	}

	if compsLocation == "" {
		return nil, nil
	}

	url, err := url.Parse(*r.settings.URL)
	if err != nil {
		return nil, err
	}
	url.Path = path.Join(url.Path, compsLocation)
	return pointy.Pointer(url.String()), nil
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

// ParseCompsXML creates PackageGroup array and Environment array from comps.xml body response
func ParseCompsXML(body io.ReadCloser) (Comps, error) {
	var comps Comps
	packageGroups := []PackageGroup{}
	environments := []Environment{}

	byteValue, err := io.ReadAll(body)

	if err != nil {
		return comps, fmt.Errorf("io.reader read failure: %w", err)
	}

	decoder := xml.NewDecoder(bytes.NewReader(byteValue))

	for {
		t, decodeError := decoder.Token()

		if decodeError == io.EOF {
			break
		} else if decodeError != nil {
			return comps, fmt.Errorf("error decoding token: %w", decodeError)
		} else if t == nil {
			break
		}

		switch elType := t.(type) {
		case xml.StartElement:
			if elType.Name.Local == "group" {
				var packageGroup PackageGroup
				if decodeElementError := decoder.DecodeElement(&packageGroup, &elType); decodeElementError != nil {
					return comps, decodeElementError
				}
				packageGroups = append(packageGroups, packageGroup)
			} else if elType.Name.Local == "environment" {
				var environment Environment
				if decodeElementError := decoder.DecodeElement(&environment, &elType); decodeElementError != nil {
					return comps, decodeElementError
				}
				environments = append(environments, environment)
			}
		}
	}

	return Comps{packageGroups, environments}, err
}

// Custom unmarshal methods for localized elements
func (pn *PackageGroupName) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var t string
	if err := d.DecodeElement(&t, &start); err != nil {
		return err
	}
	if len(start.Attr) == 0 {
		*pn = PackageGroupName(t)
	}
	return nil
}

func (pd *PackageGroupDescription) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var t string
	if err := d.DecodeElement(&t, &start); err != nil {
		return err
	}
	if len(start.Attr) == 0 {
		*pd = PackageGroupDescription(t)
	}
	return nil
}

func (en *EnvironmentName) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var t string
	if err := d.DecodeElement(&t, &start); err != nil {
		return err
	}
	if len(start.Attr) == 0 {
		*en = EnvironmentName(t)
	}
	return nil
}

func (ed *EnvironmentDescription) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var t string
	if err := d.DecodeElement(&t, &start); err != nil {
		return err
	}
	if len(start.Attr) == 0 {
		*ed = EnvironmentDescription(t)
	}
	return nil
}

// Unzips a compressed body response, then parses the contained XML for package information
// This uses a BufferedReader to peek at the data to figure out what type of compression to use.
// This also gets wrapped in a LimitedReader to prevent large files from causing an OOM
//
// Returns an array of package data
func ParseCompressedXMLData(body io.Reader, maxSize int64) ([]Package, error) {
	var reader io.Reader
	var err error
	result := []Package{}

	bufferedReader := bufio.NewReader(body)

	// peek at the first bytes to determine the type
	header, err := bufferedReader.Peek(20)
	if err != nil {
		return result, err
	}

	if err != nil {
		return []Package{}, err
	}
	fileType, err := filetype.Match(header)
	if err != nil {
		return []Package{}, err
	}

	switch fileType {
	case matchers.TypeGz:
		reader, err = gzip.NewReader(bufferedReader)
	case matchers.TypeZstd:
		reader, err = zstd.NewReader(bufferedReader)
	case matchers.TypeXz:
		reader, err = xz.NewReader(bufferedReader)
	default:
		return []Package{}, fmt.Errorf("invalid file type: must be gzip, xz, or zstd.")
	}
	if err != nil {
		return []Package{}, fmt.Errorf("Error unzipping response body: %w", err)
	}

	limitedReader := io.LimitReader(reader, maxSize)
	decoder := xml.NewDecoder(limitedReader)

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
					return result, decodeElementError
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
