package slack

import "testing"

// Test for filterOutLogMessage function
func TestFilterOutLogMessage(t *testing.T) {
	tests := []struct {
		message string
		want    bool
	}{
		{"<@U0787V2AW9W> has joined the channel", true},
		{"User message content", false},
		{"Another message", false},
	}
	for _, tt := range tests {
		if got := filterOutLogMessage(tt.message); got != tt.want {
			t.Errorf("filterOutLogMessage(%q) = %v, want %v", tt.message, got, tt.want)
		}
	}
}
