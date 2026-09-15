package diagnostics

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPropertyKeysOnlyExposeNamesAndTypes(t *testing.T) {
	keys := PropertyKeys(map[string]interface{}{"email": "private@example.com", "value": json.Number("99"), "paid": true})
	if len(keys) != 3 || keys[0] != "email:string" || keys[1] != "paid:boolean" || keys[2] != "value:number" {
		t.Fatalf("keys = %v", keys)
	}
}

func TestHealthDetectors(t *testing.T) {
	now := time.Now().UTC()
	pageview := func(timestamp time.Time) Entry {
		return Entry{Timestamp: timestamp, Status: "accepted", Mode: "live", Type: "pageview", Origin: "https://site.example", Path: "/pricing"}
	}
	for _, entries := range [][]Entry{
		{pageview(now), pageview(now.Add(2 * time.Second))},
		{pageview(now.Add(2 * time.Second)), pageview(now)},
	} {
		if !hasDuplicatePageviews(entries) {
			t.Fatal("duplicate page views were not detected")
		}
	}
	if hasDuplicatePageviews([]Entry{pageview(now), pageview(now.Add(10 * time.Second))}) {
		t.Fatal("distant page views were reported as duplicates")
	}
	if !hasMissingUTMAttribution([]Entry{{Path: "/pricing?utm_source=newsletter", Source: "direct"}}) {
		t.Fatal("missing UTM attribution was not detected")
	}
}

func TestDiagnosticSanitizers(t *testing.T) {
	if got := cleanOrigin("https://example.com/private?q=1"); got != "https://example.com" {
		t.Fatalf("origin = %q", got)
	}
	if got := cleanReferrer("https://Search.Example/results?q=secret"); got != "search.example" {
		t.Fatalf("referrer = %q", got)
	}
	if got := cleanPath("/checkout?email=private@example.com&utm_source=newsletter#token"); got != "/checkout?email=&utm_source=" {
		t.Fatalf("path = %q", got)
	}
}
