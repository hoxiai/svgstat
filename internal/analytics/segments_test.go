package analytics

import (
	"testing"
)

func TestAnalysisSegments(t *testing.T) {
	segments := analysisSegments(&RequestData{Source: "newsletter", Medium: "email", Campaign: "launch", Path: "/pricing", DeviceType: "mobile", Country: "CN"})
	if len(segments) != 6 || segments["source"] != "newsletter" || segments["path"] != "/pricing" || segments["country"] != "CN" {
		t.Fatalf("segments = %#v", segments)
	}
	segments = analysisSegments(&RequestData{Source: "direct", Medium: "none"})
	if len(segments) != 2 {
		t.Fatalf("empty dimensions were retained: %#v", segments)
	}
}

func TestParseFunnelSessionState(t *testing.T) {
	values := []interface{}{"2", `{"source":"newsletter","path":"/pricing"}`}
	if state := parseStateValue(values, 0); state != 2 {
		t.Fatalf("state = %d", state)
	}
	segments := parseSegmentState(values)
	if segments["source"] != "newsletter" || segments["path"] != "/pricing" {
		t.Fatalf("segments = %#v", segments)
	}
	if parseStateValue(nil, 0) != 0 || parseSegmentState([]interface{}{"bad", "{"}) != nil {
		t.Fatal("invalid session state was accepted")
	}
}

func TestSegmentKeyPartIsStableAndBounded(t *testing.T) {
	first := segmentKeyPart("/pricing:enterprise")
	if len(first) != 16 || first != segmentKeyPart("/pricing:enterprise") || first == segmentKeyPart("/pricing") {
		t.Fatalf("unexpected segment key: %q", first)
	}
}
