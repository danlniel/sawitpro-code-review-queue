package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
)

var (
	queues         = make(map[int]Queue)
	queueIDCounter = 1
)

type Queue struct {
	ID            int
	Title         string
	CreatedBy     string
	TaggedMembers []string
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

func main() {
	r := gin.Default()

	// Slack interaction route
	r.POST("/queue", handleQueue)
	r.Run(":8080") // listen and serve on port 8080
}

func handleQueue(c *gin.Context) {
	var json map[string]string
	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	text := json["text"]
	userID := json["user_id"]
	command := strings.Fields(text)

	switch command[0] {
	case "add":
		addQueue(command[1:], userID, c)
	case "list":
		listQueues(c)
	case "remove":
		removeQueue(command[1:], c)
	case "approve":
		approveQueue(command[1:], userID, c)
	case "report":
		generateReport(c)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid command"})
	}
}

func addQueue(args []string, userID string, c *gin.Context) {
	title := args[0]
	taggedMembers := args[1:]
	queueEntry := Queue{
		ID:            queueIDCounter,
		Title:         title,
		CreatedBy:     userID,
		TaggedMembers: taggedMembers,
		CreatedAt:     time.Now(),
		CompletedAt:   nil,
	}
	queues[queueIDCounter] = queueEntry
	queueIDCounter++

	c.JSON(http.StatusOK, gin.H{
		"response_type": "in_channel",
		"text":          fmt.Sprintf("Queue '%s' added with ID: %d and tagged members: %s", title, queueEntry.ID, strings.Join(taggedMembers, ", ")),
	})
}

func listQueues(c *gin.Context) {
	if len(queues) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"response_type": "in_channel",
			"text":          "No active queues.",
		})
		return
	}

	var queueList string
	for _, queue := range queues {
		if queue.CompletedAt == nil {
			queueList += fmt.Sprintf("ID: %d, Title: %s, Tagged: %s\n", queue.ID, queue.Title, strings.Join(queue.TaggedMembers, ", "))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"response_type": "in_channel",
		"text":          fmt.Sprintf("Active Queues:\n%s", queueList),
	})
}

func removeQueue(args []string, c *gin.Context) {
	queueID := 0
	_, err := fmt.Sscanf(args[0], "%d", &queueID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}

	if _, exists := queues[queueID]; exists {
		delete(queues, queueID)
		c.JSON(http.StatusOK, gin.H{
			"response_type": "in_channel",
			"text":          fmt.Sprintf("Queue ID %d has been removed.", queueID),
		})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Queue ID %d not found.", queueID)})
	}
}

func approveQueue(args []string, userID string, c *gin.Context) {
	queueID := 0
	_, err := fmt.Sscanf(args[0], "%d", &queueID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}

	queueEntry, exists := queues[queueID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue not found"})
		return
	}

	for i, tagged := range queueEntry.TaggedMembers {
		if tagged == userID {
			queueEntry.TaggedMembers = append(queueEntry.TaggedMembers[:i], queueEntry.TaggedMembers[i+1:]...)
			if len(queueEntry.TaggedMembers) == 0 {
				// Mark as completed if no tagged members are left
				completedAt := time.Now()
				queueEntry.CompletedAt = &completedAt
				queues[queueID] = queueEntry
				c.JSON(http.StatusOK, gin.H{
					"response_type": "in_channel",
					"text":          fmt.Sprintf("Queue ID %d has been completed.", queueID),
				})
			} else {
				queues[queueID] = queueEntry
				c.JSON(http.StatusOK, gin.H{
					"response_type": "in_channel",
					"text":          fmt.Sprintf("User %s approved queue ID %d. Remaining tagged members: %s", userID, queueID, strings.Join(queueEntry.TaggedMembers, ", ")),
				})
			}
			return
		}
	}

	c.JSON(http.StatusForbidden, gin.H{"error": "You are not tagged in this queue."})
}

func generateReport(c *gin.Context) {
	if len(queues) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"response_type": "in_channel",
			"text":          "No queues found.",
		})
		return
	}

	var report string
	for _, queue := range queues {
		if queue.CompletedAt != nil {
			duration := queue.CompletedAt.Sub(queue.CreatedAt)
			report += fmt.Sprintf("ID: %d, Title: %s, Duration: %s\n", queue.ID, queue.Title, duration)
		} else {
			report += fmt.Sprintf("ID: %d, Title: %s, Status: In Progress\n", queue.ID, queue.Title)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"response_type": "in_channel",
		"text":          fmt.Sprintf("Queue Report:\n%s", report),
	})
}
