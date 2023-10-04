package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var privateServerID = ""

type serverInfoMessage struct {
	PrivateServerID string `json:"private_server_id"`
}

type webSocketRequestMessage struct {
	RequestID string            `json:"request_id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Header    map[string]string `json:"header"`
	Query     string            `json:"query"`
	Body      string            `json:"body"`
}

type webSocketResponseMessage struct {
	RequestID  string            `json:"request_id"`
	StatusCode int               `json:"status_code"`
	Header     map[string]string `json:"header"`
	Body       string            `json:"body"`
}

func initConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal("error reading config file:", err)
	}
}

func validateConfig() {
	if viper.GetString("private.server.host") == "" {
		log.Fatal("PRIVATE_SERVER_HOST is required")
	}
	if viper.GetString("cloud.server.host") == "" {
		log.Fatal("CLOUD_SERVER_HOST is required")
	}
}

func validatePrivateServerID() {
	privateServerID = viper.GetString("private.server.id")
	if privateServerID == "" {
		privateServerID = uuid.New().String()
	}
}

func main() {
	initConfig()
	validateConfig()
	validatePrivateServerID()

	for {
		performWebsocketConnection()

		// Reconnect to the WebSocket server
		log.Println("reconnecting...")
		time.Sleep(5 * time.Second)
	}
}

func performWebsocketConnection() {
	defer log.Println("connection closed")

	// Prepare the WebSocket server URL
	u := url.URL{
		Scheme: "ws",
		Host:   viper.GetString("cloud.server.host"),
		Path:   viper.GetString("cloud.server.path"),
	} // Change the host and path accordingly

	// Connect to WebSocket server
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("error while connecting to cloud server:", err)
		return
	}
	defer c.Close()
	log.Println("connected to cloud server:", u.String())

	// Send private server ID after establishing the connection
	serverInfo := serverInfoMessage{
		PrivateServerID: privateServerID,
	}
	serverInfoJSON, err := json.Marshal(serverInfo)
	if err != nil {
		log.Println("error marshalling server info JSON:", err)
		return
	}
	err = c.WriteMessage(
		websocket.TextMessage,
		serverInfoJSON,
	)
	if err != nil {
		log.Println("error sending server info:", err)
		return
	}
	log.Println("sent server info:", privateServerID)

	for {
		// Wait for a message from the WebSocket server
		messageType, message, err := c.ReadMessage()
		if err != nil {
			log.Println("Error while reading:", err)
			return
		}

		if messageType == websocket.TextMessage {

			// Parse the received message (assuming it's JSON)
			var requestMessage webSocketRequestMessage
			if err := json.Unmarshal(message, &requestMessage); err != nil {
				log.Println("Error unmarshalling JSON:", err)
				// return for disconnecting and reconnecting, because we can't handle the error
				return
			}

			// Prepare the HTTP request
			reqBody := bytes.NewBuffer([]byte(requestMessage.Body))
			reqURL := viper.GetString("private.server.host") + requestMessage.Path
			if requestMessage.Query != "" {
				reqURL += "?" + requestMessage.Query
			}
			req, err := http.NewRequest(requestMessage.Method, reqURL, reqBody)
			if err != nil {
				log.Println("Error creating request:", err)
				responseError(c, requestMessage.RequestID, http.StatusInternalServerError, "cannot create request")
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
				responseError(c, requestMessage.RequestID, http.StatusInternalServerError, "cannot send request")
				continue
			}

			// Read the response body
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Println("Error reading response body:", err)
				responseError(c, requestMessage.RequestID, http.StatusInternalServerError, "cannot read response body")
				continue
			}
			defer resp.Body.Close()

			header := make(map[string]string)
			for k, v := range resp.Header {
				header[k] = v[0]
			}

			// Prepare a JSON response
			response := webSocketResponseMessage{
				RequestID:  requestMessage.RequestID,
				StatusCode: resp.StatusCode,
				Header:     header,
				Body:       string(body),
			}
			responseJSON, err := json.Marshal(response)
			if err != nil {
				log.Println("Error marshalling JSON:", err)
				responseError(c, requestMessage.RequestID, http.StatusInternalServerError, "cannot marshal response")
				continue
			}

			// Send the JSON response back to the WebSocket server
			if err := c.WriteMessage(websocket.TextMessage, responseJSON); err != nil {
				log.Println("Error writing response:", err)
				// return for disconnecting and reconnecting, because we can't handle the error
				return
			}
		}
	}
}

func responseToWebsocket(c *websocket.Conn, v webSocketResponseMessage) {
	if err := c.WriteJSON(v); err != nil {
		log.Println("error writing response to websocket:", err)
		return
	}
	log.Println("response sent to websocket success")
}

func responseError(c *websocket.Conn, requestID string, statusCode int, msg string) {
	response := webSocketResponseMessage{
		RequestID:  requestID,
		StatusCode: statusCode,
		Header:     map[string]string{"Content-Type": "application/json"},
		Body:       fmt.Sprintf(`{"error": "%s"}`, msg),
	}
	responseToWebsocket(c, response)
}
