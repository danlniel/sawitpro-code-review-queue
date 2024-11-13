package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type Queue struct {
	ID          int
	Title       string
	CreatedAt   time.Time
	TaggedUsers []string
	Approved    bool
}

var (
	queues     = make(map[int]Queue)
	queueID    = 1
	queueMutex sync.Mutex
)

func main() {
	// Simulating Slack commands
	runSlackBot()
}

func runSlackBot() {
	// Example commands
	handleCommand("queue add Task1 mr_link @user1 @user2")
	handleCommand("queue list")
	handleCommand("queue approve 1")
	handleCommand("queue list")
	handleCommand("queue remove 1")
	handleCommand("queue list")
}

// handleCommand processes the Slack commands
func handleCommand(command string) {
	parts := strings.Fields(command)
	if len(parts) < 2 {
		fmt.Println("Invalid command format")
		return
	}

	switch parts[0] {
	case "queue":
		handleQueueCommand(parts[1:], command)
	default:
		fmt.Println("Unknown command")
	}
}

// handleQueueCommand processes all queue-related commands
func handleQueueCommand(args []string, fullCommand string) {
	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Println("Usage: queue add <title_of_queue> <mr_link> @tag @tag")
			return
		}
		addQueue(args[1:])
	case "list":
		listQueues()
	case "remove":
		if len(args) < 2 {
			fmt.Println("Usage: queue remove <id_queue>")
			return
		}
		removeQueue(args[1])
	case "approve":
		if len(args) < 2 {
			fmt.Println("Usage: queue approve <id_queue>")
			return
		}
		approveQueue(args[1])
	default:
		fmt.Println("Unknown queue command")
	}
}

// addQueue adds a new queue
func addQueue(args []string) {
	queueMutex.Lock()
	defer queueMutex.Unlock()

	// Command format: queue add <title_of_queue> <mr_link> @tag @tag
	title := args[0]
	// mr_link is optional for now, just to keep things simple
	taggedUsers := args[1:]

	// Create new queue and add it to map
	newQueue := Queue{
		ID:          queueID,
		Title:       title,
		CreatedAt:   time.Now(),
		TaggedUsers: taggedUsers,
	}

	queues[queueID] = newQueue
	fmt.Printf("Queue added: ID %d, Title: %s, Tagged Users: %v\n", queueID, title, taggedUsers)

	// Increment the queue ID for the next queue
	queueID++
}

// listQueues lists all the registered queues
func listQueues() {
	if len(queues) == 0 {
		fmt.Println("No active queues.")
		return
	}

	for _, queue := range queues {
		status := "Pending"
		if queue.Approved {
			status = "Completed"
		}
		fmt.Printf("ID: %d, Title: %s, Created At: %s, Status: %s, Tagged Users: %v\n",
			queue.ID, queue.Title, queue.CreatedAt.Format(time.RFC1123), status, queue.TaggedUsers)
	}
}

// removeQueue removes a queue by ID
func removeQueue(idStr string) {
	queueMutex.Lock()
	defer queueMutex.Unlock()

	id := parseQueueID(idStr)
	if _, exists := queues[id]; exists {
		delete(queues, id)
		fmt.Printf("Queue ID %d removed.\n", id)
	} else {
		fmt.Printf("Queue with ID %d does not exist.\n", id)
	}
}

// approveQueue approves a queue by ID, removing tagged users
func approveQueue(idStr string) {
	queueMutex.Lock()
	defer queueMutex.Unlock()

	id := parseQueueID(idStr)
	queue, exists := queues[id]
	if !exists {
		fmt.Printf("Queue with ID %d does not exist.\n", id)
		return
	}

	// If the queue has no tagged users, consider it completed
	if len(queue.TaggedUsers) == 0 {
		queue.Approved = true
	} else {
		// Otherwise, remove the first tagged user
		queue.TaggedUsers = queue.TaggedUsers[1:]
	}

	// Update the queue status
	queues[id] = queue
	fmt.Printf("Queue ID %d approved. Remaining tagged users: %v\n", id, queue.TaggedUsers)

	// If no tagged users left, mark as completed
	if len(queue.TaggedUsers) == 0 {
		queue.Approved = true
		queues[id] = queue
		fmt.Printf("Queue ID %d is completed.\n", id)
	}
}

// parseQueueID converts a string to an integer (queue ID)
func parseQueueID(idStr string) int {
	var id int
	_, err := fmt.Sscanf(idStr, "%d", &id)
	if err != nil {
		fmt.Println("Invalid queue ID:", idStr)
	}
	return id
}
