package httpserver

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

var (
	listenAddr                  = flag.String("http.listenAddr", ":8420", "address with port for listening for HTTP requests")
	maxGracefulShutdownDuration = flag.Duration("http.maxGracefulShutdownDuration", 3*time.Second, `The maximum duration for a graceful shutdown of the HTTP server. A highly loaded server may require increased value for a graceful shutdown`)
)

var server *http.Server

func Serve() {
	mux := http.NewServeMux()
	mux.Handle("/", handleRootPath())
	mux.Handle("/metrics", handleMetricsPath())
	mux.Handle("/health", handleHealth())

	server = &http.Server{
		Addr:    *listenAddr,
		Handler: mux,
	}

	err := server.ListenAndServe()
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			// The server gracefully closed.
			return
		}
		log.Fatalf("FATAL: cannot serve http at %s: %s", *listenAddr, err)
	}
}

// Stop stops the http server on the given addr, which has been started
// via Serve func.
func Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), *maxGracefulShutdownDuration)
	defer cancel()
	log.Println("Shutting down web server")
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("cannot gracefully shutdown http server in %.3fs; "+
			"probably, `-http.maxGracefulShutdownDuration` command-line flag value must be increased; error: %s", maxGracefulShutdownDuration.Seconds(), err)
	}
	return nil
}

func handleRootPath() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		_, err := w.Write([]byte(`<html>
	        <head><title>VictoriaMetrics SlackToLogs processor</title></head>
	            <body>
	               <h1>VictoriaMetrics SlackToLogs</h1>
					<p>For more information, visit <a href='https://github.com/VictoriaMetrics/slack2logs'>GitHub</a></p>
					<ul>
						<li><a href='/metrics'>Metrics</a></li>
					</ul>
	                <p></p>
	           </body>
	    </html>`))

		if err != nil {
			respondWithError(w, r, http.StatusBadRequest, fmt.Errorf("error write response: %s", err))
		}
	})
}

func handleMetricsPath() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		metrics.WritePrometheus(w, true)
	})
}

func handleHealth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func respondWithError(w http.ResponseWriter, r *http.Request, statusCode int, err error) bool {
	fmt.Errorf(err.Error(), r.URL.Path)
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(err.Error()))
	return true
}
