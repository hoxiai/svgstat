package metrics

import (
	"testing"
	"time"
)

func TestInstallationStatus_DateEndCalculation(t *testing.T) {
	d := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	endOfDay := d.Add(24*time.Hour - time.Second)
	if !endOfDay.After(d) {
		t.Fatalf("expected endOfDay (%v) to be after d (%v)", endOfDay, d)
	}
	if endOfDay.Format("2006-01-02") != "2026-09-20" {
		t.Fatalf("expected date format to remain 2026-09-20, got %s", endOfDay.Format("2006-01-02"))
	}
}
