package slack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"slack2logs/flagutil"
)

const (
	historicalRequestLimit = 500
	joinedChannelMessage   = "> has joined the channel"
	idLength               = 10
)

var (
	botToken           = flag.String("slack.auth.botToken", "", "Bot user OAuth token for Your Workspace")
	appToken           = flag.String("slack.auth.appToken", "", "App-level tokens allow your app to use platform features that apply to multiple (or all) installations")
	listeningChannels  = flagutil.NewArrayString("slack.channels", "Channels ids from slack to listen messages")
	batchFlushInterval = flag.Duration("slack.batchFlushInterval", 10*time.Second, "Interval for flushing batch of messages to the additional service")
)

var (
	messagesReceivedCount = metrics.GetOrCreateCounter(`vm_slack2logs_messages_received_total{source="slack"}`)
	messageOutCount       = metrics.GetOrCreateCounter(`vm_slack2logs_messages_out_total{source="slack"}`)
	handleMessageErrors   = metrics.GetOrCreateCounter(`vm_slack2logs_errors_total{source="slack"}`)
)

// Client represents slack client
type Client struct {
	socketClient      *socketmode.Client
	messageC          chan Message
	threadC           chan ThreadRequest
	listeningChannels map[string]struct{}

	mx    sync.Mutex
	batch Messages
}

// Message represents a slack message
// which would be sent to the additional service
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

// ThreadRequest represents request for getting
// historical thread messages
type ThreadRequest struct {
	ChannelID string
	Timestamp string
}

type Messages map[string]Message

func New() *Client {
	if len(*listeningChannels) == 0 {
		log.Fatalf("got %d slack channels to listen to. At least one slack channel should be defined", len(*listeningChannels))
	}
	client := slack.New(*botToken, slack.OptionAppLevelToken(*appToken))
	// go-slack comes with a SocketMode package that we need to use that
	// accepts a Slack client and outputs a Socket mode client instead
	socketClient := socketmode.New(
		client,
		// Option to set a custom logger
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	c := Client{
		socketClient:      socketClient,
		messageC:          make(chan Message, 1),
		threadC:           make(chan ThreadRequest, 1),
		listeningChannels: make(map[string]struct{}, len(*listeningChannels)),
		batch:             make(Messages),
	}
	for _, ch := range *listeningChannels {
		c.listeningChannels[ch] = struct{}{}
	}
	return &c
}

// Run starts slack websocket client and event listener
func (c *Client) Run(ctx context.Context) error {
	go c.handleEvents(ctx)
	return c.socketClient.RunContext(ctx)
}

// RunHistoricalBackfilling starts websocket client and collect
// historical messages and threads
func (c *Client) RunHistoricalBackfilling(ctx context.Context) error {
	go c.collectHistoricalMessages(ctx)
	go c.collectThreadMessages(ctx)
	err := c.socketClient.RunContext(ctx)
	if err != nil {
		return fmt.Errorf("error run slack socket client: %w", err)
	}
	return nil
}

// Export sends slack message to the additional service via callback
func (c *Client) Export(ctx context.Context, cb func(m Message)) {
	ticker := time.NewTicker(*batchFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			c.flush(cb)
			return
		case <-ticker.C:
			c.flush(cb)
		case m, ok := <-c.messageC:
			if !ok {
				return
			}
			cb(m)
			messageOutCount.Inc()
		}
	}
}

func (c *Client) flush(cb func(m Message)) {
	if len(c.batch) == 0 {
		return
	}
	log.Printf("sending batch of %d messages", len(c.batch))
	c.mx.Lock()
	for _, m := range c.batch {
		cb(m)
	}
	messageOutCount.Add(len(c.batch))
	clear(c.batch)
	c.mx.Unlock()
	log.Printf("batch flushed successfuly")
}

func (c *Client) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down socketmode listener")
			close(c.messageC)
			return
		case event := <-c.socketClient.Events:
			// We have a new Events, let's type switch the event
			// Add more use cases here if you want to listen to other events.
			switch event.Type {
			// handle EventAPI events
			case socketmode.EventTypeEventsAPI:
				// The Event sent on the channel is not the same as the EventAPI events so we need to type cast it
				eventsAPIEvent, ok := event.Data.(slackevents.EventsAPIEvent)
				if !ok {
					log.Printf("Could not type cast the event to the EventsAPIEvent: %v\n", event)
					handleMessageErrors.Inc()
					continue
				}
				if err := c.handleEventMessage(ctx, eventsAPIEvent); err != nil {
					log.Printf("error handle event message: %s", err)
					handleMessageErrors.Inc()
					continue
				}
				// We need to send an Acknowledge to the slack server
				err := c.socketClient.AckCtx(ctx, event.Request.EnvelopeID, *event.Request)
				if err != nil {
					log.Printf("error ack to the channel: %s", err)
					handleMessageErrors.Inc()
					continue
				}
			}
		}
	}
}

func (c *Client) handleEventMessage(ctx context.Context, event slackevents.EventsAPIEvent) error {

	switch event.Type {
	// First we check if this is an CallbackEvent
	case slackevents.CallbackEvent:

		innerEvent := event.InnerEvent
		// Yet Another Type switch on the actual Data to see if its an AppMentionEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			messagesReceivedCount.Inc()
			if _, ok := c.listeningChannels[ev.Channel]; !ok {
				return fmt.Errorf("got message from unsupported channel id: %s", ev.Channel)
			}
			// skip messages like join channel
			if filterOutLogMessage(ev.Text) {
				return nil
			}
			if ev.SubType == slack.MsgSubTypeMessageChanged {
				ev.User = ev.Message.User
				ev.Text = ev.Message.Text
				if ev.Message.ThreadTimeStamp != ev.Message.TimeStamp {
					// this is thread message
					ev.ThreadTimeStamp = ev.Message.ThreadTimeStamp
				}
			}

			threadTS := ev.ThreadTimeStamp
			if ev.ThreadTimeStamp == "" {
				threadTS = ev.TimeStamp
			}
			user, err := c.socketClient.GetUserInfoContext(ctx, ev.User)
			if err != nil {
				return fmt.Errorf("error get user from message: %w", err)
			}
			ch, err := c.socketClient.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
				ChannelID: ev.Channel,
			})
			if err != nil {
				return fmt.Errorf("error get conversation info: %s", err)
			}
			ts, err := strconv.ParseFloat(ev.TimeStamp, 64)
			if err != nil {
				return fmt.Errorf("fail to parse timestamp:%q: %s", ev.TimeStamp, err)
			}

			id := generateMessageID(threadTS)

			m := Message{
				ThreadID:              id,
				Type:                  ev.Type,
				User:                  ev.User,
				Text:                  ev.Text,
				ThreadTimeStamp:       threadTS,
				TimeStamp:             time.Unix(int64(ts), 0).Format(time.RFC3339),
				ChannelID:             ev.Channel,
				ChannelName:           ch.Name,
				UserID:                user.ID,
				DisplayName:           user.Profile.DisplayName,
				DisplayNameNormalized: user.Profile.DisplayNameNormalized,
			}

			if ev.SubType == slack.MsgSubTypeMessageChanged {
				if ev.PreviousMessage.ThreadTimeStamp != "" {
					m.ThreadTimeStamp = ev.PreviousMessage.ThreadTimeStamp
				} else {
					m.ThreadTimeStamp = ev.PreviousMessage.TimeStamp
				}
			}

			timestamp := getBatchTimestamp(ev)
			c.mx.Lock()
			c.batch[timestamp] = m
			c.mx.Unlock()
		default:
			return errors.New("got unsupported inner event type")
		}
	default:
		return errors.New("unsupported event type")
	}
	return nil
}

func (c *Client) collectHistoricalMessages(ctx context.Context) {
	var wg sync.WaitGroup
	for ch := range c.listeningChannels {
		wg.Add(1)
		go func(channelID string) {
			defer wg.Done()
			for {
				var cursor string
				params := &slack.GetConversationHistoryParameters{
					ChannelID:          channelID,
					Cursor:             cursor,
					Inclusive:          true,
					Limit:              historicalRequestLimit,
					IncludeAllMetadata: false,
				}

				historyContext, err := c.socketClient.GetConversationHistoryContext(ctx, params)
				if err != nil {
					log.Printf("error get historical conversation for channel id %s, with error: %s", channelID, err)
					break
				}
				if len(historyContext.Error) != 0 {
					log.Printf("error with history context: %s", err)
					break
				}
				for _, m := range historyContext.Messages {
					user, err := c.socketClient.GetUserInfoContext(ctx, m.User)
					if err != nil {
						log.Printf("error get user %q from message: %s", m.User, err)
						if errors.Is(err, context.Canceled) {
							return
						}
						continue
					}
					ch, err := c.socketClient.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
						ChannelID: channelID,
					})
					if err != nil {
						log.Printf("error get conversation info: %s", err)
						if errors.Is(err, context.Canceled) {
							return
						}
						continue
					}
					ts, err := strconv.ParseFloat(m.Timestamp, 64)
					if err != nil {
						log.Printf("fail to parse timestamp:%q: %s", m.Timestamp, err)
						continue
					}
					c.threadC <- ThreadRequest{
						ChannelID: channelID,
						Timestamp: m.Timestamp,
					}

					if m.ThreadTimestamp == "" {
						m.ThreadTimestamp = m.Timestamp
					}
					c.messageC <- Message{
						Type:                  m.Type,
						User:                  m.User,
						Text:                  m.Text,
						ThreadTimeStamp:       m.ThreadTimestamp,
						TimeStamp:             time.Unix(int64(ts), 0).Format(time.RFC3339),
						ChannelID:             channelID,
						ChannelName:           ch.Name,
						UserID:                user.ID,
						DisplayName:           user.Profile.DisplayName,
						DisplayNameNormalized: user.Profile.DisplayNameNormalized,
					}
				}
				cursor = historyContext.ResponseMetaData.NextCursor
				if !historyContext.HasMore {
					break
				}
			}
		}(ch)
	}
	wg.Wait()
	close(c.messageC)
	close(c.threadC)
}

func (c *Client) collectThreadMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case threadInfo := <-c.threadC:
			var repliesCursor string
			repliesMessages, hasMore, nextCursor, err := c.socketClient.GetConversationRepliesContext(ctx, &slack.GetConversationRepliesParameters{
				ChannelID:          threadInfo.ChannelID,
				Timestamp:          threadInfo.Timestamp,
				Cursor:             repliesCursor,
				Inclusive:          true,
				Limit:              historicalRequestLimit,
				IncludeAllMetadata: false,
			})
			if err != nil {
				log.Printf("error get replies for timestamp: %q", threadInfo.Timestamp)
				continue
			}
			for _, rp := range repliesMessages {
				user, err := c.socketClient.GetUserInfoContext(ctx, rp.User)
				if err != nil {
					log.Printf("error get user %q from message: %s", rp.User, err)
					continue
				}
				ch, err := c.socketClient.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
					ChannelID: threadInfo.ChannelID,
				})
				if err != nil {
					log.Printf("error get conversation info for channel %q with timestamp %s: %s", threadInfo.ChannelID, threadInfo.Timestamp, err)
					continue
				}
				ts, err := strconv.ParseFloat(rp.Timestamp, 64)
				if err != nil {
					log.Printf("fail to parse timestamp:%q: %s", rp.Timestamp, err)
					continue
				}
				c.messageC <- Message{
					Type:                  rp.Type,
					User:                  rp.User,
					Text:                  rp.Text,
					ThreadTimeStamp:       rp.ThreadTimestamp,
					TimeStamp:             time.Unix(int64(ts), 0).Format(time.RFC3339),
					ChannelID:             threadInfo.ChannelID,
					ChannelName:           ch.Name,
					UserID:                user.ID,
					DisplayName:           user.Profile.DisplayName,
					DisplayNameNormalized: user.Profile.DisplayNameNormalized,
				}
			}
			repliesCursor = nextCursor
			if !hasMore {
				break
			}
		}
	}
}

func getBatchTimestamp(ev *slackevents.MessageEvent) string {
	timestamp := ev.TimeStamp
	if ev.SubType == slack.MsgSubTypeMessageChanged {
		timestamp = ev.PreviousMessage.TimeStamp
	}
	return timestamp
}

func generateMessageID(threadTs string) string {
	hash := sha256.New()
	hash.Write([]byte(threadTs))
	hashBytes := hash.Sum(nil)
	encoded := hex.EncodeToString(hashBytes)
	id := encoded[:idLength]
	return id
}

func filterOutLogMessage(msg string) bool {
	// filter out "user joined Slack channel messages", msg example "<@U0787V2AW9W> has joined the channel"
	return strings.HasSuffix(msg, joinedChannelMessage)
}
