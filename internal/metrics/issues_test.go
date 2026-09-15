package metrics

import (
	"testing"
	"time"
)

func TestBuildIssueReportDetectsActionableProblems(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	current, previous := newIssuePeriod(), newIssuePeriod()
	current.PV, current.UV = 50, 100
	previous.PV, previous.UV = 200, 100
	current.EventVisitors["purchase"] = 2
	previous.EventVisitors["purchase"] = 10
	current.Events["web_vital_lcp"] = 20
	current.EventValues["web_vital_lcp"] = map[string]float64{"XXX": 100000}
	current.EventVisitors["js_error"] = 25
	previous.EventVisitors["js_error"] = 5
	lastActive := utcDay(now)
	report := buildIssueReport("project-1", 7, now, current, previous, &lastActive, []IssueGoal{{Name: "Purchase", EventName: "purchase"}})
	if report.Status != "critical" || len(report.Issues) != 4 {
		t.Fatalf("report = %#v", report)
	}
	want := map[string]bool{"traffic_drop": true, "conversion_drop": true, "vital_poor": true, "javascript_errors_spike": true}
	for _, issue := range report.Issues {
		delete(want, issue.Code)
	}
	if len(want) != 0 {
		t.Fatalf("missing issues = %#v", want)
	}
}

func TestBuildIssueReportAvoidsLowVolumeNoise(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	current, previous := newIssuePeriod(), newIssuePeriod()
	current.PV, current.UV = 2, 2
	previous.PV, previous.UV = 10, 10
	current.EventVisitors["purchase"] = 0
	previous.EventVisitors["purchase"] = 2
	lastActive := utcDay(now)
	report := buildIssueReport("project-1", 7, now, current, previous, &lastActive, []IssueGoal{{Name: "Purchase", EventName: "purchase"}})
	if report.Status != "insufficient_data" || len(report.Issues) != 0 {
		t.Fatalf("report = %#v", report)
	}
}

func TestBuildIssueReportDetectsCollectionStopped(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	current, previous := newIssuePeriod(), newIssuePeriod()
	previous.PV, previous.UV = 80, 50
	lastActive := utcDay(now).AddDate(0, 0, -3)
	report := buildIssueReport("project-1", 7, now, current, previous, &lastActive, nil)
	if report.Status != "critical" || len(report.Issues) != 1 || report.Issues[0].Code != "collection_stopped" {
		t.Fatalf("report = %#v", report)
	}
}
