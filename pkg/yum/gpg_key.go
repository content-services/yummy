package yum

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// FetchGPGKey GETs GPG Key from url with request timeout maximum timeout.
func FetchGPGKey(ctx context.Context, url string, client *http.Client) (*string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("error creating request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	code := resp.StatusCode
	gpgKeyString, err := responseBodyToString(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	if err == nil && code < 200 || code > 299 {
		return nil, code, fmt.Errorf("received http %d", code)
	}
	if _, openpgpErr := openpgp.ReadArmoredKeyRing(strings.NewReader(*gpgKeyString)); err != nil {
		return nil, code, openpgpErr //Bad key
	}
	return gpgKeyString, code, nil
}
