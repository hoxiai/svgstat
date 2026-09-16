package worker

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hoxiai/svgstat/internal/analytics"
)

func TestFlushDates(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want []string
	}{
		{
			name: "regular day",
			now:  time.Date(2026, 8, 22, 15, 4, 5, 0, time.UTC),
			want: []string{"2026-08-22", "2026-08-21"},
		},
		{
			name: "month boundary",
			now:  time.Date(2026, 3, 1, 0, 30, 0, 0, time.UTC),
			want: []string{"2026-03-01", "2026-02-28"},
		},
		{
			name: "year boundary",
			now:  time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			want: []string{"2026-01-01", "2025-12-31"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flushDates(tt.now)
			if len(got) != len(tt.want) {
				t.Fatalf("flushDates() returned %d dates, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("flushDates()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDailyStatsArgs(t *testing.T) {
	stats := &analytics.DailyStats{
		ProjectID: "proj-1",
		Date:      "2026-08-22",
		PV:        10,
		UV:        4,
		Requests:  12,
		Bots:      2,
		Referrers: map[string]int64{"github.com": 7},
		Countries: map[string]int64{"CN": 5, "US": 3},
		Regions:   map[string]int64{"Zhejiang": 2},
		Cities:    map[string]int64{"Hangzhou": 2},
		Devices:   map[string]int64{"desktop": 8},
		Browsers:  map[string]int64{"chrome": 9},
		Paths:     map[string]int64{"/svg/demo/counter/visits.svg": 10},
		IPs:       map[string]int64{"1.2.3.4": 3},
		Sources:   map[string]int64{"newsletter": 4},
		Mediums:   map[string]int64{"email": 4},
		Campaigns: map[string]int64{"launch": 4},
	}

	args, err := dailyStatsArgs(stats)
	if err != nil {
		t.Fatalf("dailyStatsArgs() error = %v", err)
	}

	if len(args) != 36 {
		t.Fatalf("dailyStatsArgs() returned %d args, want 36", len(args))
	}

	id, ok := args[0].(string)
	if !ok || len(id) != 32 {
		t.Errorf("args[0] (id) = %v, want 32-char hex string", args[0])
	}
	if args[1] != "proj-1" {
		t.Errorf("args[1] (project_id) = %v, want proj-1", args[1])
	}
	if args[2] != "2026-08-22" {
		t.Errorf("args[2] (date) = %v, want 2026-08-22", args[2])
	}
	if args[3] != int64(10) || args[4] != int64(4) || args[5] != int64(12) || args[6] != int64(2) {
		t.Errorf("numeric args = %v %v %v %v, want 10 4 12 2", args[3], args[4], args[5], args[6])
	}
	if args[28] != int64(0) || args[29] != int64(0) || args[30] != int64(0) || args[31] != int64(0) {
		t.Errorf("session numeric args = %v, want zero defaults", args[28:32])
	}

	var countries map[string]int64
	countriesRaw, ok := args[8].([]byte)
	if !ok {
		t.Fatalf("args[8] (countries) is %T, want []byte", args[8])
	}
	if err := json.Unmarshal(countriesRaw, &countries); err != nil {
		t.Fatalf("failed to unmarshal countries: %v", err)
	}
	if countries["CN"] != 5 || countries["US"] != 3 {
		t.Errorf("countries = %v, want map[CN:5 US:3]", countries)
	}
}

func TestDailyStatsArgsEmptyMaps(t *testing.T) {
	stats := &analytics.DailyStats{
		ProjectID: "proj-1",
		Date:      "2026-08-22",
		Referrers: map[string]int64{},
		Countries: map[string]int64{},
		Regions:   map[string]int64{},
		Cities:    map[string]int64{},
		Devices:   map[string]int64{},
		Browsers:  map[string]int64{},
		Paths:     map[string]int64{},
		IPs:       map[string]int64{},
		Sources:   map[string]int64{},
		Mediums:   map[string]int64{},
		Campaigns: map[string]int64{},
	}

	args, err := dailyStatsArgs(stats)
	if err != nil {
		t.Fatalf("dailyStatsArgs() error = %v", err)
	}

	// JSONB columns must receive {} rather than null so queries can rely on it.
	jsonIndexes := []int{7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 32, 33, 34, 35}
	for _, i := range jsonIndexes {
		raw, ok := args[i].([]byte)
		if !ok {
			t.Fatalf("args[%d] is %T, want []byte", i, args[i])
		}
		if string(raw) != "{}" {
			t.Errorf("args[%d] = %s, want {}", i, raw)
		}
	}
}

func TestHasTrafficIncludesWebsitePageviews(t *testing.T) {
	if !hasTraffic(&analytics.DailyStats{PV: 1}) {
		t.Fatal("website-only page views were treated as empty traffic")
	}
	if hasTraffic(&analytics.DailyStats{}) {
		t.Fatal("empty statistics were treated as traffic")
	}
	if !hasTraffic(&analytics.DailyStats{Events: map[string]int64{"signup": 1}}) {
		t.Fatal("event-only statistics were treated as empty traffic")
	}
}
