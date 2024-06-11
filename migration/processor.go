package migration

import (
	"context"
	"log"

	"slack2logs/slack"
	"slack2logs/vmlogs"
)

// Importer defines importer interface
// which should be implemented for each importer
type Importer interface {
	Import(ctx context.Context, message vmlogs.Message) error
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

// Run starts export import process
func (p *Processor) Run(ctx context.Context) {
	p.exporter.Export(ctx, func(m slack.Message) {
		if err := p.importer.Import(ctx, vmlogs.Message(m)); err != nil {
			log.Printf("error import message to the importer: %s", err)
		}
	})
}

func New(exporter Exporter, importer Importer) *Processor {
	p := Processor{exporter: exporter, importer: importer}
	return &p
}
