package vmlogs

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"slack2logs/auth"
	"slack2logs/transporter"
)

const jsonLinePath = "insert/jsonline"

var (
	vmlogsAddr     = flag.String("vmlogs.addr", "http://localhost:9428", "VictoriaLogs address to perform import requests. Should be the same as --httpListenAddr value of the VictoriaLogs instance.")
	vmlogsUser     = flag.String("vmlogs.auth.user", "", "Username for VictoriaLogs HTTP server's Basic Auth.")
	vmlogsPassword = flag.String("vmlogs.auth.password", "", "Password for VictoriaLogs HTTP server's Basic Auth.")
)

var (
	defaultLogsFields = map[string][]string{
		"_stream_fields": {"channel_id", "channel_name"},
		"_msg_field":     {"text"},
		"_time_field":    {"ts"},
	}

	messagesDeliveryCount = metrics.GetOrCreateCounter(`vm_slack2logs_messages_delivery_total{destination="vmlogs"}`)
	handleMessageErrors   = metrics.GetOrCreateCounter(`vm_slack2logs_delivery_errors_total{destination="vmlogs"}`)
)

// Client is an HTTP client for importing
// logs via jsonline protocol.
type Client struct {
	authCfg     *auth.Config
	extraLabels []string
	httpClient  *http.Client
	url         *url.URL
}

func New() (*Client, error) {
	vmLogsAuthCfg, err := auth.Generate(auth.WithBasicAuth(*vmlogsUser, *vmlogsPassword))
	if err != nil {
		log.Fatalf("error create vmlogs authentication configuration: %s", err)
	}

	c := Client{
		authCfg: vmLogsAuthCfg,
		httpClient: &http.Client{
			Transport: &http.Transport{},
			Timeout:   30 * time.Second,
		},
	}

	reqURL := fmt.Sprintf("%s/%s", *vmlogsAddr, jsonLinePath)
	u, err := url.Parse(reqURL)
	if err != nil {
		return nil, fmt.Errorf("incorrect import address defined %s: %w", *vmlogsAddr, err)
	}

	q := u.Query()
	for k, fieldNames := range defaultLogsFields {
		fields := strings.Join(fieldNames, ",")
		q.Add(k, fields)
	}
	u.RawQuery = q.Encode()
	c.url = u

	return &c, nil
}

// Import make request to the VictoriaLogs server with
// given message
func (c *Client) Import(ctx context.Context, message transporter.Message) error {
	messagesDeliveryCount.Inc()
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(message)
	if err != nil {
		handleMessageErrors.Inc()
		return fmt.Errorf("error marshal message when importing: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url.String(), &buf)
	if err != nil {
		handleMessageErrors.Inc()
		return fmt.Errorf("error create import request: %w", err)
	}

	if c.authCfg != nil {
		c.authCfg.SetHeaders(req, true)
	}

	return c.do(req)
}

func (c *Client) do(req *http.Request) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		handleMessageErrors.Inc()
		return fmt.Errorf("unexpected error when performing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			handleMessageErrors.Inc()
			return fmt.Errorf("failed to read response body for status code %d: %s", resp.StatusCode, err)
		}
		handleMessageErrors.Inc()
		return fmt.Errorf("unexpected response code %d: %s", resp.StatusCode, string(body))
	}
	return err
}
