package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type Queue struct {
	ID            int
	Title         string
	MRLink        string
	Tags          []string
	Owner         string
	InReviewState bool
}

type SlackHandler struct {
	API           *slack.Client
	SigningSecret string
	Queues        map[int]*Queue
	NextID        int
	mu            sync.Mutex
	BotUserID     string
}

func NewSlackHandler(botToken, signingSecret string) *SlackHandler {
	client := slack.New(botToken)
	authResp, err := client.AuthTest()
	if err != nil {
		log.Printf("[ERROR] Failed to authenticate bot: %v", err)
	}

	return &SlackHandler{
		API:           client,
		SigningSecret: signingSecret,
		Queues:        make(map[int]*Queue),
		NextID:        1,
		BotUserID:     authResp.UserID,
	}
}

func (sh *SlackHandler) HandleEventEndpoint(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[ERROR] Failed to read request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sv, err := slack.NewSecretsVerifier(r.Header, sh.SigningSecret)
	if err != nil {
		log.Printf("[ERROR] Failed to create secrets verifier: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := sv.Write(body); err != nil {
		log.Printf("[ERROR] Failed to write to secrets verifier: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := sv.Ensure(); err != nil {
		log.Printf("[ERROR] Secret verification failed: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Printf("[ERROR] Failed to parse Slack event: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch eventsAPIEvent.Type {
	case slackevents.URLVerification:
		sh.handleURLVerification(w, body)
	case slackevents.CallbackEvent:
		sh.handleCallbackEvent(w, eventsAPIEvent.InnerEvent)
	default:
		log.Printf("[WARN] Unsupported event type: %s", eventsAPIEvent.Type)
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func (sh *SlackHandler) handleURLVerification(w http.ResponseWriter, body []byte) {
	var challengeResponse *slackevents.ChallengeResponse
	if err := json.Unmarshal(body, &challengeResponse); err != nil {
		log.Printf("[ERROR] Failed to unmarshal challenge response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text")
	w.Write([]byte(challengeResponse.Challenge))
}

func (sh *SlackHandler) handleCallbackEvent(w http.ResponseWriter, innerEvent slackevents.EventsAPIInnerEvent) {
	switch ev := innerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		if ev.User == sh.BotUserID || ev.SubType != "" {
			return
		}
		command := strings.TrimSpace(ev.Text)
		switch {
		case strings.HasPrefix(command, "queue add"):
			sh.handleQueueAdd(w, ev)
		case strings.HasPrefix(command, "queue list"):
			sh.handleQueueList(w, ev)
		case strings.HasPrefix(command, "queue remove"):
			sh.handleQueueRemove(w, ev)
		case strings.HasPrefix(command, "queue approve"):
			sh.handleQueueApprove(w, ev)
		case strings.HasPrefix(command, "queue review"):
			sh.handleQueueReview(w, ev)
		case strings.HasPrefix(command, "queue update"):
			sh.handleQueueUpdate(w, ev)
		case strings.HasPrefix(command, "queue help"):
			sh.handleQueueHelp(w, ev)
		default:
			log.Printf("[INFO] Unrecognized command: %s", command)
		}
	default:
		log.Printf("[WARN] Unsupported inner event type: %T", innerEvent.Data)
	}
}

func (sh *SlackHandler) handleQueueAdd(w http.ResponseWriter, ev *slackevents.MessageEvent) {
	parts := strings.Fields(ev.Text)
	if len(parts) < 4 {
		sh.API.PostMessage(ev.Channel, slack.MsgOptionText("Usage: queue add <title> <MR link> @tag @tag", false))
		return
	}

	sh.mu.Lock()
	defer sh.mu.Unlock()

	queue := &Queue{
		ID:     sh.NextID,
		Title:  parts[2],
		MRLink: parts[3],
		Tags:   parts[4:],
		Owner:  ev.User,
	}
	sh.Queues[sh.NextID] = queue
	sh.NextID++

	msg := fmt.Sprintf("Queue added: *%s*\nMR Link: %s\nTags: %s", queue.Title, queue.MRLink, strings.Join(queue.Tags, ", "))
	sh.API.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))
}

func (sh *SlackHandler) handleQueueList(w http.ResponseWriter, ev *slackevents.MessageEvent) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if len(sh.Queues) == 0 {
		sh.API.PostMessage(ev.Channel, slack.MsgOptionText("No queues available.", false))
		return
	}

	var queueList strings.Builder
	for _, queue := range sh.Queues {
		mention := ""
		if queue.InReviewState {
			mention = fmt.Sprintf("Owner: <@%s>", queue.Owner)
		} else {
			mention = fmt.Sprintf("Tags: %s", strings.Join(queue.Tags, ", "))
		}

		queueList.WriteString(fmt.Sprintf("ID: %d | Title: %s | MR: %s | %s\n",
			queue.ID, queue.Title, queue.MRLink, mention))
	}
	sh.API.PostMessage(ev.Channel, slack.MsgOptionText(queueList.String(), false))
}

func (sh *SlackHandler) handleQueueRemove(w http.ResponseWriter, ev *slackevents.MessageEvent) {
	id, err := parseQueueID(ev.Text)
	if err != nil {
		sh.API.PostMessage(ev.Channel, slack.MsgOptionText(err.Error(), false))
		return
	}

	sh.mu.Lock()
	defer sh.mu.Unlock()

	if _, exists := sh.Queues[id]; !exists {
		sh.API.PostMessage(ev.Channel, slack.MsgOptionText("Queue not found.", false))
		return
	}

	delete(sh.Queues, id)
	sh.API.PostMessage(ev.Channel, slack.MsgOptionText("Queue removed.", false))
}

func (sh *SlackHandler) handleQueueApprove(w http.ResponseWriter, ev *slackevents.MessageEvent) {
	parts := strings.Fields(ev.Text)
	if len(parts) < 3 {
		sh.API.PostMessage(ev.Channel, slack.MsgOptionText("Usage: queue approve <id>", false))
		return
	}

	id, err := strconv.Atoi(parts[2])
	if err != nil {
		sh.API.PostMessage(ev.Channel, slack.MsgOptionText("Invalid queue ID.", false))
		return
	}

	sh.mu.Lock()
	queue, exists := sh.Queues[id]
	if !exists {
		sh.mu.Unlock() // Release lock before returning
		sh.API.PostMessage(ev.Channel, slack.MsgOptionText("Queue not found.", false))
		return
	}

	if len(queue.Tags) > 0 {
		approvedTag := fmt.Sprintf("<@%s>", ev.User) // Format user ID as a Slack tag
		tagIndex := -1

		// Find the tag to remove
		for i, tag := range queue.Tags {
			if tag == approvedTag {
				tagIndex = i
				break
			}
		}

		if tagIndex != -1 {
			// Remove the tag
			queue.Tags = append(queue.Tags[:tagIndex], queue.Tags[tagIndex+1:]...)
			sh.mu.Unlock() // Release lock after update
			sh.API.PostMessage(ev.Channel, slack.MsgOptionText("Queue approved and tag removed.", false))
		} else {
			sh.mu.Unlock() // Release lock
			sh.API.PostMessage(ev.Channel, slack.MsgOptionText("Your tag was not found in the queue.", false))
			return
		}
	} else {
		// No tags left, mark as complete
		sh.mu.Unlock() // Release lock
		sh.API.PostMessage(ev.Channel, slack.MsgOptionText("Queue completed; no tags left.", false))
	}

	// Show the updated list of queues
	sh.handleQueueList(w, ev) // This will use the current queue state
}

func (sh *SlackHandler) handleQueueReview(w http.ResponseWriter, ev *slackevents.MessageEvent) {
	id, err := parseQueueID(ev.Text)
	if err != nil {
		sh.API.PostMessage(ev.Channel, slack.MsgOptionText(err.Error(), false))
		return
	}

	sh.mu.Lock() // Locking the mutex
	queue, exists := sh.Queues[id]
	if !exists {
		sh.mu.Unlock() // Unlocking before early return
		sh.API.PostMessage(ev.Channel, slack.MsgOptionText("Queue not found.", false))
		return
	}

	queue.InReviewState = true
	msg := fmt.Sprintf("Queue %d is now in review.", queue.ID)
	sh.API.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))

	// Unlock the mutex before calling handleQueueList
	sh.mu.Unlock()

	// Now call handleQueueList without holding the mutex
	sh.handleQueueList(w, ev)
}

func (sh *SlackHandler) handleQueueUpdate(w http.ResponseWriter, ev *slackevents.MessageEvent) {
	id, err := parseQueueID(ev.Text)
	if err != nil {
		sh.API.PostMessage(ev.Channel, slack.MsgOptionText(err.Error(), false))
		return
	}

	sh.mu.Lock() // Locking the mutex
	queue, exists := sh.Queues[id]
	if !exists {
		sh.mu.Unlock() // Unlocking before early return
		sh.API.PostMessage(ev.Channel, slack.MsgOptionText("Queue not found.", false))
		return
	}

	queue.InReviewState = false
	msg := fmt.Sprintf("Queue %d has been updated and is no longer in review.", queue.ID)
	sh.API.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))

	// Unlock the mutex before calling handleQueueList
	sh.mu.Unlock()

	// Now call handleQueueList without holding the mutex
	sh.handleQueueList(w, ev)
}

func (sh *SlackHandler) handleQueueHelp(w http.ResponseWriter, ev *slackevents.MessageEvent) {
	helpMessage := `Here are the available queue commands:
- ` + "`queue add <title> <link> @tag @tag...`" + `: Adds a queue with a title, link, and optional tags (user mentions)
  Example: ` + "`queue add \"New Feature\" https://example.com @user1 @user2`" + `
- ` + "`queue list`" + `: Lists all queues
- ` + "`queue remove <queueID>`" + `: Removes a queue by ID
- ` + "`queue approve <queueID>`" + `: Approves a queue by ID
- ` + "`queue review <queueID>`" + `: Marks a queue as under review
- ` + "`queue update <queueID>`" + `: Updates a queue
- ` + "`queue help`" + `: Displays this help message`

	// Send the help message to the Slack channel
	sh.API.PostMessage(ev.Channel, slack.MsgOptionText(helpMessage, false))
}

func parseQueueID(command string) (int, error) {
	parts := strings.Fields(command)
	if len(parts) < 3 {
		return 0, fmt.Errorf("Usage: <command> <id>")
	}

	id, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, fmt.Errorf("Invalid queue ID.")
	}
	return id, nil
}
