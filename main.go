package main

import (
	"fmt"
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
	fmt.Println("Hello, Go!")
}
