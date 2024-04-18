# Local Bridger

A local service that receives JSON messages from a cloud server via a WebSocket, converts them into HTTP requests and
sends them to a private local server.

It then collects the HTTP responses, converts them into JSON messages, and sends them back to a "Cloud Bridger" via a
WebSocket.

## Configuration

- see `config.yaml` for configuration options
- for Environment Variables, replace `.` with `_` and use all caps

### Environment Variables

**Local**

| Environment Variable | Description                                                        | Comment  |
|----------------------|--------------------------------------------------------------------|----------|
| `LOCAL_ID`           | the unique id of the private local server, e.g. `local-Em4Yk899`   | required |
| `LOCAL_HOST`         | the host of the private local server, e.g. `http://localhost:8080` | required |

**Cloud**

| Environment Variable | Description                                                           | Comment  |
|----------------------|-----------------------------------------------------------------------|----------|
| `CLOUD_WEBSOCKET`    | path to connect to the cloud's WebSocket, e.g. `wss://example.com/ws` | required |

## Running

**Example**

```shell
LOCAL_ID=Em4Yk899 LOCAL_HOST=http://localhost:8080 CLOUD_WEBSOCKET=wss://example.com/ws go run main.go
```

You must change the values of `LOCAL_ID`, `LOCAL_HOST`, and `CLOUD_WEBSOCKET` to match your setup.