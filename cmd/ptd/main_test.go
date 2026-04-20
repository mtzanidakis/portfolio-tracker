package main

import (
	"testing"
	"time"
)

func TestParseLifetime(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"30d", 30 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"720h", 720 * time.Hour, false},
		{"15m", 15 * time.Minute, false},
		{"  7d  ", 7 * 24 * time.Hour, false},
		{"", 0, true},
		{"bad", 0, true},
		{"xd", 0, true},
	}
	for _, tc := range cases {
		got, err := parseLifetime(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%q: expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected err %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.in, got, tc.want)
		}
	}
}
