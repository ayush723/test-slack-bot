package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	godotenv.Load(".env")

	token := os.Getenv("SLACK_AUTH_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")
	slackClient := slack.New(token, slack.OptionDebug(true), slack.OptionAppLevelToken(appToken))

	socketClient := socketmode.New(
		slackClient,
		socketmode.OptionDebug(false),
		// Option to set a custom logger
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processNewEvents(ctx, slackClient, socketClient)
	socketClient.Run()

}

// processNewEvents will be reading from Events channel
func processNewEvents(ctx context.Context, client *slack.Client, socketClient *socketmode.Client) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down events listener.")
			return
		case event := <-socketClient.Events:
			switch event.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := event.Data.(slackevents.EventsAPIEvent)
				if !ok {
					log.Printf("Could not type cast the event to the EventsAPIEvent: %v\n", event)
					continue
				}
				socketClient.Ack(*event.Request)
				err := handleEventMessage(eventsAPIEvent, client)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func handleEventMessage(event slackevents.EventsAPIEvent, slackClient *slack.Client) error {
	switch event.Type {
	case slackevents.CallbackEvent:
		innerEvent := event.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			err := handleAppMentionEvent(ev, slackClient)
			if err != nil {
				return err
			}
		}
	default:
		return errors.New("unsupported event type")
	}
	return nil
}

func handleAppMentionEvent(event *slackevents.AppMentionEvent, slackClient *slack.Client) error {
	// Grab the user name based on the ID of the one who mentioned the bot
	user, err := slackClient.GetUserInfo(event.User)
	if err != nil {
		return err
	}
	text := strings.ToLower(event.Text)
	attachment := slack.Attachment{
		Fields: []slack.AttachmentField{
			{
				Title: "Date",
				Value: time.Now().Local().String(),
			},
			{
				Title: "Initializer",
				Value: user.Name,
				Short: true,
			},
		},
	}
	if strings.Contains(text, "hello") {
		attachment.Text = fmt.Sprintf("Hello, %s", user.RealName)
		attachment.Pretext = "Greetings"
		attachment.Color = "#4af030"
	} else {
		// Send a message to the user
		attachment.Text = fmt.Sprintf("How can I help you %s?", user.RealName)
		attachment.Pretext = "How can I be of service?"
		attachment.Color = "#3d3d3d"
	}
	channelId := os.Getenv("SLACK_CHANNEL_ID")
	_, _, err = slackClient.PostMessage(channelId, slack.MsgOptionAttachments(attachment))
	if err != nil {
		return err
	}
	return nil
}
