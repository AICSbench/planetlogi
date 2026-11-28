package config

import "testing"

func TestSuperNovaWindowConfigDefaultsToEnforcement(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		want        bool
	}{
		{"default", "", true},
		{"invalid override", "unexpected", true},
		{"demo override", "false", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SUPERNOVA_ENFORCE_OPEN", tc.value)
			if got := Load().SuperNovaEnforceOpen; got != tc.want {
				t.Fatalf("SuperNovaEnforceOpen = %t; want %t", got, tc.want)
			}
		})
	}
}
