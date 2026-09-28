package analytics

import (
	"os"
	"strings"
	"testing"
)

// The SDK copies attribution parameters from the landing URL onto later
// pageviews; any parameter the server attributes on must be carried.
func TestSDKCarriesAttributionParams(t *testing.T) {
	source, err := os.ReadFile("../../web/static/js/sdk.js")
	if err != nil {
		t.Fatalf("read sdk.js: %v", err)
	}
	sdk := string(source)
	params := []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"}
	for _, id := range clickIDs {
		params = append(params, id.param)
	}
	for _, param := range params {
		if !strings.Contains(sdk, "'"+param+"'") {
			t.Errorf("sdk.js does not carry %q across pageviews", param)
		}
	}
	if !strings.Contains(sdk, "body.set('search'") {
		t.Error("sdk.js does not send the site search term")
	}
}
