package model

import "testing"

func TestParseAvatarMediaID(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    int64
		wantErr bool
	}{
		{name: "empty is no-avatar sentinel", in: "", want: 0},
		{name: "blank trims to sentinel", in: "  ", want: 0},
		{name: "decimal snowflake", in: "1930000000000000001", want: 1930000000000000001},
		{name: "surrounding space trimmed", in: " 42 ", want: 42},
		{name: "non-decimal rejected", in: "med_x", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseAvatarMediaID(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseAvatarMediaID(%q) = %d, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAvatarMediaID(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("ParseAvatarMediaID(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatAvatarMediaID(t *testing.T) {
	if got := FormatAvatarMediaID(0); got != "" {
		t.Fatalf("FormatAvatarMediaID(0) = %q, want empty (no avatar)", got)
	}
	if got := FormatAvatarMediaID(1930000000000000001); got != "1930000000000000001" {
		t.Fatalf("FormatAvatarMediaID = %q", got)
	}
}

func TestAvatarMediaIDRoundTrip(t *testing.T) {
	for _, wire := range []string{"", "1930000000000000001", "42"} {
		id, err := ParseAvatarMediaID(wire)
		if err != nil {
			t.Fatalf("ParseAvatarMediaID(%q): %v", wire, err)
		}
		if got := FormatAvatarMediaID(id); got != wire {
			t.Fatalf("round-trip %q → %d → %q", wire, id, got)
		}
	}
}
