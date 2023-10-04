# ws2rest

**Local Bridger** for WebSockets to REST

## Configuration

- see `config.yaml` for configuration options
- for Environment Variables, replace `.` with `_` and use all caps

### Environment Variables

**Private Server**

| Environment Variable  | Description                    | Comment  |
|-----------------------|--------------------------------|----------|
| `PRIVATE_SERVER_HOST` | the host of the private server | required |
| `PRIVATE_SERVER_ID`   | the id of the private server   | required |

**Cloud Server**

| Environment Variable | Description                                     | Comment  |
|----------------------|-------------------------------------------------|----------|
| `CLOUD_SERVER_HOST`  | the host of the cloud server                    | required |
| `CLOUD_SERVER_PATH`  | the path of the cloud server, defaults to `/ws` | optional |
