# API Reference – SIOCODE Local AI Provider

This document describes all API endpoints exposed by the SIOCODE Local AI Provider.

**Note:** Specific API families can be disabled via configuration (`features` in YAML or `PATCH /testing/config`):
- OpenAI Chat Completions API endpoints can be disabled by setting `features.chat_completions_enabled: false` (default: `true`)
- OpenAI Responses API endpoints can be disabled by setting `features.responses_enabled: false` (default: `true`)
- Vertex AI / Gemini API endpoints can be disabled by setting `features.vertex_enabled: false` (default: `true`)
- Authentication enforcement can be enabled by setting `features.require_auth: true` (default: `false`)
- Strict model existence validation on generation endpoints can be enabled by setting `features.strict_models: true` (default: `false`)

---

## 🧠 Health Check

### `GET /healthz`

Returns a health check response.

**Response:**
```json
{
  "status": "OK"
}
```

---

## 🤖 OpenAI Models API

### `GET /v1/models`

Lists the currently available models in OpenAI-compatible format.

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o",
      "object": "model",
      "created": 1715367049,
      "owned_by": "openai"
    },
    {
      "id": "gemini-1.5-pro",
      "object": "model",
      "created": 1715000000,
      "owned_by": "google"
    }
  ]
}
```

---

### `GET /v1/models/{model}`

Retrieves a model instance by ID, providing basic information about the model such as the owner and creation timestamp.

**Path Parameters:**

| Parameter | Type   | Description                           |
|-----------|--------|---------------------------------------|
| `model`   | string | The ID of the model to use for this request |

**Response:**
```json
{
  "id": "gpt-4o",
  "object": "model",
  "created": 1715367049,
  "owned_by": "openai"
}
```

**Errors:**

- `404 Not Found` - If the model does not exist:
  ```json
  {
    "error": {
      "message": "The model 'unknown-model' does not exist",
      "type": "invalid_request_error",
      "param": "model",
      "code": "model_not_found"
    }
  }
  ```

---

## 💬 OpenAI Chat Completions API

**Configuration:** Available when `features.chat_completions_enabled: true` (default).

### `POST /v1/chat/completions`

Creates a model response for the given chat conversation. Supports both synchronous JSON responses (`stream: false` or omitted) and Server-Sent Events streaming (`stream: true`).

**Content-Type:** `application/json`

**Request:**
```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "What is the weather in San Francisco?"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get the current weather in a given location",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "The city and state, e.g. San Francisco, CA"
            },
            "unit": {
              "type": "string",
              "enum": ["celsius", "fahrenheit"]
            }
          },
          "required": ["location"]
        },
        "strict": false
      }
    }
  ],
  "tool_choice": "auto",
  "stream": false,
  "temperature": 1,
  "max_completion_tokens": 256
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `model` | string | Yes | ID of the model to use (e.g. `"gpt-4o"`). |
| `messages` | array | Yes | A list of messages comprising the conversation so far. Each message object contains `role` (`"developer"`, `"system"`, `"user"`, `"assistant"`, or `"tool"`), `content` (string, `null`, or array of content parts such as `[{"type": "text", "text": "..."}]`), and optionally `name`, `tool_calls` (for `"assistant"` messages), or `tool_call_id` (for `"tool"` messages). |
| `tools` | array | No | A list of tools the model may call. Each item has `type: "function"` and `function: {"name": string, "description": string, "parameters": object, "strict": boolean}`. |
| `tool_choice` | string or object | No | Controls which (if any) tool is called by the model (`"none"`, `"auto"`, `"required"`, or `{"type": "function", "function": {"name": "my_function"}}`). |
| `parallel_tool_calls` | boolean | No | Whether to enable parallel function calling during tool use (default: `true`). |
| `stream` | boolean | No | If set to `true`, partial message deltas will be sent as SSE `data: <json>\n\n` events, terminating with `data: [DONE]\n\n` (default: `false`). |
| `stream_options` | object | No | Options for streaming response. When `{"include_usage": true}` is set, an additional chunk containing the `usage` object and `choices: []` is streamed right before `data: [DONE]\n\n`. |
| `temperature` | number | No | Sampling temperature parameter (accepted for compatibility). |
| `top_p` | number | No | Nucleus sampling parameter (accepted for compatibility). |
| `max_tokens` | integer | No | Maximum number of tokens to generate (accepted for compatibility). |
| `max_completion_tokens` | integer | No | Upper bound for generated completion tokens (accepted for compatibility). |
| `response_format` | object | No | Format that the model must output, e.g. `{"type": "text"}` or `{"type": "json_object"}` or `{"type": "json_schema", ...}` (accepted for compatibility). |
| `user` | string | No | Unique identifier representing your end-user. |

**Response (Synchronous Text Completion, `stream: false`):**
```json
{
  "id": "chatcmpl-1",
  "object": "chat.completion",
  "created": 1715367049,
  "model": "gpt-4o",
  "system_fingerprint": "fp_local_ai_provider",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I assist you today?",
        "refusal": null
      },
      "logprobs": null,
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 8,
    "total_tokens": 20,
    "prompt_tokens_details": {
      "cached_tokens": 0,
      "audio_tokens": 0
    },
    "completion_tokens_details": {
      "reasoning_tokens": 0,
      "audio_tokens": 0,
      "accepted_prediction_tokens": 0,
      "rejected_prediction_tokens": 0
    }
  }
}
```

**Response (Synchronous Tool Call Completion, `stream: false`):**

When the matched rule returns tool calls, `choices[0].message.tool_calls` is populated, `choices[0].message.content` is `null` (unless text is also configured on the matcher), and `choices[0].finish_reason` is `"tool_calls"`:

```json
{
  "id": "chatcmpl-2",
  "object": "chat.completion",
  "created": 1715367050,
  "model": "gpt-4o",
  "system_fingerprint": "fp_local_ai_provider",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": null,
        "refusal": null,
        "tool_calls": [
          {
            "id": "call_weather_1",
            "type": "function",
            "function": {
              "name": "get_weather",
              "arguments": "{\"location\":\"San Francisco, CA\",\"unit\":\"celsius\"}"
            }
          }
        ]
      },
      "logprobs": null,
      "finish_reason": "tool_calls"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 15,
    "total_tokens": 40,
    "prompt_tokens_details": {
      "cached_tokens": 0,
      "audio_tokens": 0
    },
    "completion_tokens_details": {
      "reasoning_tokens": 0,
      "audio_tokens": 0,
      "accepted_prediction_tokens": 0,
      "rejected_prediction_tokens": 0
    }
  }
}
```

**Response (Streaming SSE, `stream: true`):**

- **Headers:** `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`
- **Format:** Standard SSE lines `data: <json>\n\n` where each JSON object has `"object": "chat.completion.chunk"`, followed by `data: [DONE]\n\n`.

*Streaming Text Example:*
```text
data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1715367049,"model":"gpt-4o","system_fingerprint":"fp_local_ai_provider","choices":[{"index":0,"delta":{"role":"assistant","content":"","refusal":null},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1715367049,"model":"gpt-4o","system_fingerprint":"fp_local_ai_provider","choices":[{"index":0,"delta":{"content":"Hello! How can I a"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1715367049,"model":"gpt-4o","system_fingerprint":"fp_local_ai_provider","choices":[{"index":0,"delta":{"content":"ssist you today?"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1715367049,"model":"gpt-4o","system_fingerprint":"fp_local_ai_provider","choices":[{"index":0,"delta":{},"logprobs":null,"finish_reason":"stop"}]}

data: [DONE]

```

*Streaming Tool Call Example:*
```text
data: {"id":"chatcmpl-2","object":"chat.completion.chunk","created":1715367050,"model":"gpt-4o","system_fingerprint":"fp_local_ai_provider","choices":[{"index":0,"delta":{"role":"assistant","content":null,"refusal":null},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-2","object":"chat.completion.chunk","created":1715367050,"model":"gpt-4o","system_fingerprint":"fp_local_ai_provider","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_weather_1","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"San Francisco, CA\",\"unit\":\"celsius\"}"}}]},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-2","object":"chat.completion.chunk","created":1715367050,"model":"gpt-4o","system_fingerprint":"fp_local_ai_provider","choices":[{"index":0,"delta":{},"logprobs":null,"finish_reason":"tool_calls"}]}

data: [DONE]

```

*(If `stream_options.include_usage: true` was passed in the request, an extra chunk with `"choices": []` and `"usage": {...}` is emitted immediately before `data: [DONE]`).*

**Errors:**

- `400 Bad Request` - If `model` is missing, `messages` is empty/missing, or JSON is malformed
- `401 Unauthorized` - If `features.require_auth: true` and `Authorization: Bearer <token>` is missing
- `404 Not Found` - If `features.chat_completions_enabled: false` or `features.strict_models: true` and `model` is unknown
- `429 Too Many Requests` / `500 Internal Server Error` - When simulated error is configured on the matched rule or globally

---

## ⚡ OpenAI Responses API

**Configuration:** Available when `features.responses_enabled: true` (default).

### `POST /v1/responses`

Creates a model response using the OpenAI Responses API. Supports plain text or structured array `input` items, function calling, and semantic Server-Sent Events streaming (`stream: true`).

**Content-Type:** `application/json`

**Request (String Input):**
```json
{
  "model": "gpt-4o",
  "instructions": "You are a helpful assistant.",
  "input": "Hello! How are you?",
  "stream": false
}
```

**Request (Multi-Turn Array Input with Tools and `function_call_output`):**
```json
{
  "model": "gpt-4o",
  "instructions": "You are a helpful weather assistant.",
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "What is the weather in San Francisco?"
        }
      ]
    },
    {
      "type": "function_call",
      "id": "fc_1",
      "call_id": "call_weather_1",
      "name": "get_weather",
      "arguments": "{\"location\":\"San Francisco, CA\",\"unit\":\"celsius\"}"
    },
    {
      "type": "function_call_output",
      "call_id": "call_weather_1",
      "output": "{\"temperature\": 18, \"condition\": \"sunny\"}"
    }
  ],
  "tools": [
    {
      "type": "function",
      "name": "get_weather",
      "description": "Get the current weather in a given location",
      "parameters": {
        "type": "object",
        "properties": {
          "location": { "type": "string" },
          "unit": { "type": "string", "enum": ["celsius", "fahrenheit"] }
        },
        "required": ["location"]
      },
      "strict": false
    }
  ],
  "tool_choice": "auto",
  "parallel_tool_calls": true,
  "stream": false
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `model` | string | Yes | Model ID used to generate the response (e.g. `"gpt-4o"`). |
| `input` | string or array | Yes | Text, image, or file inputs to the model, used to generate a response. Can be a single string or an array of input items:<ul><li>Message item: `{"type": "message", "role": "user" \| "system" \| "developer" \| "assistant", "content": string \| [{"type": "input_text" \| "output_text" \| "text", "text": "..."}]}` (`"type": "message"` may be omitted when `role` and `content` are provided)</li><li>Function call item: `{"type": "function_call", "id": "fc_...", "call_id": "call_...", "name": "...", "arguments": "..."}`</li><li>Function call output item: `{"type": "function_call_output", "call_id": "call_...", "output": "..."}`</li></ul> |
| `instructions` | string | No | Inserts a system (or developer) message as the first item in the model's context. |
| `tools` | array | No | An array of tools the model may call while generating a response. Supports the standard Responses API flat function format `{"type": "function", "name": "...", "description": "...", "parameters": {...}, "strict": false}` as well as the nested `{"type": "function", "function": {"name": "...", ...}}` format. |
| `tool_choice` | string or object | No | How the model should select which tool (or tools) to use (`"none"`, `"auto"`, `"required"`, or `{"type": "function", "name": "..."}`). |
| `parallel_tool_calls` | boolean | No | Whether to allow the model to run tool calls in parallel (default: `true`). |
| `previous_response_id` | string | No | The unique ID of the previous response to the model for multi-turn conversations. |
| `stream` | boolean | No | If set to `true`, the model response data will be streamed to the client using Server-Sent Events (`text/event-stream`). Default: `false`. |
| `temperature` | number | No | Sampling temperature to use, between 0 and 2 (default: `1.0`). |
| `top_p` | number | No | Nucleus sampling parameter (default: `1.0`). |
| `max_output_tokens` | integer | No | An upper bound for the number of tokens that can be generated for a response. |
| `store` | boolean | No | Whether to store the generated model response for later retrieval (default: `true`). |
| `metadata` | object | No | Set of key-value pairs that can be attached to an object (default: `{}`). |
| `text` | object | No | Configuration options for a text response from the model, e.g. `{"format": {"type": "text"}}`. |
| `truncation` | string | No | The truncation strategy to use for the model response (`"disabled"` or `"auto"`, default: `"disabled"`). |
| `user` | string | No | A unique identifier representing your end-user. |

**Response (Synchronous Text Output, `stream: false`):**
```json
{
  "id": "resp_1",
  "object": "response",
  "created_at": 1715367049,
  "status": "completed",
  "error": null,
  "incomplete_details": null,
  "instructions": "You are a helpful assistant.",
  "max_output_tokens": null,
  "model": "gpt-4o",
  "output": [
    {
      "type": "message",
      "id": "msg_1",
      "status": "completed",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "Hello! How can I assist you today?",
          "annotations": []
        }
      ]
    }
  ],
  "output_text": "Hello! How can I assist you today?",
  "parallel_tool_calls": true,
  "previous_response_id": null,
  "reasoning": {
    "effort": null,
    "summary": null
  },
  "store": true,
  "temperature": 1.0,
  "text": {
    "format": {
      "type": "text"
    }
  },
  "tool_choice": "auto",
  "tools": [],
  "top_p": 1.0,
  "truncation": "disabled",
  "usage": {
    "input_tokens": 12,
    "input_tokens_details": {
      "cached_tokens": 0
    },
    "output_tokens": 8,
    "output_tokens_details": {
      "reasoning_tokens": 0
    },
    "total_tokens": 20
  },
  "user": null,
  "metadata": {}
}
```

**Response (Synchronous Function Call Output, `stream: false`):**

When the matched rule returns tool calls, `output` contains items of `type: "function_call"`:

```json
{
  "id": "resp_2",
  "object": "response",
  "created_at": 1715367050,
  "status": "completed",
  "error": null,
  "incomplete_details": null,
  "instructions": "You are a helpful weather assistant.",
  "max_output_tokens": null,
  "model": "gpt-4o",
  "output": [
    {
      "type": "function_call",
      "id": "fc_1",
      "call_id": "call_weather_1",
      "name": "get_weather",
      "arguments": "{\"location\":\"San Francisco, CA\",\"unit\":\"celsius\"}",
      "status": "completed"
    }
  ],
  "output_text": "",
  "parallel_tool_calls": true,
  "previous_response_id": null,
  "reasoning": {
    "effort": null,
    "summary": null
  },
  "store": true,
  "temperature": 1.0,
  "text": {
    "format": {
      "type": "text"
    }
  },
  "tool_choice": "auto",
  "tools": [
    {
      "type": "function",
      "name": "get_weather",
      "description": "Get the current weather in a given location",
      "parameters": {
        "type": "object",
        "properties": {
          "location": { "type": "string" },
          "unit": { "type": "string", "enum": ["celsius", "fahrenheit"] }
        },
        "required": ["location"]
      },
      "strict": false
    }
  ],
  "top_p": 1.0,
  "truncation": "disabled",
  "usage": {
    "input_tokens": 25,
    "input_tokens_details": {
      "cached_tokens": 0
    },
    "output_tokens": 15,
    "output_tokens_details": {
      "reasoning_tokens": 0
    },
    "total_tokens": 40
  },
  "user": null,
  "metadata": {}
}
```

**Response (Streaming SSE, `stream: true`):**

- **Headers:** `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`
- **Format:** Semantic Server-Sent Events formatted as `event: <type>\ndata: <json>\n\n`, ending with `data: [DONE]\n\n`.

*Streaming Text Event Sequence:*
1. `event: response.created` — Emitted when the response is created (`status: "in_progress"`, `output: []`).
2. `event: response.in_progress` — Emitted when the response moves to `"in_progress"`.
3. `event: response.output_item.added` — Emitted when the assistant message item is initialized (`item: {"id": "msg_1", "type": "message", "status": "in_progress", "role": "assistant", "content": []}`).
4. `event: response.content_part.added` — Emitted when the `output_text` part is initialized (`part: {"type": "output_text", "text": "", "annotations": []}`).
5. `event: response.output_text.delta` — Emitted for each text chunk (`delta: "<chunk>"`).
6. `event: response.output_text.done` — Emitted when text generation finishes (`text: "<full_text>"`).
7. `event: response.content_part.done` — Emitted when the `output_text` part finishes (`part: {"type": "output_text", "text": "<full_text>", "annotations": []}`).
8. `event: response.output_item.done` — Emitted when the message output item finishes (`status: "completed"`).
9. `event: response.completed` — Emitted with the complete final `response` object (including `output` and `usage`).
10. `data: [DONE]`

```text
event: response.created
data: {"type":"response.created","sequence_number":0,"response":{"id":"resp_1","object":"response","created_at":1715367049,"status":"in_progress","error":null,"incomplete_details":null,"model":"gpt-4o","output":[]}}

event: response.in_progress
data: {"type":"response.in_progress","sequence_number":1,"response":{"id":"resp_1","object":"response","created_at":1715367049,"status":"in_progress","error":null,"incomplete_details":null,"model":"gpt-4o","output":[]}}

event: response.output_item.added
data: {"type":"response.output_item.added","sequence_number":2,"output_index":0,"item":{"id":"msg_1","type":"message","status":"in_progress","role":"assistant","content":[]}}

event: response.content_part.added
data: {"type":"response.content_part.added","sequence_number":3,"item_id":"msg_1","output_index":0,"content_index":0,"part":{"type":"output_text","text":"","annotations":[]}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","sequence_number":4,"item_id":"msg_1","output_index":0,"content_index":0,"delta":"Hello! How can I a"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","sequence_number":5,"item_id":"msg_1","output_index":0,"content_index":0,"delta":"ssist you today?"}

event: response.output_text.done
data: {"type":"response.output_text.done","sequence_number":6,"item_id":"msg_1","output_index":0,"content_index":0,"text":"Hello! How can I assist you today?"}

event: response.content_part.done
data: {"type":"response.content_part.done","sequence_number":7,"item_id":"msg_1","output_index":0,"content_index":0,"part":{"type":"output_text","text":"Hello! How can I assist you today?","annotations":[]}}

event: response.output_item.done
data: {"type":"response.output_item.done","sequence_number":8,"output_index":0,"item":{"id":"msg_1","type":"message","status":"completed","role":"assistant","content":[{"type":"output_text","text":"Hello! How can I assist you today?","annotations":[]}]}}

event: response.completed
data: {"type":"response.completed","sequence_number":9,"response":{"id":"resp_1","object":"response","created_at":1715367049,"status":"completed","error":null,"incomplete_details":null,"model":"gpt-4o","output":[{"type":"message","id":"msg_1","status":"completed","role":"assistant","content":[{"type":"output_text","text":"Hello! How can I assist you today?","annotations":[]}]}],"output_text":"Hello! How can I assist you today?","usage":{"input_tokens":12,"input_tokens_details":{"cached_tokens":0},"output_tokens":8,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":20}}}

data: [DONE]

```

*Streaming Function Call Event Sequence:*
1. `event: response.created`
2. `event: response.in_progress`
3. `event: response.output_item.added` (`item: {"id": "fc_1", "type": "function_call", "status": "in_progress", "call_id": "call_weather_1", "name": "get_weather", "arguments": ""}`)
4. `event: response.function_call_arguments.delta` (`{"type": "response.function_call_arguments.delta", "sequence_number": 3, "item_id": "fc_1", "output_index": 0, "call_id": "call_weather_1", "name": "get_weather", "delta": "{\"location\":\"San Francisco, CA\",\"unit\":\"celsius\"}"}`)
5. `event: response.function_call_arguments.done` (`{"type": "response.function_call_arguments.done", "sequence_number": 4, "item_id": "fc_1", "output_index": 0, "call_id": "call_weather_1", "name": "get_weather", "arguments": "{\"location\":\"San Francisco, CA\",\"unit\":\"celsius\"}"}`)
6. `event: response.output_item.done` (`item: {"id": "fc_1", "type": "function_call", "status": "completed", "call_id": "call_weather_1", "name": "get_weather", "arguments": "{\"location\":\"San Francisco, CA\",\"unit\":\"celsius\"}"}`)
7. `event: response.completed`
8. `data: [DONE]`

**Errors:**

- `400 Bad Request` - If `model` or `input` is missing, or JSON is malformed
- `401 Unauthorized` - If `features.require_auth: true` and `Authorization: Bearer <token>` is missing
- `404 Not Found` - If `features.responses_enabled: false` or `features.strict_models: true` and `model` is unknown
- `429 Too Many Requests` / `500 Internal Server Error` - When simulated error is configured on the matched rule or globally

---

## 🌐 Vertex AI / Gemini GenerateContent API

**Configuration:** Available when `features.vertex_enabled: true` (default).

Both Google AI Studio (`v1beta/models/{model}:*`) and Google Cloud Vertex AI (`v1/projects/{project}/locations/{location}/publishers/google/models/{model}:*`) URL paths are supported.

### `POST /v1beta/models/{model}:generateContent`
### `POST /v1/projects/{project}/locations/{location}/publishers/google/models/{model}:generateContent`

Generates a synchronous model response in Vertex AI / Gemini `GenerateContentResponse` format.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `model` | string | The model identifier (e.g. `"gemini-1.5-pro"`, `"gemini-2.0-flash"`) |
| `project` | string | Google Cloud project ID (for Vertex AI path) |
| `location` | string | Google Cloud region (for Vertex AI path, e.g. `"us-central1"`) |

**Content-Type:** `application/json`

**Request:**
```json
{
  "systemInstruction": {
    "role": "system",
    "parts": [
      {
        "text": "You are a helpful weather assistant."
      }
    ]
  },
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "What is the weather in San Francisco?"
        }
      ]
    },
    {
      "role": "model",
      "parts": [
        {
          "functionCall": {
            "name": "get_weather",
            "args": {
              "location": "San Francisco, CA"
            }
          }
        }
      ]
    },
    {
      "role": "user",
      "parts": [
        {
          "functionResponse": {
            "name": "get_weather",
            "response": {
              "temperature": 18,
              "condition": "sunny"
            }
          }
        }
      ]
    }
  ],
  "tools": [
    {
      "functionDeclarations": [
        {
          "name": "get_weather",
          "description": "Get the current weather for a location",
          "parameters": {
            "type": "OBJECT",
            "properties": {
              "location": {
                "type": "STRING"
              }
            },
            "required": ["location"]
          }
        }
      ]
    }
  ],
  "toolConfig": {
    "functionCallingConfig": {
      "mode": "AUTO"
    }
  },
  "generationConfig": {
    "temperature": 0.7,
    "maxOutputTokens": 256
  }
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `contents` | array | Yes | The content of the current conversation with the model. Each `Content` item has `role` (`"user"` or `"model"`) and `parts` (array of `Part` objects containing `text`, `functionCall: {"name": string, "args": object}`, or `functionResponse: {"name": string, "response": object}`). |
| `systemInstruction` | object | No | Developer/system instruction (`{"parts": [{"text": "..."}]}`). Also accepts snake_case `system_instruction`. |
| `tools` | array | No | A list of `Tool` objects the model may use (`[{"functionDeclarations": [{"name": string, "description": string, "parameters": object}]}]`). |
| `toolConfig` | object | No | Tool configuration (`{"functionCallingConfig": {"mode": "AUTO" \| "ANY" \| "NONE", "allowedFunctionNames": [...]}}`). |
| `generationConfig` | object | No | Configuration options for model generation (`temperature`, `topP`, `topK`, `maxOutputTokens`, `stopSequences`, `responseMimeType`, `responseSchema`). |

**Response (Text Output):**
```json
{
  "candidates": [
    {
      "content": {
        "role": "model",
        "parts": [
          {
            "text": "The current weather in San Francisco is 18°C and sunny."
          }
        ]
      },
      "finishReason": "STOP",
      "index": 0
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 12,
    "candidatesTokenCount": 11,
    "totalTokenCount": 23
  },
  "modelVersion": "gemini-1.5-pro"
}
```

**Response (Function Call Output):**

When the matched rule returns tool calls, each tool call is placed in `candidates[0].content.parts` as a `functionCall` object with `name` and parsed JSON `args`:

```json
{
  "candidates": [
    {
      "content": {
        "role": "model",
        "parts": [
          {
            "functionCall": {
              "id": "call_weather_1",
              "name": "get_weather",
              "args": {
                "location": "San Francisco, CA",
                "unit": "celsius"
              }
            }
          }
        ]
      },
      "finishReason": "STOP",
      "index": 0
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 15,
    "candidatesTokenCount": 10,
    "totalTokenCount": 25
  },
  "modelVersion": "gemini-1.5-pro"
}
```

**Errors:**

- `400 Bad Request` - If `contents` is empty/missing or JSON is malformed
- `401 Unauthorized` - If `features.require_auth: true` and neither `Authorization: Bearer <token>` nor `x-goog-api-key` header (or `?key=` query parameter) is present
- `404 Not Found` - If `features.vertex_enabled: false` or `features.strict_models: true` and `model` is unknown
- `429 Too Many Requests` / `500 Internal Server Error` - When simulated error is configured on the matched rule or globally

---

### `POST /v1beta/models/{model}:streamGenerateContent`
### `POST /v1/projects/{project}/locations/{location}/publishers/google/models/{model}:streamGenerateContent`

Streams a model response in Vertex AI / Gemini format.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `alt` | string | No | Response format (`"sse"` or `"json"`). Defaults to SSE (`text/event-stream`). When `alt=json` is passed (without `Accept: text/event-stream`), returns a JSON array `[{...}, {...}]` of `GenerateContentResponse` objects. |

**Response (Streaming SSE, default or `?alt=sse`):**

- **Headers:** `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`
- **Format:** Each chunk is emitted as `data: <GenerateContentResponse JSON>\n\n`:

```text
data: {"candidates":[{"content":{"role":"model","parts":[{"text":"The current weather "}]},"index":0}],"modelVersion":"gemini-1.5-pro"}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":"in San Francisco is 18°C and sunny."}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":11,"totalTokenCount":23},"modelVersion":"gemini-1.5-pro"}

```

---

## 🧪 Testing, Inspection & Runtime Management API

These endpoints allow tests to inspect recorded requests, verify which matcher rule was triggered, dynamically manage matchers and models at runtime, configure artificial delays or error injection, and reset state between test cases.

---

### `GET /testing/requests`

Returns recorded generation requests in chronological order (oldest first).

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | No | Filter by exact `model` name |
| `api_type` | string | No | Filter by API type (`"chat_completions"`, `"responses"`, or `"vertex"`) |
| `matched_rule_id` | string | No | Filter by `matched_rule_id` (use `"__default__"` to filter requests that fell back to the default response) |
| `limit` | integer | No | Maximum number of requests to return (returns the most recent `limit` matching requests) |

**Response:**
```json
{
  "requests": [
    {
      "id": "req_1",
      "timestamp": "2025-05-10T12:00:00Z",
      "api_type": "chat_completions",
      "method": "POST",
      "path": "/v1/chat/completions",
      "headers": {
        "Content-Type": "application/json"
      },
      "model": "gpt-4o",
      "stream": false,
      "system_prompt": "You are a helpful assistant.",
      "messages": [
        {
          "role": "system",
          "content": "You are a helpful assistant."
        },
        {
          "role": "user",
          "content": "What is the weather in San Francisco?"
        }
      ],
      "last_user_message": "What is the weather in San Francisco?",
      "tools": ["get_weather"],
      "tool_choice": "auto",
      "tool_outputs": [],
      "raw_body": {
        "model": "gpt-4o",
        "messages": [
          { "role": "system", "content": "You are a helpful assistant." },
          { "role": "user", "content": "What is the weather in San Francisco?" }
        ]
      },
      "matched_rule_id": "weather-tool-trigger",
      "response_status": 200,
      "response_text": "",
      "response_tool_calls": [
        {
          "id": "call_weather_1",
          "name": "get_weather",
          "arguments": "{\"location\":\"San Francisco, CA\",\"unit\":\"celsius\"}"
        }
      ]
    }
  ],
  "count": 1
}
```

---

### `GET /testing/requests/{id}`

Retrieves a specific recorded request by ID.

**Path Parameters:**

| Parameter | Type   | Description               |
|-----------|--------|---------------------------|
| `id`      | string | The recorded request's ID |

**Response:**
Returns the single recorded request object (same structure as items in `GET /testing/requests`).

**Errors:**

- `404 Not Found` - If recorded request does not exist

---

### `DELETE /testing/requests`

Clears all recorded requests and resets the request ID counter.

**Response:**
```json
{
  "message": "Recorded requests cleared"
}
```

---

### `GET /testing/matchers`

Returns all configured request matchers, ordered by evaluation priority (descending `priority`, then registration order).

**Response:**
```json
{
  "matchers": [
    {
      "id": "weather-tool-trigger",
      "priority": 100,
      "disabled": false,
      "match": {
        "model": "gpt-4o",
        "message_contains": "weather in San Francisco",
        "has_tools": true,
        "has_tool_output": false
      },
      "response": {
        "tool_calls": [
          {
            "id": "call_weather_1",
            "name": "get_weather",
            "arguments": "{\"location\":\"San Francisco, CA\",\"unit\":\"celsius\"}"
          }
        ]
      }
    }
  ]
}
```

---

### `GET /testing/matchers/{id}`

Retrieves a specific request matcher by ID.

**Path Parameters:**

| Parameter | Type   | Description       |
|-----------|--------|-------------------|
| `id`      | string | The matcher's ID  |

**Response:**
Returns the single matcher object.

**Errors:**

- `404 Not Found` - If matcher does not exist

---

### `PUT /testing/matchers/{id}`

Creates or updates a request matcher by ID and resets its sequential selection counter.

**Path Parameters:**

| Parameter | Type   | Description       |
|-----------|--------|-------------------|
| `id`      | string | The matcher's ID  |

**Content-Type:** `application/json`

**Request:**
```json
{
  "priority": 100,
  "disabled": false,
  "match": {
    "api_type": "chat_completions",
    "model": "gpt-4o",
    "model_prefix": "gpt-4",
    "role": "user",
    "message_exact": "What is the weather in San Francisco?",
    "message_contains": "weather in San Francisco",
    "message_regex": "(?i)weather in [a-z ]+",
    "any_message_contains": "San Francisco",
    "system_contains": "helpful assistant",
    "system_regex": "(?i)helpful",
    "has_tools": true,
    "tool_name": "get_weather",
    "tool_choice": "auto",
    "has_tool_output": false,
    "tool_call_id": "call_weather_1",
    "tool_output_contains": "sunny",
    "tool_output_regex": "\"temperature\":\\s*\\d+",
    "stream": false,
    "header_match": {
      "X-Test-Case": "weather"
    }
  },
  "response": {
    "text": "The weather in San Francisco is 18°C and sunny.",
    "responses": [
      "Response 1",
      "Response 2"
    ],
    "selection_strategy": "sequential",
    "reasoning_content": "Checking weather data...",
    "tool_calls": [
      {
        "id": "call_weather_1",
        "name": "get_weather",
        "arguments": "{\"location\":\"San Francisco, CA\"}"
      }
    ],
    "finish_reason": "stop",
    "delay_ms": 0,
    "stream_chunk_delay_ms": 0,
    "stream_chunk_size": 20,
    "status_code": 200,
    "error_message": "",
    "error_type": "",
    "error_code": ""
  }
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `priority` | integer | No | Evaluation priority (higher runs first, default: `0`). |
| `disabled` | boolean | No | If `true`, this matcher is skipped during request evaluation (default: `false`). |
| `match` | object | Yes | Criteria that must all match (logical AND). Empty object `{}` matches all requests.<ul><li>`api_type` (string): `"chat_completions"`, `"responses"`, or `"vertex"`</li><li>`model` (string): Exact model match</li><li>`model_prefix` (string): Model prefix match</li><li>`role` (string): Requires a message with this role (`"system"`, `"developer"`, `"user"`, `"assistant"`, `"tool"`); scopes `message_*` checks to the last message of this role (otherwise `message_*` checks inspect the last user message, or the last message if no user message exists)</li><li>`message_exact` (string): Exact match on target message text</li><li>`message_contains` (string): Substring match on target message text</li><li>`message_regex` (string): Regular expression match on target message text</li><li>`any_message_contains` (string): Substring match against any message in the conversation</li><li>`system_contains` (string): Substring match on system/developer instructions</li><li>`system_regex` (string): Regular expression match on system/developer instructions</li><li>`has_tools` (boolean): Whether the request defines at least one tool (`true`) or zero tools (`false`)</li><li>`tool_name` (string): Requires a tool with this function name in the request's `tools` list</li><li>`tool_choice` (string): Matches normalized `tool_choice` value</li><li>`has_tool_output` (boolean): Whether the conversation contains at least one tool output (`role: "tool"`, `function_call_output`, or `functionResponse`)</li><li>`tool_call_id` (string): Requires a tool output with this `tool_call_id` / `call_id`</li><li>`tool_output_contains` (string): Substring match on the tool output content</li><li>`tool_output_regex` (string): Regular expression match on the tool output content</li><li>`stream` (boolean): Matches streaming (`true`) or non-streaming (`false`) requests</li><li>`header_match` (map[string]string): Case-insensitive HTTP header key/value matches</li></ul> |
| `response` | object | Yes | Response configuration to return when matched:<ul><li>`text` (string): Fixed text string to return</li><li>`responses` ([]string): List of candidate text strings (takes precedence over `text` when non-empty)</li><li>`selection_strategy` (string): `"sequential"` (round-robin, default), `"random"`, or `"first"`</li><li>`reasoning_content` (string): Optional reasoning content</li><li>`tool_calls` ([]object): Tool calls to return (`id`, `name`, `arguments` as string or object)</li><li>`finish_reason` (string): Override finish reason (defaults to `"tool_calls"` if `tool_calls` is non-empty, else `"stop"`)</li><li>`delay_ms` (integer): Artificial latency in ms before responding</li><li>`stream_chunk_delay_ms` (integer): Delay in ms between streamed SSE chunks</li><li>`stream_chunk_size` (integer): Characters per streamed text delta chunk (default: `20`)</li><li>`status_code` (integer): HTTP status code (default: `200`; if `>= 400`, returns a simulated error response)</li><li>`error_message` (string): Error message when `status_code >= 400`</li><li>`error_type` (string): Error type when `status_code >= 400` (default: `"api_error"`)</li><li>`error_code` (string): Error code when `status_code >= 400`</li></ul> |

**Response (Create - `201 Created` / Update - `200 OK`):**
```json
{
  "id": "ping-rule",
  "priority": 100,
  "disabled": false,
  "match": {
    "model": "gpt-4o",
    "message_contains": "ping"
  },
  "response": {
    "text": "pong"
  }
}
```

**Errors:**

- `400 Bad Request` - If request body is invalid JSON or contains an invalid regular expression

---

### `DELETE /testing/matchers/{id}`

Deletes a request matcher by ID.

**Path Parameters:**

| Parameter | Type   | Description       |
|-----------|--------|-------------------|
| `id`      | string | The matcher's ID  |

**Response:**
```json
{
  "message": "Matcher deleted"
}
```

**Errors:**

- `404 Not Found` - If matcher does not exist

---

### `PUT /testing/models/{id}`

Creates or updates a model in the available models list.

**Path Parameters:**

| Parameter | Type   | Description     |
|-----------|--------|-----------------|
| `id`      | string | The model's ID  |

**Content-Type:** `application/json`

**Request:**
```json
{
  "owned_by": "custom-org",
  "created": 1720000000
}
```

**Response (Create - `201 Created` / Update - `200 OK`):**
```json
{
  "id": "custom-model",
  "object": "model",
  "created": 1720000000,
  "owned_by": "custom-org"
}
```

**Errors:**

- `400 Bad Request` - If request body is invalid JSON

---

### `DELETE /testing/models/{id}`

Deletes a model by ID.

**Path Parameters:**

| Parameter | Type   | Description     |
|-----------|--------|-----------------|
| `id`      | string | The model's ID  |

**Response:**
```json
{
  "message": "Model deleted"
}
```

**Errors:**

- `404 Not Found` - If model does not exist

---

### `GET /testing/config`

Returns the current runtime configuration and global testing parameters.

**Response:**
```json
{
  "port": 8080,
  "default_response": "I am a local AI provider default response.",
  "default_responses": [],
  "selection_strategy": "sequential",
  "delay_ms": 0,
  "stream_chunk_delay_ms": 0,
  "stream_chunk_size": 20,
  "simulated_error": {
    "enabled": false,
    "status_code": 500,
    "message": "",
    "type": "api_error",
    "code": ""
  },
  "features": {
    "chat_completions_enabled": true,
    "responses_enabled": true,
    "vertex_enabled": true,
    "strict_models": false,
    "require_auth": false
  }
}
```

---

### `PATCH /testing/config`

Updates runtime configuration and global testing parameters. All fields are optional; only provided fields are updated.

**Content-Type:** `application/json`

**Request:**
```json
{
  "default_response": "Updated fallback text",
  "default_responses": ["Fallback 1", "Fallback 2"],
  "selection_strategy": "sequential",
  "delay_ms": 50,
  "stream_chunk_delay_ms": 10,
  "stream_chunk_size": 15,
  "simulated_error": {
    "enabled": true,
    "status_code": 429,
    "message": "Rate limit exceeded",
    "type": "rate_limit_error",
    "code": "rate_limit_exceeded"
  },
  "features": {
    "chat_completions_enabled": true,
    "responses_enabled": true,
    "vertex_enabled": true,
    "strict_models": true,
    "require_auth": false
  }
}
```

**Response:**
Returns the updated runtime configuration object (same schema as `GET /testing/config`).

**Errors:**

- `400 Bad Request` - If request body is invalid JSON

---

### `POST /testing/reset`

Resets all models, matchers, sequential counters, runtime configuration, and recorded requests back to the initial state loaded from the YAML configuration file at startup.

**Response:**
```json
{
  "message": "State reset to initial configuration"
}
```

---

## 📋 Response Codes Summary

| Code | Description                                                      |
|------|------------------------------------------------------------------|
| 200  | Success - Request completed successfully                          |
| 201  | Created - Resource created successfully                           |
| 204  | No Content - Success with no response body (CORS preflight)      |
| 400  | Bad Request - Invalid request format, missing fields, or bad regex|
| 401  | Unauthorized - Authentication required and missing/invalid       |
| 404  | Not Found - Resource, model, or disabled endpoint does not exist |
| 429  | Too Many Requests - Simulated rate limit error                   |
| 500  | Internal Server Error - Simulated server error                   |

---

## 📄 License

MIT © 2026 SIOCODE
