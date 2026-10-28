package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ChatCompletionStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type ChatCompletionRequest struct {
	Model               string                       `json:"model"`
	Messages            []map[string]interface{}     `json:"messages"`
	Tools               []map[string]interface{}     `json:"tools"`
	ToolChoice          interface{}                  `json:"tool_choice"`
	ParallelToolCalls   *bool                        `json:"parallel_tool_calls"`
	Stream              bool                         `json:"stream"`
	StreamOptions       *ChatCompletionStreamOptions `json:"stream_options"`
	Temperature         *float64                     `json:"temperature"`
	TopP                *float64                     `json:"top_p"`
	MaxTokens           *int                         `json:"max_tokens"`
	MaxCompletionTokens *int                         `json:"max_completion_tokens"`
	ResponseFormat      interface{}                  `json:"response_format"`
	User                string                       `json:"user"`
}

func extractMessageContentText(rawContent interface{}) string {
	if rawContent == nil {
		return ""
	}
	switch v := rawContent.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if t, ok := m["text"].(string); ok {
					parts = append(parts, t)
				}
			} else if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "")
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func normalizeToolChoice(tc interface{}, hasTools bool) string {
	if tc == nil {
		if hasTools {
			return "auto"
		}
		return ""
	}
	switch v := tc.(type) {
	case string:
		return v
	case map[string]interface{}:
		if fn, ok := v["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok && name != "" {
				return name
			}
		}
		if name, ok := v["name"].(string); ok && name != "" {
			return name
		}
		if t, ok := v["type"].(string); ok {
			return t
		}
	}
	return ""
}

func extractToolNames(rawTools []map[string]interface{}) []string {
	names := make([]string, 0, len(rawTools))
	for _, t := range rawTools {
		if fn, ok := t["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok && name != "" {
				names = append(names, name)
				continue
			}
		}
		if name, ok := t["name"].(string); ok && name != "" {
			names = append(names, name)
		}
	}
	return names
}

func POST_v1_chat_completions(w http.ResponseWriter, r *http.Request) {
	AppContext.mu.RLock()
	features := AppContext.Config.Features
	AppContext.mu.RUnlock()

	if !features.ChatCompletionsEnabled {
		writeOpenAIError(w, http.StatusNotFound, "The chat completions endpoint is disabled", "invalid_request_error", nil, "endpoint_disabled")
		return
	}

	if features.RequireAuth && !checkOpenAIAuth(r) {
		writeOpenAIError(w, http.StatusUnauthorized, "Missing or invalid Authorization header", "authentication_error", nil, "invalid_api_key")
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "Failed to read request body", "invalid_request_error", nil, "invalid_body")
		return
	}

	var rawBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &rawBody); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "Malformed JSON in request body", "invalid_request_error", nil, "invalid_json")
		return
	}

	var req ChatCompletionRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "Invalid chat completion request payload", "invalid_request_error", nil, "invalid_request")
		return
	}

	if strings.TrimSpace(req.Model) == "" {
		writeOpenAIError(w, http.StatusBadRequest, "Missing required parameter: 'model'", "invalid_request_error", "model", "missing_required_parameter")
		return
	}

	if len(req.Messages) == 0 {
		writeOpenAIError(w, http.StatusBadRequest, "Missing required parameter: 'messages'", "invalid_request_error", "messages", "missing_required_parameter")
		return
	}

	if features.StrictModels && !modelExists(req.Model) {
		writeOpenAIError(
			w,
			http.StatusNotFound,
			fmt.Sprintf("The model '%s' does not exist", req.Model),
			"invalid_request_error",
			"model",
			"model_not_found",
		)
		return
	}

	normMessages := make([]NormalizedMessage, 0, len(req.Messages))
	toolOutputs := make([]NormalizedToolOutput, 0)
	var systemParts []string
	lastUserMessage := ""
	promptTextBuilder := strings.Builder{}

	for _, rawMsg := range req.Messages {
		role, _ := rawMsg["role"].(string)
		contentStr := extractMessageContentText(rawMsg["content"])
		name, _ := rawMsg["name"].(string)
		toolCallId, _ := rawMsg["tool_call_id"].(string)

		var msgToolCalls []ToolCallConfig
		if rawTcs, ok := rawMsg["tool_calls"].([]interface{}); ok {
			for idx, rawTc := range rawTcs {
				if tcMap, ok := rawTc.(map[string]interface{}); ok {
					tcId, _ := tcMap["id"].(string)
					if tcId == "" {
						tcId = fmt.Sprintf("call_%d", idx+1)
					}
					tcName := ""
					tcArgs := "{}"
					if fnMap, ok := tcMap["function"].(map[string]interface{}); ok {
						tcName, _ = fnMap["name"].(string)
						tcArgs = convertArgumentsToString(fnMap["arguments"])
					} else {
						tcName, _ = tcMap["name"].(string)
						tcArgs = convertArgumentsToString(tcMap["arguments"])
					}
					msgToolCalls = append(msgToolCalls, ToolCallConfig{
						Id:        tcId,
						Name:      tcName,
						Arguments: tcArgs,
					})
				}
			}
		}

		normMsg := NormalizedMessage{
			Role:       role,
			Content:    contentStr,
			Name:       name,
			ToolCalls:  msgToolCalls,
			ToolCallId: toolCallId,
		}
		normMessages = append(normMessages, normMsg)
		promptTextBuilder.WriteString(contentStr)
		promptTextBuilder.WriteString(" ")

		if role == "system" || role == "developer" {
			if contentStr != "" {
				systemParts = append(systemParts, contentStr)
			}
		} else if role == "user" {
			lastUserMessage = contentStr
		} else if role == "tool" {
			toolOutputs = append(toolOutputs, NormalizedToolOutput{
				ToolCallId: toolCallId,
				Name:       name,
				Content:    contentStr,
				Output:     contentStr,
			})
		}
	}

	toolNames := extractToolNames(req.Tools)
	toolChoice := normalizeToolChoice(req.ToolChoice, len(toolNames) > 0)

	recordedReq := RecordedRequest{
		ApiType:         "chat_completions",
		Method:          r.Method,
		Path:            r.URL.Path,
		Headers:         extractRequestHeaders(r),
		Model:           req.Model,
		Stream:          req.Stream,
		SystemPrompt:    strings.Join(systemParts, "\n"),
		Messages:        normMessages,
		LastUserMessage: lastUserMessage,
		Tools:           toolNames,
		ToolChoice:      toolChoice,
		ToolOutputs:     toolOutputs,
		RawBody:         rawBody,
	}

	res := evaluateAndRecordRequest(&recordedReq, r.Header)
	if res.StatusCode >= 400 {
		var codeVal interface{}
		if res.ErrorCode != "" {
			codeVal = res.ErrorCode
		}
		writeOpenAIError(w, res.StatusCode, res.ErrorMessage, res.ErrorType, nil, codeVal)
		return
	}

	cmplId := nextChatCompletionId()
	created := time.Now().Unix()
	promptTokens := estimateTokens(promptTextBuilder.String())
	completionSource := res.Text
	if completionSource == "" && len(res.ToolCalls) > 0 {
		for _, tc := range res.ToolCalls {
			completionSource += tc.Name + " " + tc.Arguments + " "
		}
	}
	completionTokens := estimateTokens(completionSource)
	usageObj := map[string]interface{}{
		"prompt_tokens":     promptTokens,
		"completion_tokens": completionTokens,
		"total_tokens":      promptTokens + completionTokens,
		"prompt_tokens_details": map[string]interface{}{
			"cached_tokens": 0,
			"audio_tokens":  0,
		},
		"completion_tokens_details": map[string]interface{}{
			"reasoning_tokens":           0,
			"audio_tokens":               0,
			"accepted_prediction_tokens": 0,
			"rejected_prediction_tokens": 0,
		},
	}

	if !req.Stream {
		msgObj := map[string]interface{}{
			"role":    "assistant",
			"refusal": nil,
		}
		if len(res.ToolCalls) > 0 && res.Text == "" {
			msgObj["content"] = nil
		} else {
			msgObj["content"] = res.Text
		}
		if res.ReasoningContent != "" {
			msgObj["reasoning_content"] = res.ReasoningContent
		}
		if len(res.ToolCalls) > 0 {
			formattedTcs := make([]map[string]interface{}, 0, len(res.ToolCalls))
			for _, tc := range res.ToolCalls {
				formattedTcs = append(formattedTcs, map[string]interface{}{
					"id":   tc.Id,
					"type": "function",
					"function": map[string]interface{}{
						"name":      tc.Name,
						"arguments": tc.Arguments,
					},
				})
			}
			msgObj["tool_calls"] = formattedTcs
		}

		respPayload := map[string]interface{}{
			"id":                 cmplId,
			"object":             "chat.completion",
			"created":            created,
			"model":              req.Model,
			"system_fingerprint": "fp_local_ai_provider",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       msgObj,
					"logprobs":      nil,
					"finish_reason": res.FinishReason,
				},
			},
			"usage": usageObj,
		}
		writeJSON(w, http.StatusOK, respPayload)
		return
	}

	// Streaming SSE response
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	writeSSEChunk := func(payload map[string]interface{}) {
		b, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	// 1. Initial role chunk
	initialDelta := map[string]interface{}{
		"role":    "assistant",
		"refusal": nil,
	}
	if len(res.ToolCalls) > 0 && res.Text == "" {
		initialDelta["content"] = nil
	} else {
		initialDelta["content"] = ""
	}
	writeSSEChunk(map[string]interface{}{
		"id":                 cmplId,
		"object":             "chat.completion.chunk",
		"created":            created,
		"model":              req.Model,
		"system_fingerprint": "fp_local_ai_provider",
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         initialDelta,
				"logprobs":      nil,
				"finish_reason": nil,
			},
		},
	})

	// 2. Content chunks (if any)
	if res.Text != "" {
		chunks := splitIntoChunks(res.Text, res.StreamChunkSize)
		for _, chunk := range chunks {
			if res.StreamChunkDelayMs > 0 {
				time.Sleep(time.Duration(res.StreamChunkDelayMs) * time.Millisecond)
			}
			writeSSEChunk(map[string]interface{}{
				"id":                 cmplId,
				"object":             "chat.completion.chunk",
				"created":            created,
				"model":              req.Model,
				"system_fingerprint": "fp_local_ai_provider",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"content": chunk,
						},
						"logprobs":      nil,
						"finish_reason": nil,
					},
				},
			})
		}
	}

	// 3. Tool call chunks (if any)
	if len(res.ToolCalls) > 0 {
		if res.StreamChunkDelayMs > 0 {
			time.Sleep(time.Duration(res.StreamChunkDelayMs) * time.Millisecond)
		}
		streamedTcs := make([]map[string]interface{}, 0, len(res.ToolCalls))
		for idx, tc := range res.ToolCalls {
			streamedTcs = append(streamedTcs, map[string]interface{}{
				"index": idx,
				"id":    tc.Id,
				"type":  "function",
				"function": map[string]interface{}{
					"name":      tc.Name,
					"arguments": tc.Arguments,
				},
			})
		}
		writeSSEChunk(map[string]interface{}{
			"id":                 cmplId,
			"object":             "chat.completion.chunk",
			"created":            created,
			"model":              req.Model,
			"system_fingerprint": "fp_local_ai_provider",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"tool_calls": streamedTcs,
					},
					"logprobs":      nil,
					"finish_reason": nil,
				},
			},
		})
	}

	// 4. Finish reason chunk
	writeSSEChunk(map[string]interface{}{
		"id":                 cmplId,
		"object":             "chat.completion.chunk",
		"created":            created,
		"model":              req.Model,
		"system_fingerprint": "fp_local_ai_provider",
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         map[string]interface{}{},
				"logprobs":      nil,
				"finish_reason": res.FinishReason,
			},
		},
	})

	// 5. Optional usage chunk
	if req.StreamOptions != nil && req.StreamOptions.IncludeUsage {
		writeSSEChunk(map[string]interface{}{
			"id":                 cmplId,
			"object":             "chat.completion.chunk",
			"created":            created,
			"model":              req.Model,
			"system_fingerprint": "fp_local_ai_provider",
			"choices":            []map[string]interface{}{},
			"usage":              usageObj,
		})
	}

	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
