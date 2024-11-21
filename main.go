package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Fatal("[ERROR] Error loading .env file")
	}

	// Create SlackHandler and Server
	slackHandler := NewSlackHandler(os.Getenv("SLACK_BOT_TOKEN"), os.Getenv("SLACK_SIGNING_SECRET"))
	server := NewServer(slackHandler, "3000")

	// Start the server
	server.Start()
}
