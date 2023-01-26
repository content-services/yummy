package yum

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/openlyinc/pointy"
	"github.com/stretchr/testify/assert"
)

//go:embed "mocks/repomd.xml"
var repomdXML []byte

//go:embed "mocks/primary.xml.gz"
var primaryXML []byte

//go:embed "mocks/repomd.xml.asc"
var signatureXML []byte

func TestConfigure(t *testing.T) {
	firstURL := "http://first.example.com"
	firstClient := &http.Client{}
	settings := YummySettings{
		Client: firstClient,
		URL:    &firstURL,
	}
	r, _ := NewRepository(settings)

	assert.EqualValues(t, firstURL, *r.settings.URL)
	assert.Equal(t, firstClient, r.settings.Client)

	secondURL := "http://second.example.com"
	secondClient := &http.Client{Timeout: time.Second}
	secondSettings := YummySettings{
		Client: secondClient,
		URL:    &secondURL,
	}
	r.Configure(secondSettings)
	assert.Equal(t, secondURL, *r.settings.URL)
	assert.Equal(t, secondClient.Timeout, r.settings.Client.Timeout)
	assert.NotEqual(t, firstClient.Timeout, r.settings.Client.Timeout)
}

func TestClear(t *testing.T) {
	s := server()
	defer s.Close()

	c := s.Client()
	settings := YummySettings{
		Client: c,
		URL:    &s.URL,
	}
	r, _ := NewRepository(settings)

	_, _, _ = r.Repomd()
	_, _, _ = r.Packages()
	_, _, _ = r.Signature()
	assert.NotNil(t, r.repomd)
	assert.NotNil(t, r.packages)
	assert.NotNil(t, r.repomdSignature)

	r.Clear()
	assert.Nil(t, r.repomd)
	assert.Nil(t, r.packages)
	assert.Nil(t, r.repomdSignature)

}
func TestGetPrimaryURL(t *testing.T) {
	xmlFile, err := os.Open("mocks/repomd.xml")
	assert.Nil(t, err)
	settings := YummySettings{
		URL: pointy.String("http://foo.example.com/repo/"),
	}
	r, err := NewRepository(settings)
	assert.Nil(t, err)
	repomd, err := ParseRepomdXML(xmlFile)
	assert.Nil(t, err)
	r.repomd = &repomd

	primary, err := r.getPrimaryURL()
	assert.Nil(t, err)
	assert.Equal(t, "http://foo.example.com/repo/repodata/primary.xml.gz", primary)
}

func TestFetchRepomd(t *testing.T) {
	s := server()
	defer s.Close()

	c := s.Client()
	settings := YummySettings{
		Client: c,
		URL:    &s.URL,
	}
	r, _ := NewRepository(settings)

	repomdStringMock := string(repomdXML)
	expected := Repomd{
		XMLName: xml.Name{
			Space: "http://linux.duke.edu/metadata/repo",
			Local: "repomd",
		},
		Data: []Data{
			{
				Type:     "other",
				Location: Location{Href: "repodata/other.xml.gz"},
			},
			{
				Type:     "filelists",
				Location: Location{Href: "repodata/filelists.xml.gz"},
			},
			{
				Type:     "primary",
				Location: Location{Href: "repodata/primary.xml.gz"},
			},
			{
				Type:     "updateinfo",
				Location: Location{Href: "repodata/updateinfo.xml.gz"},
			},
		},
		Revision:     "1308257578",
		RepomdString: &repomdStringMock,
	}

	repomd, code, err := r.Repomd()
	assert.Equal(t, expected, *repomd)
	assert.Equal(t, *repomd, *r.repomd)
	assert.Equal(t, 200, code)
	assert.Nil(t, err)
}

func TestFetchPackages(t *testing.T) {
	s := server()
	defer s.Close()

	c := s.Client()
	settings := YummySettings{
		Client: c,
		URL:    &s.URL,
	}
	r, _ := NewRepository(settings)

	packages, code, err := r.Packages()
	assert.Equal(t, 2, len(packages))
	assert.Equal(t, packages, r.packages)
	assert.Equal(t, 200, code)
	assert.Nil(t, err)
}

func TestBadUrl(t *testing.T) {
	badUrl := "example.com/"
	s := server()
	defer s.Close()

	c := s.Client()
	settings := YummySettings{
		Client: c,
		URL:    &badUrl,
	}
	r, _ := NewRepository(settings)
	_, code, err := r.Repomd()
	assert.Error(t, err)
	assert.Equal(t, code, 0)
}

func TestFetchRepomdSignature(t *testing.T) {
	s := server()
	defer s.Close()

	c := s.Client()
	settings := YummySettings{
		Client: c,
		URL:    &s.URL,
	}
	r, _ := NewRepository(settings)

	signature, code, err := r.Signature()
	assert.NotEmpty(t, signature)
	assert.Equal(t, signature, r.repomdSignature)
	assert.Equal(t, 200, code)
	assert.Nil(t, err)
}

// Check that the parser can decompress a gzip file and read the correct number of packages
func TestParseCompressedXMLData(t *testing.T) {
	xmlFile, err := os.Open("mocks/primary.xml.gz")
	if err != nil {
		log.Fatal(err)
	}
	defer xmlFile.Close()
	result, err := ParseCompressedXMLData(xmlFile)
	if err != nil {
		t.Errorf("Error in test: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Error - Expected to return 2 packages but received: %v", len(result))
	}
	if result[0].Checksum.Type != "sha1" {
		t.Errorf(fmt.Sprintf("Checksum of %s received, should be sha1", result[0].Checksum.Type))
	}
	if result[0].Summary == "" {
		t.Errorf("Did not properly parse summary")
	}
	if result[0].Checksum.Value == "" {
		t.Errorf("Did not properly parse checksum")
	}
}

func server() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/repodata/repomd.xml", serveRepomdXML)
	mux.HandleFunc("/repodata/primary.xml.gz", servePrimaryXML)
	mux.HandleFunc("/repodata/repomd.xml.asc", serveSignatureXML)
	mux.HandleFunc("/gpgkey.pub", serveGPGKey)
	return httptest.NewServer(mux)
}

func serveRepomdXML(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/xml")
	body := repomdXML
	_, _ = w.Write(body)
}

func servePrimaryXML(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/gzip")
	body := primaryXML
	_, _ = w.Write(body)
}

func serveSignatureXML(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/xml")
	body := signatureXML
	_, _ = w.Write(body)
}
