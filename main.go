package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"net/url"
)

// replace with your client ID
const privateServerID = "my_client_id"
const localServer = "http://localhost:8000"

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

	log.Println("connected to bridger server:", u.String())

	// Send private server ID after establishing the connection
	err = c.WriteMessage(
		websocket.TextMessage,
		[]byte(fmt.Sprintf(`{"private_server_id": "%s"}`, privateServerID)),
	)
	if err != nil {
		log.Fatal("Error sending private_server_id:", err)
	}

	log.Println("sent private_server_id:", privateServerID)

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

			b := bytes.NewBuffer([]byte(requestMessage.Body))

			// Prepare the HTTP request
			req, err := http.NewRequest(requestMessage.Method, localServer+requestMessage.Path, b)
			if err != nil {
				log.Println("Error creating request:", err)
				continue
			}

			// Set the request headers
			for k, v := range requestMessage.Header {
				req.Header.Set(k, v)
			}

			// Send the HTTP request
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Println("Error sending request:", err)
				continue
			}

			// Read the response body
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Println("Error reading response body:", err)
				continue
			}
			defer resp.Body.Close()

			header := make(map[string]string)
			for k, v := range resp.Header {
				header[k] = v[0]
			}

			// Prepare a JSON response
			response := ResponseMessage{
				RequestID:  requestMessage.RequestID,
				StatusCode: resp.StatusCode,
				Header:     header,
				Body:       string(body),
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
