package yum

import (
	_ "embed"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed "mocks/gpgkey.pub"
var gpgKey []byte

func TestFetchGPGKey(t *testing.T) {
	s := server()
	defer s.Close()

	c := s.Client()

	gpg, code, err := FetchGPGKey(s.URL+"/gpgkey.pub", c)
	assert.NotEmpty(t, gpg)
	assert.Equal(t, 200, code)
	assert.Nil(t, err)
}

func serveGPGKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/octet-stream")
	body := gpgKey
	_, _ = w.Write(body)
}
