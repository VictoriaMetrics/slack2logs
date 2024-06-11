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

	"slack2logs/envflag"
	"slack2logs/flagutil"
	"slack2logs/httpserver"
	"slack2logs/processor"
	"slack2logs/slack"
	"slack2logs/vmlogs"
)

func main() {
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()

	// Create a context that can be used to cancel goroutine
	ctx, cancel := context.WithCancel(context.Background())

	log.Println("Init slack client")
	slackClient := slack.New()
	go func() {
		log.Println("Start listen message in the channels")
		if err := slackClient.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("error run slack client: %s", err)
		}
	}()
	log.Println("Init vmlogs client")
	logs, err := vmlogs.New()
	if err != nil {
		log.Fatalf("error initialize VictoriaLogs client: %s", err)
	}

	prcs := processor.New(slackClient, logs)

	go httpserver.Serve()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Gracefully shutting down process")
		// Make this cancel called properly in a real program, graceful shutdown etc
		cancel()
	}()

	prcs.Run(ctx)

	err = httpserver.Stop()
	if err != nil {
		log.Fatalf("error shutdown http server: %s", err)
	}
	log.Println("Process stopped successfully")
}

func usage() {
	const s = `
slack2logs collects messages from the slack channels, converts them and save to the VictoriaLogs.
`
	flagutil.Usage(s)
}
