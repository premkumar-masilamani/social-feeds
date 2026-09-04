package instagram

import (
	"testing"
	"time"
)

func TestParseCaptionDate(t *testing.T) {
	tests := []struct {
		caption  string
		wantYear int
		wantMon  time.Month
		wantDay  int
	}{
		{
			caption:  "Photo by Samyuktha Ranilakshmi on April 11, 2026. May be an image of...",
			wantYear: 2026,
			wantMon:  time.April,
			wantDay:  11,
		},
		{
			caption:  "Video by Samyuktha Ranilakshmi on August 15, 2026.",
			wantYear: 2026,
			wantMon:  time.August,
			wantDay:  15,
		},
		{
			caption:  "Video on January 02, 2025 with friends.",
			wantYear: 2025,
			wantMon:  time.January,
			wantDay:  2,
		},
		{
			caption:  "No date mentioned here",
			wantYear: 0,
		},
	}

	for _, tt := range tests {
		got := parseCaptionDate(tt.caption)
		if tt.wantYear == 0 {
			if !got.IsZero() {
				t.Errorf("expected zero time, got %v", got)
			}
		} else {
			if got.Year() != tt.wantYear || got.Month() != tt.wantMon || got.Day() != tt.wantDay {
				t.Errorf("got %v, want %04d-%02d-%02d", got, tt.wantYear, tt.wantMon, tt.wantDay)
			}
		}
	}
}

func TestShortcodeExtraction(t *testing.T) {
	tests := []struct {
		href     string
		expected string
	}{
		{href: "/p/C_abc123/", expected: "C_abc123"},
		{href: "/reel/DcFuLeATEc9/", expected: "DcFuLeATEc9"},
		{href: "/myu_stories/p/DW_6haMiEsX/", expected: "DW_6haMiEsX"},
		{href: "/myu_stories/reel/DU-zjM6CIlT/", expected: "DU-zjM6CIlT"},
		{href: "/explore/", expected: ""},
	}

	for _, tt := range tests {
		m := shortcodeRegex.FindStringSubmatch(tt.href)
		got := ""
		if len(m) > 1 {
			got = m[1]
		}
		if got != tt.expected {
			t.Errorf("for %q, got %q, want %q", tt.href, got, tt.expected)
		}
	}
}
