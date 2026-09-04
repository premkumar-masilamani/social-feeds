package instagram

import (
	"testing"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantHandle string
		wantErr    bool
	}{
		{
			name:       "Plain handle",
			input:      "natgeo",
			wantHandle: "natgeo",
			wantErr:    false,
		},
		{
			name:       "Handle with @ prefix",
			input:      "@natgeo",
			wantHandle: "natgeo",
			wantErr:    false,
		},
		{
			name:       "Handle with dots and underscores",
			input:      "nasa.official_1",
			wantHandle: "nasa.official_1",
			wantErr:    false,
		},
		{
			name:    "Full HTTPS URL is rejected",
			input:   "https://www.instagram.com/natgeo/",
			wantErr: true,
		},
		{
			name:    "HTTP URL is rejected",
			input:   "http://instagram.com/natgeo",
			wantErr: true,
		},
		{
			name:    "URL with query parameters is rejected",
			input:   "https://www.instagram.com/natgeo/?igsh=MWF5eA==",
			wantErr: true,
		},
		{
			name:       "Whitespace padded line",
			input:      "   @cristiano   ",
			wantHandle: "cristiano",
			wantErr:    false,
		},
		{
			name:    "Empty line",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "Comment line",
			input:   "# this is a comment",
			wantErr: true,
		},
		{
			name:    "Reserved word / explore",
			input:   "explore",
			wantErr: true,
		},
		{
			name:    "Reserved word / reels",
			input:   "reels",
			wantErr: true,
		},
		{
			name:    "Invalid characters",
			input:   "user!name$",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := ParseTarget(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tt.input, err)
			}
			if profile.Handle != tt.wantHandle {
				t.Errorf("got handle %q, want %q", profile.Handle, tt.wantHandle)
			}
			if profile.Platform != "instagram" {
				t.Errorf("got platform %q, want %q", profile.Platform, "instagram")
			}
			wantURL := "https://www.instagram.com/" + tt.wantHandle + "/"
			if profile.URL != wantURL {
				t.Errorf("got URL %q, want %q", profile.URL, wantURL)
			}
		})
	}
}
