package main

import "testing"

func TestShouldBootstrapServicesOnlyForBrandNewEmptyState(t *testing.T) {
	tests := []struct {
		name           string
		stateDBExisted bool
		serviceCount   int
		want           bool
	}{
		{name: "brand-new empty state", stateDBExisted: false, serviceCount: 0, want: true},
		{name: "existing empty state remains intentionally empty", stateDBExisted: true, serviceCount: 0, want: false},
		{name: "existing populated state", stateDBExisted: true, serviceCount: 3, want: false},
		{name: "new state already populated", stateDBExisted: false, serviceCount: 1, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldBootstrapServices(tc.stateDBExisted, tc.serviceCount); got != tc.want {
				t.Fatalf("shouldBootstrapServices(%v, %d)=%v want %v", tc.stateDBExisted, tc.serviceCount, got, tc.want)
			}
		})
	}
}
