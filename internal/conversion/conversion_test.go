package conversion

import "testing"

func TestValidateFunnel(t *testing.T) {
	name, steps, err := ValidateFunnel("Signup", []string{"Viewed.Pricing", " SIGNUP "})
	if err != nil {
		t.Fatalf("ValidateFunnel() error = %v", err)
	}
	if name != "Signup" || steps[0] != "viewed.pricing" || steps[1] != "signup" {
		t.Fatalf("unexpected normalized funnel: %q %v", name, steps)
	}
	if _, _, err := ValidateFunnel("Short", []string{"signup"}); err == nil {
		t.Fatal("one-step funnel was accepted")
	}
	if _, _, err := ValidateFunnel("Bad", []string{"ok", "not valid"}); err == nil {
		t.Fatal("invalid event name was accepted")
	}
}

func TestValidateGoal(t *testing.T) {
	name, event, err := ValidateGoal("Paid order", " Purchase.Completed ")
	if err != nil {
		t.Fatalf("ValidateGoal() error = %v", err)
	}
	if name != "Paid order" || event != "purchase.completed" {
		t.Fatalf("unexpected normalized goal: %q %q", name, event)
	}
}
