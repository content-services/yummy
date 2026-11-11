package yum

import (
	"context"
	_ "embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed "mocks/gpgkey.pub"
var gpgKey []byte

func TestFetchGPGKey(t *testing.T) {
	s := server()
	defer s.Close()

	c := s.Client()

	gpg, code, err := FetchGPGKey(context.Background(), s.URL+"/gpgkey.pub", c)
	assert.NotEmpty(t, gpg)
	assert.Equal(t, 200, code)
	assert.Nil(t, err)
}

func TestFetchGPGKeyInvalid(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not a gpg key</html>"))
	}))
	defer s.Close()

	gpg, code, err := FetchGPGKey(context.Background(), s.URL, s.Client())
	assert.Nil(t, gpg)
	assert.Equal(t, 200, code)
	assert.Error(t, err)
}

func TestFetchGPGKeyHTTPError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	gpg, code, err := FetchGPGKey(context.Background(), s.URL, s.Client())
	assert.Nil(t, gpg)
	assert.Equal(t, 404, code)
	assert.Error(t, err)
}

func serveGPGKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/octet-stream")
	body := gpgKey
	_, _ = w.Write(body)
}
