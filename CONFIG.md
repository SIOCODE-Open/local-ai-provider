# Configuration Reference – SIOCODE Local AI Provider

This document describes all configuration options for the SIOCODE Local AI Provider.

---

## Configuration File

The local AI provider is configured via a YAML file. By default, the server looks for `/config.yaml`, but you can specify a custom path using:

1. Command-line flag: `-c` or `--config-path`
2. Environment variable: `CONFIG_PATH`

### Configuration File Path

- **Command-Line Flag**: `-c <path>` or `--config-path <path>`
- **Environment Variable**: `CONFIG_PATH`
- **Default**: `/config.yaml`
- **Example**:
  - `local-ai-provider -c /app/my-config.yaml`
  - `local-ai-provider --config-path /app/my-config.yaml`
  - `CONFIG_PATH=/app/my-config.yaml local-ai-provider`

---

## Configuration Structure

### YAML Format

```yaml
port: 8080
allowed_origins: "*"
default_response: "I am a local AI provider default response."
selection_strategy: "sequential"
delay_ms: 0
stream_chunk_delay_ms: 0
stream_chunk_size: 20

features:
  chat_completions_enabled: true
  responses_enabled: true
  vertex_enabled: true
  strict_models: false
  require_auth: false

simulated_error:
  enabled: false
  status_code: 500
  message: ""
  type: "api_error"
  code: ""

models:
  - id: "gpt-4o"
    owned_by: "openai"
    created: 1715367049
  - id: "gemini-1.5-pro"
    owned_by: "google"
    created: 1715000000

matchers:
  - id: "greeting-rule"
    priority: 50
    disabled: false
    match:
      message_regex: "(?i)^(hello|hi|hey)\\b"
    response:
      selection_strategy: "sequential"
      responses:
        - "Hello! How can I assist you today?"
        - "Hi there! What can I help you build?"
```

---

## Configuration Options

### `port` (integer, optional)

The port on which the server will listen.

- **Type**: Integer
- **Default**: `8080`
- **Environment Variable Override**: `PORT`
- **Example**: `port: 8080`

If the `PORT` environment variable is set, it will override the value in the configuration file.

---

### `allowed_origins` (string, optional)

The allowed origins for Cross-Origin Resource Sharing (CORS) requests.

- **Type**: String
- **Default**: `*` (all origins allowed)
- **Example**: `allowed_origins: http://localhost:3000`

This value configures CORS headers for cross-origin requests. The server will:
- Set the `Access-Control-Allow-Origin` header to the configured value
- Handle preflight `OPTIONS` requests automatically
- Allow credentials and standard AI SDK headers (`Authorization`, `Content-Type`, `x-goog-api-key`, `OpenAI-Beta`, `OpenAI-Organization`, `OpenAI-Project`) for cross-origin requests

Common configurations:
- `*` - Allow all origins (default, convenient for local testing)
- `http://localhost:3000` - Allow a single specific origin
- `http://localhost:3000,https://example.com` - Allow multiple specific origins

---

### `default_response` (string, optional)

The fallback text response returned when no active matcher rule matches an incoming generation request and `default_responses` is empty.

- **Type**: String
- **Default**: `"I am a local AI provider default response."`
- **Example**: `default_response: "I am a local AI provider default response."`

---

### `default_responses` (array of strings, optional)

A list of fallback text responses used when no active matcher rule matches an incoming generation request. When non-empty, `default_responses` takes precedence over `default_response` and selects a response according to `selection_strategy`.

- **Type**: Array of strings
- **Default**: `[]` (empty array)
- **Example**:
  ```yaml
  default_responses:
    - "Default fallback response #1"
    - "Default fallback response #2"
  ```

---

### `selection_strategy` (string, optional)

The default strategy used to pick a response from `default_responses` (and from a matcher's `response.responses` when the matcher does not specify its own `selection_strategy`).

- **Type**: String
- **Default**: `"sequential"`
- **Allowed Values**:
  - `"sequential"` - Cycles through the candidate responses in round-robin order (`0, 1, ..., n-1, 0, ...`)
  - `"random"` - Selects a random candidate response on each request
  - `"first"` - Always selects the first candidate response (`responses[0]`)
- **Example**: `selection_strategy: "sequential"`

---

### `delay_ms` (integer, optional)

Global artificial delay in milliseconds applied before returning any generation response (both synchronous and streaming). Individual matchers can override this value via `response.delay_ms`.

- **Type**: Integer
- **Default**: `0` (no delay)
- **Example**: `delay_ms: 100`

---

### `stream_chunk_delay_ms` (integer, optional)

Global artificial delay in milliseconds between consecutive Server-Sent Events (SSE) chunks when streaming responses (`stream: true` or `:streamGenerateContent`). Individual matchers can override this value via `response.stream_chunk_delay_ms`.

- **Type**: Integer
- **Default**: `0` (no delay between chunks)
- **Example**: `stream_chunk_delay_ms: 15`

---

### `stream_chunk_size` (integer, optional)

Global number of characters (runes) emitted per streamed text delta chunk when streaming responses. Individual matchers can override this value via `response.stream_chunk_size`.

- **Type**: Integer
- **Default**: `20`
- **Example**: `stream_chunk_size: 20`

---

### `simulated_error` (object, optional)

Global error simulation configuration. When `enabled: true`, all generation endpoints (`/v1/chat/completions`, `/v1/responses`, and Vertex AI `generateContent` / `streamGenerateContent`) return a simulated HTTP error response formatted for the target API (OpenAI error envelope or Google Cloud ` GRPC / REST` status error envelope), unless a matched rule specifies its own error status code (`status_code >= 400`).

- **Type**: Object
- **Default**: `{ enabled: false, status_code: 500, message: "", type: "api_error", code: "" }`

#### SimulatedError Object Properties

##### `enabled` (boolean, optional)

Whether global error simulation is enabled.

- **Type**: Boolean
- **Default**: `false`
- **Example**: `enabled: true`

##### `status_code` (integer, optional)

The HTTP status code to return when simulated error is triggered (e.g., `429` for rate limiting, `500` for server error, `503` for service unavailable).

- **Type**: Integer
- **Default**: `500`
- **Example**: `status_code: 429`

##### `message` (string, optional)

The error message returned in the error response body.

- **Type**: String
- **Default**: `"Simulated error"` (when empty)
- **Example**: `message: "Rate limit exceeded, please retry later."`

##### `type` (string, optional)

The error type returned in the error response body.

- **Type**: String
- **Default**: `"api_error"`
- **Example**: `type: "rate_limit_error"`

##### `code` (string, optional)

The error code returned in the error response body.

- **Type**: String
- **Default**: `""`
- **Example**: `code: "rate_limit_exceeded"`

#### SimulatedError Example

```yaml
simulated_error:
  enabled: false
  status_code: 429
  message: "Rate limit reached for model"
  type: "rate_limit_error"
  code: "rate_limit_exceeded"
```

---

### `features` (object, optional)

Feature toggles and validation settings for the provider.

- **Type**: Object
- **Default**:
  ```yaml
  features:
    chat_completions_enabled: true
    responses_enabled: true
    vertex_enabled: true
    strict_models: false
    require_auth: false
  ```

#### Features Object Properties

##### `chat_completions_enabled` (boolean, optional)

Whether the OpenAI Chat Completions endpoint (`POST /v1/chat/completions`) is enabled. When `false`, requests to this endpoint return `404 Not Found`.

- **Type**: Boolean
- **Default**: `true`
- **Example**: `chat_completions_enabled: true`

##### `responses_enabled` (boolean, optional)

Whether the OpenAI Responses endpoint (`POST /v1/responses`) is enabled. When `false`, requests to this endpoint return `404 Not Found`.

- **Type**: Boolean
- **Default**: `true`
- **Example**: `responses_enabled: true`

##### `vertex_enabled` (boolean, optional)

Whether the Vertex AI / Gemini endpoints (`:generateContent` and `:streamGenerateContent`) are enabled. When `false`, requests to these endpoints return `404 Not Found`.

- **Type**: Boolean
- **Default**: `true`
- **Example**: `vertex_enabled: true`

##### `strict_models` (boolean, optional)

Whether generation endpoints require the requested `model` to exist in the configured `models` list.
- When `false` (default), generation requests succeed with any model name even if it is not listed in `models`.
- When `true`, generation requests specifying an unconfigured model return `404 Not Found` (`model_not_found`).

- **Type**: Boolean
- **Default**: `false`
- **Example**: `strict_models: false`

##### `require_auth` (boolean, optional)

Whether requests to API endpoints (`/v1/*` and Vertex AI endpoints) must include authentication credentials.
- For OpenAI endpoints (`/v1/*`), requires an `Authorization: Bearer <token>` header with a non-empty token.
- For Vertex AI / Gemini endpoints, accepts `Authorization: Bearer <token>`, an `x-goog-api-key: <key>` header, or a `?key=<key>` query parameter.
- When `false` (default), authentication headers are not required.

- **Type**: Boolean
- **Default**: `false`
- **Example**: `require_auth: false`

#### Features Example

```yaml
features:
  chat_completions_enabled: true
  responses_enabled: true
  vertex_enabled: true
  strict_models: false
  require_auth: false
```

---

### `models` (array, optional)

An array of model objects exposed by `GET /v1/models` and `GET /v1/models/{model}` (and validated on generation requests when `features.strict_models: true`).

- **Type**: Array of Model objects
- **Default**: `[]` (empty array)

#### Model Object Properties

##### `id` (string, required)

Unique model identifier (e.g., `"gpt-4o"`, `"gpt-4o-mini"`, `"gemini-1.5-pro"`).

- **Type**: String
- **Required**: Yes
- **Example**: `id: "gpt-4o"`

##### `owned_by` (string, optional)

The organization or provider that owns the model.

- **Type**: String
- **Default**: `"local"`
- **Example**: `owned_by: "openai"`

##### `created` (integer, optional)

Unix timestamp (in seconds) of when the model was created.

- **Type**: Integer (int64)
- **Default**: `1715367049`
- **Example**: `created: 1715367049`

##### `object` (string, optional)

The object type, which is always normalized to `"model"`.

- **Type**: String
- **Default**: `"model"`
- **Example**: `object: "model"`

#### Models Example

```yaml
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
```

---

### `matchers` (array, optional)

An array of rule-based request matchers used to produce deterministic text responses, multi-turn tool calls, reasoning outputs, or simulated errors.

Matchers are evaluated in descending `priority` order (highest `priority` first; ties are broken by their order in the configuration file). The first active (`disabled: false`) matcher whose `match` criteria all succeed determines the response. If no matcher matches, the server falls back to `default_responses` / `default_response` with `matched_rule_id: "__default__"`.

- **Type**: Array of Matcher objects
- **Default**: `[]` (empty array)

#### Matcher Object Properties

##### `id` (string, optional)

Unique identifier for the matcher rule. Recorded in request history as `matched_rule_id` and used to manage the rule via `/testing/matchers/{id}`.

- **Type**: String
- **Default**: `"matcher_<index>"` (e.g., `"matcher_1"`, `"matcher_2"`)
- **Example**: `id: "weather-tool-trigger"`

##### `priority` (integer, optional)

Evaluation priority for the matcher. Matchers with higher `priority` values are evaluated before matchers with lower `priority` values.

- **Type**: Integer
- **Default**: `0`
- **Example**: `priority: 100`

##### `disabled` (boolean, optional)

When set to `true`, the matcher is ignored during request evaluation.

- **Type**: Boolean
- **Default**: `false`
- **Example**: `disabled: false`

##### `match` (object, optional)

Criteria that an incoming request must satisfy for this rule to match. All specified fields in `match` are combined with logical **AND** (every specified condition must hold). An empty `match: {}` object matches every request.

- **Type**: Object (`MatchCriteria`)

###### `match` Properties

| Property | Type | Description |
|----------|------|-------------|
| `api_type` | string | Matches the normalized API endpoint type (case-insensitive): `"chat_completions"`, `"responses"`, or `"vertex"`. |
| `model` | string | Exact match against the target `model` name (e.g., `"gpt-4o"`). |
| `model_prefix` | string | Prefix match against the target `model` name (e.g., `"gpt-4"` or `"gemini-"`). |
| `role` | string | Requires a message with the specified role (`"system"`, `"developer"`, `"user"`, `"assistant"`/`"model"`, or `"tool"`) and scopes `message_exact`, `message_contains`, and `message_regex` to the **last message of that role**. When `role` is omitted, `message_*` criteria inspect the last user message (or the last message in the conversation if no user message is present). |
| `message_exact` | string | Exact match against the target message text. |
| `message_contains` | string | Case-sensitive substring match against the target message text. |
| `message_regex` | string | Regular expression match (Go `regexp` syntax, supports `(?i)` for case-insensitive matching) against the target message text. |
| `any_message_contains` | string | Substring match against **any** message in the conversation history (including `system_prompt`). |
| `system_contains` | string | Substring match against the normalized system/developer instructions (`system`/`developer` messages in Chat Completions, `instructions` or `system`/`developer` input items in Responses API, or `systemInstruction` in Vertex AI). |
| `system_regex` | string | Regular expression match against the normalized system/developer instructions. |
| `has_tools` | boolean | When `true`, requires the request to declare at least one tool/function definition. When `false`, requires the request to declare zero tools. |
| `tool_name` | string | Requires a tool with this exact function name to be present in the request's declared `tools` list. |
| `tool_choice` | string | Case-insensitive match against the normalized `tool_choice` value (e.g., `"auto"`, `"none"`, `"required"`, `"any"`, or a specific function name). |
| `has_tool_output` | boolean | When `true`, requires the conversation to contain at least one tool execution output (`role: "tool"` message in Chat Completions, `function_call_output` item in Responses API, or `functionResponse` part in Vertex AI). When `false`, requires zero tool outputs (useful for distinguishing the initial tool-trigger turn from the follow-up turn). |
| `tool_call_id` | string | Requires a tool output matching this `tool_call_id` / `call_id` (or function `name` in Vertex AI), and scopes `tool_output_contains` and `tool_output_regex` to matching tool outputs. |
| `tool_output_contains` | string | Substring match against the content/payload of a tool output in the request. |
| `tool_output_regex` | string | Regular expression match against the content/payload of a tool output in the request. |
| `stream` | boolean | When `true`, matches only streaming requests (`stream: true` or `:streamGenerateContent`). When `false`, matches only synchronous non-streaming requests. |
| `header_match` | object (map of `string: string`) | Requires all specified HTTP request headers to match their expected values (header names are matched case-insensitively via `http.Header.Get`). |

##### `response` (object, required)

Defines the response returned when the matcher matches a request.

- **Type**: Object (`ResponseConfig`)

###### `response` Properties

| Property | Type | Description |
|----------|------|-------------|
| `text` | string | Fixed assistant text response to return. |
| `responses` | array of strings | List of candidate text responses. When non-empty, takes precedence over `text` and selects one response per request using `selection_strategy`. |
| `selection_strategy` | string | Strategy for selecting from `responses`: `"sequential"` (round-robin), `"random"`, or `"first"`. Defaults to the global `selection_strategy` (`"sequential"`). |
| `reasoning_content` | string | Optional reasoning/thought text to include alongside the response. |
| `tool_calls` | array of objects | Tool/function calls to return in the response (`id`, `name`, `arguments`). Formatted automatically for OpenAI Chat Completions (`choices[0].message.tool_calls`), OpenAI Responses API (`function_call` output items), and Vertex AI (`functionCall` parts). |
| `finish_reason` | string | Overrides the completion finish reason. Defaults to `"tool_calls"` when `tool_calls` is non-empty, or `"stop"` otherwise (mapped to `"STOP"` for Vertex AI). |
| `delay_ms` | integer | Overrides the global `delay_ms` for this matcher (artificial delay in milliseconds before responding). |
| `stream_chunk_delay_ms` | integer | Overrides the global `stream_chunk_delay_ms` for this matcher (delay in milliseconds between SSE chunks). |
| `stream_chunk_size` | integer | Overrides the global `stream_chunk_size` for this matcher (number of characters per streamed text chunk). |
| `status_code` | integer | HTTP status code to return (default: `200`). When set to `>= 400` (e.g., `400`, `401`, `429`, `500`), returns a simulated error response instead of a completion. |
| `error_message` | string | Error message used when `status_code >= 400` (defaults to `"Simulated error from matcher <id>"`). |
| `error_type` | string | Error type used when `status_code >= 400` (default: `"api_error"`). |
| `error_code` | string | Error code used when `status_code >= 400` (default: `""`). |

###### `tool_calls` Item Properties

Each item in `response.tool_calls` supports the following properties:

- **`id`** (string, optional): Unique tool call identifier (defaults to `"call_1"`, `"call_2"`, etc.).
- **`name`** (string, required): Name of the function to invoke (e.g., `"get_weather"`).
- **`arguments`** (string or YAML/JSON object, optional): JSON-encoded function arguments string (e.g., `'{"location":"San Francisco, CA"}'`) or an inline YAML map/object (which is automatically serialized to a JSON string; defaults to `"{}"`).

#### Matchers Example

```yaml
matchers:
  # Step 1 of a tool-calling loop: trigger a function call when tools are present
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
          arguments:
            location: "San Francisco, CA"
            unit: "celsius"

  # Step 2 of a tool-calling loop: respond to the tool output
  - id: "weather-tool-followup"
    priority: 110
    match:
      model: "gpt-4o"
      has_tool_output: true
      tool_call_id: "call_weather_1"
      tool_output_contains: "18"
    response:
      text: "The current weather in San Francisco is 18°C and sunny."

  # Multi-response rule using regex and sequential selection
  - id: "greeting-rule"
    priority: 50
    match:
      message_regex: "(?i)^(hello|hi|hey)\\b"
    response:
      selection_strategy: "sequential"
      responses:
        - "Hello! How can I assist you today?"
        - "Hi there! What can I help you build?"

  # Simulated error rule triggered via message content or custom header
  - id: "rate-limit-simulation"
    priority: 200
    match:
      header_match:
        X-Simulate-Rate-Limit: "true"
    response:
      status_code: 429
      error_message: "Rate limit exceeded for this test case"
      error_type: "rate_limit_error"
      error_code: "rate_limit_exceeded"
```

---

## Environment Variables

### `CONFIG_PATH`

Path to the YAML configuration file.

- **Default**: `/config.yaml`
- **Example**: `CONFIG_PATH=/app/config/ai-provider.yaml`

### `PORT`

Server port (overrides the `port` setting in the configuration file).

- **Default**: `8080` (if not specified in config file)
- **Example**: `PORT=9000`

---

## Complete Configuration Example

```yaml
# Server configuration
port: 8080
allowed_origins: "*"

# Fallback response settings when no matcher rule matches
default_response: "I am a local AI provider default response."
default_responses:
  - "I am a local AI provider default response."
  - "Here is an alternative default response."
selection_strategy: "sequential"

# Global latency & streaming chunk settings
delay_ms: 0
stream_chunk_delay_ms: 0
stream_chunk_size: 20

# Global error simulation (can also be toggled at runtime via PATCH /testing/config)
simulated_error:
  enabled: false
  status_code: 500
  message: "Simulated global server error"
  type: "api_error"
  code: "internal_error"

# Feature flags
features:
  chat_completions_enabled: true
  responses_enabled: true
  vertex_enabled: true
  strict_models: false
  require_auth: false

# Available AI models (returned by GET /v1/models)
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
  - id: "gemini-2.0-flash"
    owned_by: "google"
    created: 1733961600

# Request matchers (evaluated in descending priority order)
matchers:
  - id: "weather-tool-followup"
    priority: 110
    match:
      has_tool_output: true
      tool_output_contains: "18"
    response:
      text: "The current weather in San Francisco is 18°C and sunny."

  - id: "weather-tool-trigger"
    priority: 100
    match:
      message_contains: "weather in San Francisco"
      has_tools: true
      has_tool_output: false
    response:
      tool_calls:
        - id: "call_weather_1"
          name: "get_weather"
          arguments: '{"location":"San Francisco, CA","unit":"celsius"}'

  - id: "vertex-summary-rule"
    priority: 80
    match:
      api_type: "vertex"
      model_prefix: "gemini-"
      system_contains: "summarize"
    response:
      text: "Summary: The provided document outlines key testing strategies for local AI workflows."

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

---

## Docker Configuration

When running in Docker, mount your configuration file to `/config.yaml` or set the `CONFIG_PATH` environment variable:

```yaml
services:
  local-ai-provider:
    image: siocode/local-ai-provider
    ports:
      - "8080:8080"
    volumes:
      - ./my-config.yaml:/config.yaml:ro
    environment:
      - PORT=8080
```

Or with a custom path:

```yaml
services:
  local-ai-provider:
    image: siocode/local-ai-provider
    ports:
      - "9000:9000"
    volumes:
      - ./configs/ai-provider.yaml:/app/ai-provider.yaml:ro
    environment:
      - CONFIG_PATH=/app/ai-provider.yaml
      - PORT=9000
```

---

## Notes

- **All data is in-memory**: Models, matchers, sequential counters, runtime settings, and recorded requests are stored in memory and reset when the server restarts (or when `POST /testing/reset` is called).
- **No actual AI inference**: This provider does not run LLM weights or contact external AI services. All responses and tool calls are deterministically or randomly chosen from your configured rules.
- **Runtime reconfiguration**: You can inspect requests, add/update/delete matchers and models, toggle features, or inject delays and errors at runtime via the `/testing/*` endpoints without restarting the container. See [`API.md`](./API.md) for details.
- **Cross-API normalization**: A single matcher rule works seamlessly across OpenAI Chat Completions (`/v1/chat/completions`), OpenAI Responses (`/v1/responses`), and Google Vertex AI / Gemini (`:generateContent` and `:streamGenerateContent`), unless restricted via `match.api_type`.

---

## 📄 License

MIT © 2026 SIOCODE
