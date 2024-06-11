package importer

import (
	"context"
	"log"

	"slack2logs/slack"
)

// Importer defines importer interface
// which should be implemented for each importer
type Importer interface {
	Import(ctx context.Context, message LogMessage) error
}

// Exporter defines exporter interface
// which should be implemented for each exporter
type Exporter interface {
	Export(context.Context, func(slack.Message))
}

// Processor defines object with exporter and importer
type Processor struct {
	exporter Exporter
	importer Importer
}

// LogMessage represents data for storing in the logs
type LogMessage struct {
	ThreadID              string `json:"thread_id"`
	Type                  string `json:"type"`
	User                  string `json:"user"`
	Text                  string `json:"text"`
	ThreadTimeStamp       string `json:"thread_ts"`
	TimeStamp             string `json:"ts"`
	ChannelID             string `json:"channel_id"`
	ChannelName           string `json:"channel_name"`
	UserID                string `json:"user_id"`
	DisplayName           string `json:"display_name"`
	DisplayNameNormalized string `json:"display_name_normalized"`
}

func New(exporter Exporter, importer Importer) *Processor {
	p := Processor{exporter: exporter, importer: importer}
	return &p
}

// Run starts export import process
func (p *Processor) Run(ctx context.Context) {
	p.exporter.Export(ctx, func(m slack.Message) {
		logMsg := LogMessage(m)
		if err := p.importer.Import(ctx, logMsg); err != nil {
			log.Printf("error import message to the importer: %s", err)
		}
	})
}
