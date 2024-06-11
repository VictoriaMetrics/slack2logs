package importer

import (
	"context"
	"errors"
	"testing"

	"slack2logs/slack"
)

// Mock implementations of Importer and Exporter for testing purposes
type mockImporter struct {
	importFunc func(ctx context.Context, message LogMessage) error
}

func (m *mockImporter) Import(ctx context.Context, message LogMessage) error {
	return m.importFunc(ctx, message)
}

type mockExporter struct {
	exportFunc func(ctx context.Context, processMessage func(slack.Message))
}

func (m *mockExporter) Export(ctx context.Context, processMessage func(slack.Message)) {
	m.exportFunc(ctx, processMessage)
}

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

// Test for Processor.Run method
func TestProcessorRun(t *testing.T) {
	mockImp := &mockImporter{
		importFunc: func(ctx context.Context, message LogMessage) error {
			if message.Text == "test message" {
				return nil
			}
			return errors.New("import error")
		},
	}
	mockExp := &mockExporter{
		exportFunc: func(ctx context.Context, processMessage func(slack.Message)) {
			processMessage(slack.Message{Text: "test message"})
			processMessage(slack.Message{Text: "<@U0787V2AW9W> has joined the channel"}) // This should be filtered out
		},
	}
	processor := New(mockExp, mockImp)
	ctx := context.Background()
	processor.Run(ctx)
}

// Test for Importer and Exporter interaction
func TestProcessorImportError(t *testing.T) {
	mockImp := &mockImporter{
		importFunc: func(ctx context.Context, message LogMessage) error {
			return errors.New("import error")
		},
	}
	mockExp := &mockExporter{
		exportFunc: func(ctx context.Context, processMessage func(slack.Message)) {
			processMessage(slack.Message{Text: "test message"})
		},
	}
	processor := New(mockExp, mockImp)
	ctx := context.Background()
	processor.Run(ctx)
}
