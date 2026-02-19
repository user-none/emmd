package emu

import "testing"

func TestApplyLadder(t *testing.T) {
	tests := []struct {
		name       string
		sample     int16
		panEnabled bool
		want       int16
	}{
		{"unmuted positive", 1000, true, 1128},
		{"unmuted zero", 0, true, 128},
		{"unmuted negative", -1000, true, -1096},
		{"muted positive", 5000, false, 128},
		{"muted zero", 0, false, 128},
		{"muted negative", -5000, false, -128},
		{"max positive", 8160, true, 8288},
		{"max negative", -8176, true, -8272},
		{"boundary -1", -1, true, -97},
		{"boundary +1", 1, true, 129},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyLadder(tt.sample, tt.panEnabled)
			if got != tt.want {
				t.Errorf("applyLadder(%d, %v) = %d, want %d",
					tt.sample, tt.panEnabled, got, tt.want)
			}
		})
	}
}
