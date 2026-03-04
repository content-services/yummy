package yum

import (
	_ "embed"
	"net/http"
)

//go:embed "mocks/gpgkey.pub"
var gpgKey []byte

func serveGPGKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/octet-stream")
	body := gpgKey
	_, _ = w.Write(body)
}
