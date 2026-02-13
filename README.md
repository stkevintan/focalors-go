# Focalors-Go

A multi-platform chat bot framework written in Go, supporting **WeChat** and **Lark (Feishu)** as messaging platforms. It features a middleware-based message processing pipeline, OpenAI tool-calling integration, and a Yunzai-Bot bridge for Genshin Impact/Honkai: Star Rail/Zenless Zone Zero commands.

## Features

- **Multi-platform**: Connect to WeChat or Lark with a single configuration switch
- **Middleware pipeline**: Chain-of-responsibility message handling — each middleware can intercept, process, or pass through messages
- **OpenAI tool calling**: Natural language interface with extensible function tools (weather, image fetching, etc.)
- **Yunzai bridge**: Forward `#`/`*`/`%` prefixed commands to a [Yunzai-Bot](https://github.com/KimigaiiWuworworworworworworyi/Yunzai-Bot) instance via WebSocket
- **Avatar management**: Users can upload custom avatars via private chat (`#上传头像`)
- **Access control**: Admin-managed per-user/per-group permission system
- **Scheduled tasks**: Cron-based jobs with Redis-backed deduplication
- **Structured logging**: Context-aware logging with `slog`

## Deployment

### Docker (recommended)

The image is published to GitHub Container Registry:

```
ghcr.io/stkevintan/focalors-go:latest
```

#### Docker Compose

```yaml
services:
  focalors-go:
    image: ghcr.io/stkevintan/focalors-go:latest
    container_name: focalors-go
    restart: unless-stopped
    volumes:
      - ./config.toml:/root/config.toml:ro
    environment:
      - FOCALORS_APP_REDIS_ADDR=redis:6379
    depends_on:
      - redis

  redis:
    image: redis:7-alpine
    container_name: focalors-redis
    restart: unless-stopped
    command: redis-server --requirepass changeme
    volumes:
      - redis-data:/data

  yunzai:
    container_name: yunzai
    image: ghcr.io/stkevintan/yunzai-docker:main
    restart: unless-stopped
    volumes:
      - ./yunzai/config/:/app/Miao-Yunzai/config/config/ # Bot基础配置文件
      - ./yunzai/logs:/app/Miao-Yunzai/logs # 日志文件
      - ./yunzai/data:/app/Miao-Yunzai/data # 数据文件
      #plugin 目录,不能挂载整个目录
      - ./yunzai/plugins/genshin:/app/Miao-Yunzai/plugins/genshin
      - ./yunzai/plugins/miao-plugin:/app/Miao-Yunzai/plugins/miao-plugin
      - ./yunzai/plugins/xiaoyao-cvs-plugin:/app/Miao-Yunzai/plugins/xiaoyao-cvs-plugin
      - ./yunzai/plugins/ZZZ-Plugin:/app/Miao-Yunzai/plugins/ZZZ-Plugin
    ports:
      - 2536:2536
    environment:
      - REDIS_HOST=redis
      - REDIS_PORT=6379

    healthcheck:
      test: curl --fail http://localhost:2536 || exit 1
      start_period: 20s
      interval: 60s
      retries: 5
      timeout: 10s
    depends_on:
      - redis
volumes:
  redis-data:
```

Create a `config.toml` alongside the compose file (see [Configuration](#configuration)), then:

```bash
docker compose up -d
```

Optionally, you can update the yunzai config in the `./yunzai/config` directory.

### Build from source

```bash
go build -o focalors-go .
./focalors-go            # uses ./config.toml by default
./focalors-go -c /path/to/config.toml
```

## Configuration

All configuration is in a single `config.toml` file. The application searches for it in `.`, `./config`, `/app`, `~/.focalors-go`, or `/etc/focalors-go` — or pass an explicit path with `-c`.

### `[app]` — Application settings

| Field      | Type     | Description                                                  |
| ---------- | -------- | ------------------------------------------------------------ |
| `debug`    | bool     | Enable debug mode (verbose logging)                          |
| `loglevel` | string   | Log level: `debug`, `info`, `warn`, `error`                  |
| `admin`    | string[] | User IDs with admin privileges (platform-specific format)    |
| `platform` | string   | Messaging platform to use: `"wechat"` or `"lark"`           |

### `[app.redis]` — Redis connection

| Field      | Type   | Description            |
| ---------- | ------ | ---------------------- |
| `addr`     | string | Redis server address   |
| `password` | string | Redis password         |
| `db`       | int    | Redis database number  |

### `[wechat]` — WeChat platform settings

| Field           | Type   | Description                                     |
| --------------- | ------ | ----------------------------------------------- |
| `server`        | string | WeChat HTTP API server URL                      |
| `subURL`        | string | WebSocket URL for receiving messages             |
| `token`         | string | API authentication token                        |
| `webhookSecret` | string | Webhook verification secret                     |
| `webhookHost`   | string | Host address for webhook callbacks              |

### `[lark]` — Lark (Feishu) platform settings

| Field               | Type   | Description                        |
| -------------------- | ------ | ---------------------------------- |
| `appId`              | string | Lark app ID                        |
| `appSecret`          | string | Lark app secret                    |
| `verificationToken`  | string | Event subscription verification token |

### `[yunzai]` — Yunzai-Bot bridge

| Field    | Type   | Description                                 |
| -------- | ------ | ------------------------------------------- |
| `server` | string | Yunzai GSUIDCore WebSocket endpoint          |

### `[openai]` — OpenAI / Azure OpenAI settings

| Field        | Type   | Description                               |
| ------------ | ------ | ----------------------------------------- |
| `apiKey`     | string | API key                                   |
| `endpoint`   | string | API endpoint URL (Azure OpenAI format)    |
| `deployment` | string | Model deployment name                     |
| `apiVersion` | string | API version string (e.g. `2025-01-01-preview`) |

### `[weather]` — Weather service (Amap/Gaode API)

| Field | Type   | Description         |
| ----- | ------ | ------------------- |
| `key`  | string | Amap Web API key   |

## Developing

### Project structure

```
main.go              # Entry point, wires everything together
config/              # Configuration loading (viper/toml)
client/              # GenericClient & GenericMessage interfaces
wechat/              # WeChat platform implementation
lark/                # Lark platform implementation
db/                  # Redis wrapper and data stores (AvatarStore, JiandanStore)
middlewares/         # Message processing pipeline
tooling/             # OpenAI function-calling tools
service/             # Business logic (weather, jiandan, access)
scheduler/           # Cron task runner
slogger/             # Structured logger factory
yunzai/              # Yunzai-Bot WebSocket client
```

### Adding a new middleware

Middlewares process incoming messages in a chain. Each one can handle a message and stop propagation (return `true`) or pass it along (return `false`).

1. Create a new file in `middlewares/`, e.g. `middlewares/hello.go`:

```go
package middlewares

import (
    "context"
    "focalors-go/client"
)

type helloMiddleware struct {
    *MiddlewareContext // embed for access to redis, cfg, client, avatarStore, etc.
}

func NewHelloMiddleware(base *MiddlewareContext) Middleware {
    return &helloMiddleware{MiddlewareContext: base}
}

func (h *helloMiddleware) OnMessage(ctx context.Context, msg client.GenericMessage) bool {
    if msg.IsText() && msg.GetText() == "hello" {
        h.SendText(msg, "Hello! 👋")
        return true // handled, stop propagation
    }
    return false // pass to next middleware
}

func (h *helloMiddleware) Start() error { return nil }
func (h *helloMiddleware) Stop() error  { return nil }
```

2. Register it in `main.go`:

```go
m.AddMiddlewares(
    middlewares.NewLogMsgMiddleware,
    middlewares.NewAdminMiddleware,
    middlewares.NewAccessMiddleware,
    middlewares.NewHelloMiddleware,   // <-- add here
    middlewares.NewAvatarMiddleware,
    middlewares.NewJiadanMiddleware,
    middlewares.NewBridgeMiddleware,
    middlewares.NewOpenAIMiddleware,
)
```

> **Note:** Order matters — middlewares earlier in the list get first chance to handle each message.

**Key things available via `MiddlewareContext`:**

- `m.client` — the platform client (`GenericClient`)
- `m.redis` — Redis instance
- `m.cfg` — full app configuration
- `m.access` — access control service
- `m.cron` — cron scheduler
- `m.avatarStore` — shared avatar store
- `m.SendText(target, text)` — send a text message
- `m.SendImage(target, base64)` — send an image
- `m.SendPendingMessage(target)` — send a "loading" card, returns a `PendingSender` for in-place updates

### Adding a new OpenAI tool

Tools extend the AI assistant's capabilities via function calling. Each tool declares its schema and implements an `Execute` method.

1. Create a new file in `tooling/`, e.g. `tooling/greeting.go`:

```go
package tooling

import (
    "context"
    "fmt"

    "github.com/openai/openai-go"
)

type GreetingTool struct{}

func NewGreetingTool() *GreetingTool {
    return &GreetingTool{}
}

func (g *GreetingTool) Name() string { return "greet_user" }

func (g *GreetingTool) Definition() openai.FunctionDefinitionParam {
    return openai.FunctionDefinitionParam{
        Name:        "greet_user",
        Description: openai.String("Generate a greeting for the user"),
        Parameters: openai.FunctionParameters{
            "type": "object",
            "properties": map[string]any{
                "name": map[string]string{
                    "type":        "string",
                    "description": "The name to greet",
                },
            },
            "required": []string{"name"},
        },
    }
}

func (g *GreetingTool) Execute(ctx context.Context, argsJSON string) (*ToolResult, error) {
    args, err := ParseArgs[struct{ Name string `json:"name"` }](argsJSON)
    if err != nil {
        return nil, err
    }
    return NewToolResult(fmt.Sprintf("Hello, %s!", args.Name)), nil
}
```

2. Register it in `middlewares/openai.go`:

```go
registry := tooling.NewRegistry()
registry.Register(tooling.NewWeatherTool(service.NewWeatherService(&base.cfg.Weather)))
registry.Register(tooling.NewGreetingTool())  // <-- add here
```

**`ToolResult` capabilities:**

- `NewToolResult(text)` — text summary sent back to the LLM
- `.AddText(markdown)` — append markdown to the response card
- `.AddImage(base64, altText)` — append an image to the response card
- Use `GetTarget(ctx)` to get the current chat target ID

