package transporter

import (
	"context"
	"log"
)

// Message represents data for storing in the logs
type Message struct {
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

// Importer defines importer interface
// which should be implemented for each importer
type Importer interface {
	Import(ctx context.Context, message Message) error
}

// Exporter defines exporter interface
// which should be implemented for each exporter
type Exporter interface {
	Export(context.Context, func(Message))
}

// Transport defines object with exporter and importer
type Transport struct {
	exporter Exporter
	importer Importer
}

// Run starts export import process
func (p *Transport) Run(ctx context.Context) {
	p.exporter.Export(ctx, func(m Message) {
		if err := p.importer.Import(ctx, m); err != nil {
			log.Printf("error import message to the importer: %s", err)
		}
	})
}

func New(exporter Exporter, importer Importer) *Transport {
	p := Transport{exporter: exporter, importer: importer}
	return &p
}
