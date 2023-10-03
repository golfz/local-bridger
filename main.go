package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

// replace with your client ID
const clientID = "my_client_id"

type RequestMessage struct {
	RequestID string            `json:"request_id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Header    map[string]string `json:"header"`
	Query     string            `json:"query"`
	Body      string            `json:"body"`
}

type ResponseMessage struct {
	RequestID  string            `json:"request_id"`
	StatusCode int               `json:"status_code"`
	Header     map[string]string `json:"header"`
	Body       string            `json:"body"`
}

func main() {
	// Prepare the WebSocket server URL
	u := url.URL{Scheme: "ws", Host: "localhost:9999", Path: "/ws"} // Change the host and path accordingly

	// Connect to WebSocket server
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("Error while connecting:", err)
	}
	defer c.Close()

	// Send client ID after establishing the connection
	err = c.WriteMessage(
		websocket.TextMessage,
		[]byte(fmt.Sprintf(`{"client_id": "%s"}`, clientID)),
	)
	if err != nil {
		log.Fatal("Error sending client ID:", err)
	}

	for {
		// Wait for a message from the WebSocket server
		messageType, message, err := c.ReadMessage()
		if err != nil {
			log.Println("Error while reading:", err)
			return
		}

		if messageType == websocket.TextMessage {
			// Parse the received message (assuming it's JSON)
			var requestMessage RequestMessage
			if err := json.Unmarshal(message, &requestMessage); err != nil {
				log.Println("Error unmarshalling JSON:", err)
				continue
			}

			// Prepare a JSON response
			response := ResponseMessage{
				RequestID:  requestMessage.RequestID,
				StatusCode: http.StatusOK,
				Header:     map[string]string{"Content-Type": "application/json"},
				Body:       fmt.Sprintf(`{"message": "Hello, %s!"}`, requestMessage.Body),
			}
			responseJSON, err := json.Marshal(response)
			if err != nil {
				log.Println("Error marshalling JSON:", err)
				continue
			}

			// Send the JSON response back to the WebSocket server
			if err := c.WriteMessage(websocket.TextMessage, responseJSON); err != nil {
				log.Println("Error writing response:", err)
			}
		}
	}
}
