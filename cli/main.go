package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"slack2logs/slack"
	"slack2logs/transporter"
	"slack2logs/vmlogs"
)

func main() {
	log.Println("Start migrate historical messages from slack to vmlogs")
	startTime := time.Now()

	flag.CommandLine.SetOutput(os.Stdout)
	flag.Parse()

	// Create a context that can be used to cancel goroutine
	ctx, cancel := context.WithCancel(context.Background())

	log.Println("Init slack client")
	slackClient := slack.New()
	go func() {
		log.Println("Start listen message in the channels")
		if err := slackClient.RunHistoricalBackfilling(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("error run slack client: %s", err)
		}
	}()
	log.Println("Init vmlogs client")
	logs, err := vmlogs.New()
	if err != nil {
		log.Fatalf("error initialize VictoriaLogs client: %s", err)
	}

	trns := transporter.New(slackClient, logs)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Gracefully shutting down process")
		// Make this cancel called properly in a real program, graceful shutdown etc
		cancel()
	}()

	trns.Run(ctx)

	log.Println("Process stopped successfully")
	log.Printf("Elapsed time: %s", time.Since(startTime))
}
