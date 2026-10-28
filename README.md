# Local AI Provider

A fully in-memory, configurable **local AI provider** (OpenAI & Vertex AI compatible) for development and testing of AI-powered applications and agentic workflows.

Supports:

- OpenAI Chat Completions API (`POST /v1/chat/completions`) with sync & SSE streaming
- OpenAI Responses API (`POST /v1/responses`) with sync & SSE streaming
- Vertex AI / Gemini GenerateContent API (`POST /v1beta/models/{model}:generateContent` & `:streamGenerateContent`)
- OpenAI Models API (`GET /v1/models`, `GET /v1/models/{model}`)
- Rule-based request matchers (exact, substring, regex, role, system prompt, tools, tool outputs)
- Deterministic, sequential, or random fixed text responses (**no actual AI calls**)
- Tool / function calling simulation (both synchronous and streamed deltas)
- Deep testing inspection, request history recording, and runtime state management API
- Dockerized and architecture-portable (x86_64 and arm64)

## 🚀 Purpose

This project simulates a **realistic AI provider** (like OpenAI or Google Vertex AI / Gemini) that you can run locally without any external dependencies, API keys, or GPU resources. No actual AI inference occurs—instead, requests are matched against configurable rules and return deterministic or randomized fixed responses and tool calls. It's perfect for:

- Testing AI SDKs, LLM clients, and multi-turn conversational pipelines
- Simulating function calling / tool execution loops deterministically
- Testing SSE streaming parsers, artificial latency, and rate-limit / error handling
- Developing or debugging AI integrations **without needing cloud API keys or spending tokens**
- Running fast, reproducible E2E tests in CI/CD or air-gapped environments

All state is **in-memory** and defined via a simple YAML config file or managed dynamically at runtime via the Testing API.

## 🐳 Docker

Pre-built images available at:

```
docker pull siocode/local-ai-provider
```

Available for:
- `linux/amd64` (x86 64-bit)
- `linux/arm64` (ARM 64-bit)

## 🛠️ Configuration

YAML configuration is passed via `-c` / `--config-path` flag, `CONFIG_PATH` env var, or defaults to `/config.yaml`.

See [`CONFIG.md`](./CONFIG.md) for full details.

### Example: `example/local-ai-provider.config.yaml`

```yaml
port: 8080
default_response: "I am a local AI provider default response."
selection_strategy: "sequential"

models:
  - id: "gpt-4o"
    owned_by: "openai"
    created: 1715367049
  - id: "gpt-4o-mini"
    owned_by: "openai"
    created: 1721172741
  - id: "gemini-1.5-pro"
    owned_by: "google"
    created: 1715000000

matchers:
  - id: "weather-tool-trigger"
    priority: 100
    match:
      model: "gpt-4o"
      message_contains: "weather in San Francisco"
      has_tools: true
      has_tool_output: false
    response:
      tool_calls:
        - id: "call_weather_1"
          name: "get_weather"
          arguments: '{"location":"San Francisco, CA","unit":"celsius"}'

  - id: "weather-tool-followup"
    priority: 110
    match:
      model: "gpt-4o"
      has_tool_output: true
      tool_output_contains: "18"
    response:
      text: "The current weather in San Francisco is 18°C and sunny."

  - id: "greeting-rule"
    priority: 50
    match:
      message_regex: "(?i)^(hello|hi|hey)\\b"
    response:
      selection_strategy: "sequential"
      responses:
        - "Hello! How can I assist you today?"
        - "Hi there! What can I help you build?"
```

## 🔐 Endpoints

[📄 See `API.md` for a complete reference of the API endpoints](./API.md)

### 🔧 Health & Metadata

| Method | Path       | Description  |
| ------ | ---------- | ------------ |
| GET    | `/healthz` | Health check |

### 🤖 Models API

| Method | Path                 | Description                |
| ------ | -------------------- | -------------------------- |
| GET    | `/v1/models`         | List all configured models |
| GET    | `/v1/models/{model}` | Get a specific model by ID |

### 💬 Conversational & Responses APIs

| Method | Path                                                 | Description                                            |
| ------ | ---------------------------------------------------- | ------------------------------------------------------ |
| POST   | `/v1/chat/completions`                               | OpenAI Chat Completions API (sync & SSE streaming)     |
| POST   | `/v1/responses`                                      | OpenAI Responses API (sync & SSE streaming)            |
| POST   | `/v1beta/models/{model}:generateContent`             | Vertex AI / Gemini GenerateContent API (sync)          |
| POST   | `/v1beta/models/{model}:streamGenerateContent`       | Vertex AI / Gemini StreamGenerateContent API (SSE/JSON)|

### 🧪 Testing, Inspection & Runtime Management API

| Method | Path                      | Description                                                |
| ------ | ------------------------- | ---------------------------------------------------------- |
| GET    | `/testing/requests`       | List all recorded AI requests with matched rules & outputs |
| GET    | `/testing/requests/{id}`  | Get a specific recorded request by ID                      |
| DELETE | `/testing/requests`       | Clear recorded request history                             |
| GET    | `/testing/matchers`       | List all active request matchers (ordered by priority)     |
| GET    | `/testing/matchers/{id}`  | Get a specific matcher by ID                               |
| PUT    | `/testing/matchers/{id}`  | Create or update a request matcher at runtime              |
| DELETE | `/testing/matchers/{id}`  | Delete a request matcher by ID                             |
| PUT    | `/testing/models/{id}`    | Create or update a model at runtime                        |
| DELETE | `/testing/models/{id}`    | Delete a model by ID                                       |
| GET    | `/testing/config`         | Get current runtime configuration & testing parameters     |
| PATCH  | `/testing/config`         | Update runtime testing parameters (delays, errors, toggles)|
| POST   | `/testing/reset`          | Reset all models, matchers, config, and state to initial   |

## 🧩 Request Matching & Deterministic Responses

Incoming requests across `/v1/chat/completions`, `/v1/responses`, and `/v1beta/models/{model}:*` are normalized into a unified internal representation containing:
- `model`: Target model ID
- `system_prompt`: Combined system/developer instructions
- `messages`: Conversation history (`role`, `content`, `tool_calls`, `tool_call_id`)
- `last_user_message`: Content of the most recent user message/input
- `tools`: Declared function/tool definitions
- `tool_outputs`: Outputs from previous tool calls (`role: "tool"` messages or `function_call_output` items)

Matchers are evaluated in descending `priority` order (ties broken by declaration order). The first matcher whose criteria all match determines the response:
- **Fixed or Multi-Choice Text**: Return `text` or select from `responses` using `"sequential"` (round-robin), `"random"`, or `"first"`.
- **Tool Calls**: Return structured `tool_calls` (formatted automatically as `choices[0].message.tool_calls` for Chat Completions, `output` `function_call` items for Responses API, or `functionCall` parts for Vertex AI).
- **Simulated Errors & Delays**: Optionally delay responses (`delay_ms`), throttle streaming chunks (`stream_chunk_delay_ms`), or return simulated HTTP error codes (`status_code`, `error_message`, `error_type`).

## 🧪 Example Docker Compose

### `example/docker-compose.yaml`

```yaml
services:
  local-ai-provider:
    image: siocode/local-ai-provider
    ports:
      - "8080:8080"
    volumes:
      - ./local-ai-provider.config.yaml:/config.yaml:ro
```

Start it:

```bash
cd example
docker compose up
```

## 🏗️ Building

Build the container via the following command (builds for both X86 & ARM architectures):

```bash
docker buildx build --platform=linux/arm64,linux/amd64 -t siocode/local-ai-provider:latest .
```

## 🧪 Testing

End-to-end tests are in the `e2e/` folder using Mocha + Chai + Docker Compose. To run:

```bash
cd e2e
npm install
npm test
```

## 📜 License

MIT © 2026 SIOCODE
