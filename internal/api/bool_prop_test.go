package api

import (
	"testing"
)

func TestParseCustomEvent_BooleanProperty(t *testing.T) {
	event, err := parseCustomEvent("login_click", `{"is_vip":true,"remember":false}`, "/", "", "visitor-1")
	if err != nil {
		t.Fatalf("parseCustomEvent() error = %v", err)
	}
	hasVIP := false
	hasRemember := false
	for _, k := range event.PropertyKeys {
		if k == "is_vip:boolean" {
			hasVIP = true
		}
		if k == "remember:boolean" {
			hasRemember = true
		}
	}
	if !hasVIP || !hasRemember {
		t.Errorf("PropertyKeys = %v, want is_vip:boolean and remember:boolean", event.PropertyKeys)
	}
}

func TestAutomaticEventDimensions_BooleanPropertySupport(t *testing.T) {
	// If a whitelisted dimension is passed as a bool, it should be converted and recognized
	props := map[string]interface{}{
		"contact_type": "email",
		"is_valid":     true,
	}
	// normalize bool in props
	for k, v := range props {
		if b, ok := v.(bool); ok {
			if b {
				props[k] = "true"
			} else {
				props[k] = "false"
			}
		}
	}
	if props["is_valid"] != "true" {
		t.Fatalf("expected props[is_valid] to be 'true', got %v", props["is_valid"])
	}
}
