package config

import "testing"

func TestSuperNovaWindowEnforcementConfig(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  bool
	}{
		{"default", "", true},
		{"explicit opt-out", "false", false},
		{"invalid value fails closed", "invalid", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SUPERNOVA_ENFORCE_OPEN", tc.value)
			if got := Load().SuperNovaEnforceOpen; got != tc.want {
				t.Fatalf("SUPERNOVA_ENFORCE_OPEN=%q: got %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}
