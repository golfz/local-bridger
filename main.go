package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

var localID = ""

const (
	viperKeyLocalID        = "local.id"
	viperKeyLocalHost      = "local.host"
	viperKeyCloudWebsocket = "cloud.websocket"
)

var (
	errLocalHostRequired      = errors.New("LOCAL_HOST environment variable is required")
	errCloudWebsocketRequired = errors.New("CLOUD_WEBSOCKET environment variable is required")
)

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

func validateConfig() error {
	if viper.GetString(viperKeyLocalHost) == "" {
		return errLocalHostRequired
	}
	if viper.GetString(viperKeyCloudWebsocket) == "" {
		return errCloudWebsocketRequired
	}
	return nil
}

func initConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	return validateConfig()
}

func getLocalID() string {
	if localID != "" {
		return localID
	}
	if localID = strings.TrimSpace(viper.GetString(viperKeyLocalID)); localID == "" {
		localID = uuid.New().String()
	}
	return localID
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	if err := initConfig(); err != nil {
		log.Fatalf("error initializing config: %v", err)
	}

	for {
		performWebsocketConnection()

		// Reconnect to the WebSocket server
		log.Println("reconnecting...")
		time.Sleep(5 * time.Second)
	}
}

func performWebsocketConnection() {
	defer log.Println("connection closed")

	wsURL := viper.GetString("cloud.websocket")

	// Connect to WebSocket server
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Println("error while connecting to cloud server:", err)
		return
	}
	defer c.Close()
	log.Println(`connected to "Cloud-Bridger":`, wsURL)

	// Send private server ID after establishing the connection
	serverInfo := serverInfoMessage{
		PrivateServerID: getLocalID(),
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
	log.Println(`registered local to "Cloud-Bridger"`)
	log.Printf("X-Private-Server-ID: %s\n", getLocalID())

	for {
		// Wait for a message from the WebSocket server
		messageType, message, err := c.ReadMessage()
		if err != nil {
			log.Println("Error while reading:", err)
			return
		}

		if messageType == websocket.PingMessage {
			err := c.WriteMessage(websocket.PongMessage, []byte{})
			if err != nil {
				log.Println("Error while sending pong:", err)
				// return for disconnecting and reconnecting, because we can't handle the error
				return
			}
			continue
		} else if messageType == websocket.TextMessage {

			// Parse the received message (assuming it's JSON)
			var requestMessage webSocketRequestMessage
			if err := json.Unmarshal(message, &requestMessage); err != nil {
				log.Println("Error unmarshalling JSON:", err)
				// return for disconnecting and reconnecting, because we can't handle the error
				return
			}

			log.Printf("[requestID: %s] received request message, method: %s, path: %s\n", requestMessage.RequestID, requestMessage.Method, requestMessage.Path)

			// Prepare the HTTP request
			reqBody := bytes.NewBuffer([]byte(requestMessage.Body))
			reqURL := viper.GetString("local.host") + requestMessage.Path
			if requestMessage.Query != "" {
				reqURL += "?" + requestMessage.Query
			}
			req, err := http.NewRequest(requestMessage.Method, reqURL, reqBody)
			if err != nil {
				log.Printf("[requestID: %s] Error creating request: %v\n", requestMessage.RequestID, err)
				responseError(c, requestMessage.RequestID, http.StatusInternalServerError, "cannot create request")
				continue
			}

			// Set the request headers
			for k, v := range requestMessage.Header {
				req.Header.Set(k, v)
			}

			log.Printf("[requestID: %s] try to request to private server, method: %s, path: %s\n", requestMessage.RequestID, requestMessage.Method, requestMessage.Path)

			// Send the HTTP request
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Printf("[requestID: %s] Error sending request: %v\n", requestMessage.RequestID, err)
				responseError(c, requestMessage.RequestID, http.StatusInternalServerError, "cannot send request")
				continue
			}

			// Read the response body
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("[requestID: %s] Error reading response body: %v\n", requestMessage.RequestID, err)
				responseError(c, requestMessage.RequestID, http.StatusInternalServerError, "cannot read response body")
				continue
			}
			defer resp.Body.Close()

			log.Printf("[requestID: %s] received response from private server, status code: %d\n", requestMessage.RequestID, resp.StatusCode)

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
				log.Printf("[requestID: %s] Error marshalling response: %v\n", requestMessage.RequestID, err)
				responseError(c, requestMessage.RequestID, http.StatusInternalServerError, "cannot marshal response")
				continue
			}

			log.Printf("[requestID: %s] try to send response to cloud server\n", requestMessage.RequestID)

			// Send the JSON response back to the WebSocket server
			if err := c.WriteMessage(websocket.TextMessage, responseJSON); err != nil {
				log.Printf("[requestID: %s] Error sending response: %v\n", requestMessage.RequestID, err)
				// return for disconnecting and reconnecting, because we can't handle the error
				return
			}

			log.Printf("[requestID: %s] response sent to cloud server success\n", requestMessage.RequestID)
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
