package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ResponsesRequest struct {
	Model              string                   `json:"model"`
	Input              interface{}              `json:"input"`
	Instructions       *string                  `json:"instructions"`
	Tools              []map[string]interface{} `json:"tools"`
	ToolChoice         interface{}              `json:"tool_choice"`
	ParallelToolCalls  *bool                    `json:"parallel_tool_calls"`
	PreviousResponseId *string                  `json:"previous_response_id"`
	Stream             bool                     `json:"stream"`
	Temperature        *float64                 `json:"temperature"`
	TopP               *float64                 `json:"top_p"`
	MaxOutputTokens    *int                     `json:"max_output_tokens"`
	Store              *bool                    `json:"store"`
	Metadata           map[string]interface{}   `json:"metadata"`
	Text               map[string]interface{}   `json:"text"`
	Truncation         string                   `json:"truncation"`
	User               *string                  `json:"user"`
}

func POST_v1_responses(w http.ResponseWriter, r *http.Request) {
	AppContext.mu.RLock()
	features := AppContext.Config.Features
	AppContext.mu.RUnlock()

	if !features.ResponsesEnabled {
		writeOpenAIError(w, http.StatusNotFound, "The responses endpoint is disabled", "invalid_request_error", nil, "endpoint_disabled")
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

	var req ResponsesRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "Invalid responses request payload", "invalid_request_error", nil, "invalid_request")
		return
	}

	if strings.TrimSpace(req.Model) == "" {
		writeOpenAIError(w, http.StatusBadRequest, "Missing required parameter: 'model'", "invalid_request_error", "model", "missing_required_parameter")
		return
	}

	if req.Input == nil {
		writeOpenAIError(w, http.StatusBadRequest, "Missing required parameter: 'input'", "invalid_request_error", "input", "missing_required_parameter")
		return
	}

	normMessages := make([]NormalizedMessage, 0)
	toolOutputs := make([]NormalizedToolOutput, 0)
	var systemParts []string
	lastUserMessage := ""
	promptTextBuilder := strings.Builder{}

	if req.Instructions != nil && *req.Instructions != "" {
		systemParts = append(systemParts, *req.Instructions)
		normMessages = append(normMessages, NormalizedMessage{
			Role:    "system",
			Content: *req.Instructions,
		})
		promptTextBuilder.WriteString(*req.Instructions)
		promptTextBuilder.WriteString(" ")
	}

	switch inp := req.Input.(type) {
	case string:
		if strings.TrimSpace(inp) == "" {
			writeOpenAIError(w, http.StatusBadRequest, "Parameter 'input' cannot be empty", "invalid_request_error", "input", "invalid_value")
			return
		}
		normMessages = append(normMessages, NormalizedMessage{
			Role:    "user",
			Content: inp,
		})
		lastUserMessage = inp
		promptTextBuilder.WriteString(inp)
	case []interface{}:
		if len(inp) == 0 {
			writeOpenAIError(w, http.StatusBadRequest, "Parameter 'input' cannot be an empty array", "invalid_request_error", "input", "invalid_value")
			return
		}
		for idx, rawItem := range inp {
			if s, ok := rawItem.(string); ok {
				normMessages = append(normMessages, NormalizedMessage{
					Role:    "user",
					Content: s,
				})
				lastUserMessage = s
				promptTextBuilder.WriteString(s)
				promptTextBuilder.WriteString(" ")
				continue
			}
			itemMap, ok := rawItem.(map[string]interface{})
			if !ok {
				continue
			}
			itemType, _ := itemMap["type"].(string)
			role, _ := itemMap["role"].(string)

			switch itemType {
			case "function_call":
				callId, _ := itemMap["call_id"].(string)
				if callId == "" {
					callId, _ = itemMap["id"].(string)
				}
				if callId == "" {
					callId = fmt.Sprintf("call_%d", idx+1)
				}
				name, _ := itemMap["name"].(string)
				args := convertArgumentsToString(itemMap["arguments"])
				normMessages = append(normMessages, NormalizedMessage{
					Role:    "assistant",
					Content: "",
					ToolCalls: []ToolCallConfig{
						{
							Id:        callId,
							Name:      name,
							Arguments: args,
						},
					},
				})
				promptTextBuilder.WriteString(name + " " + args + " ")
			case "function_call_output":
				callId, _ := itemMap["call_id"].(string)
				if callId == "" {
					callId, _ = itemMap["tool_call_id"].(string)
				}
				name, _ := itemMap["name"].(string)
				outStr := extractMessageContentText(itemMap["output"])
				normMessages = append(normMessages, NormalizedMessage{
					Role:       "tool",
					Content:    outStr,
					Name:       name,
					ToolCallId: callId,
				})
				toolOutputs = append(toolOutputs, NormalizedToolOutput{
					ToolCallId: callId,
					Name:       name,
					Content:    outStr,
					Output:     outStr,
				})
				promptTextBuilder.WriteString(outStr)
				promptTextBuilder.WriteString(" ")
			default:
				if role == "" {
					role = "user"
				}
				contentStr := extractMessageContentText(itemMap["content"])
				callId, _ := itemMap["tool_call_id"].(string)
				if callId == "" {
					callId, _ = itemMap["call_id"].(string)
				}
				name, _ := itemMap["name"].(string)
				normMessages = append(normMessages, NormalizedMessage{
					Role:       role,
					Content:    contentStr,
					Name:       name,
					ToolCallId: callId,
				})
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
						ToolCallId: callId,
						Name:       name,
						Content:    contentStr,
						Output:     contentStr,
					})
				}
			}
		}
	default:
		writeOpenAIError(w, http.StatusBadRequest, "Invalid 'input' type: must be string or array", "invalid_request_error", "input", "invalid_type")
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

	toolNames := extractToolNames(req.Tools)
	toolChoice := normalizeToolChoice(req.ToolChoice, len(toolNames) > 0)

	recordedReq := RecordedRequest{
		ApiType:         "responses",
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

	respId, msgId := nextResponseIds()
	createdAt := time.Now().Unix()

	inputTokens := estimateTokens(promptTextBuilder.String())
	outputSource := res.Text
	if outputSource == "" && len(res.ToolCalls) > 0 {
		for _, tc := range res.ToolCalls {
			outputSource += tc.Name + " " + tc.Arguments + " "
		}
	}
	outputTokens := estimateTokens(outputSource)
	usageObj := map[string]interface{}{
		"input_tokens": inputTokens,
		"input_tokens_details": map[string]interface{}{
			"cached_tokens": 0,
		},
		"output_tokens": outputTokens,
		"output_tokens_details": map[string]interface{}{
			"reasoning_tokens": 0,
		},
		"total_tokens": inputTokens + outputTokens,
	}

	parallelToolCalls := true
	if req.ParallelToolCalls != nil {
		parallelToolCalls = *req.ParallelToolCalls
	}
	store := true
	if req.Store != nil {
		store = *req.Store
	}
	temperature := 1.0
	if req.Temperature != nil {
		temperature = *req.Temperature
	}
	topP := 1.0
	if req.TopP != nil {
		topP = *req.TopP
	}
	truncation := "disabled"
	if req.Truncation != "" {
		truncation = req.Truncation
	}
	textCfg := req.Text
	if textCfg == nil {
		textCfg = map[string]interface{}{
			"format": map[string]interface{}{
				"type": "text",
			},
		}
	}
	toolChoiceOut := req.ToolChoice
	if toolChoiceOut == nil {
		toolChoiceOut = "auto"
	}
	toolsOut := req.Tools
	if toolsOut == nil {
		toolsOut = make([]map[string]interface{}, 0)
	}
	metadataOut := req.Metadata
	if metadataOut == nil {
		metadataOut = make(map[string]interface{})
	}

	outputItems := make([]map[string]interface{}, 0)
	if len(res.ToolCalls) > 0 {
		for idx, tc := range res.ToolCalls {
			fcId := fmt.Sprintf("fc_%d", idx+1)
			outputItems = append(outputItems, map[string]interface{}{
				"type":      "function_call",
				"id":        fcId,
				"call_id":   tc.Id,
				"name":      tc.Name,
				"arguments": tc.Arguments,
				"status":    "completed",
			})
		}
	}
	if len(res.ToolCalls) == 0 || res.Text != "" {
		outputItems = append(outputItems, map[string]interface{}{
			"type":   "message",
			"id":     msgId,
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]interface{}{
				{
					"type":        "output_text",
					"text":        res.Text,
					"annotations": []interface{}{},
				},
			},
		})
	}

	buildFullResponse := func(status string, out []map[string]interface{}, includeUsage bool) map[string]interface{} {
		obj := map[string]interface{}{
			"id":                   respId,
			"object":               "response",
			"created_at":           createdAt,
			"status":               status,
			"error":                nil,
			"incomplete_details":   nil,
			"instructions":         req.Instructions,
			"max_output_tokens":    req.MaxOutputTokens,
			"model":                req.Model,
			"output":               out,
			"output_text":          res.Text,
			"parallel_tool_calls":  parallelToolCalls,
			"previous_response_id": req.PreviousResponseId,
			"reasoning": map[string]interface{}{
				"effort":  nil,
				"summary": nil,
			},
			"store":       store,
			"temperature": temperature,
			"text":        textCfg,
			"tool_choice": toolChoiceOut,
			"tools":       toolsOut,
			"top_p":       topP,
			"truncation":  truncation,
			"user":        req.User,
			"metadata":    metadataOut,
		}
		if includeUsage {
			obj["usage"] = usageObj
		}
		return obj
	}

	if !req.Stream {
		writeJSON(w, http.StatusOK, buildFullResponse("completed", outputItems, true))
		return
	}

	// Streaming SSE response
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	seq := 0
	writeEvent := func(eventType string, payload map[string]interface{}) {
		payload["type"] = eventType
		payload["sequence_number"] = seq
		seq++
		b, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(b))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	inProgressResp := map[string]interface{}{
		"id":                 respId,
		"object":             "response",
		"created_at":         createdAt,
		"status":             "in_progress",
		"error":              nil,
		"incomplete_details": nil,
		"model":              req.Model,
		"output":             []interface{}{},
	}

	writeEvent("response.created", map[string]interface{}{
		"response": inProgressResp,
	})
	writeEvent("response.in_progress", map[string]interface{}{
		"response": inProgressResp,
	})

	outputIndex := 0
	if len(res.ToolCalls) > 0 {
		for idx, tc := range res.ToolCalls {
			fcId := fmt.Sprintf("fc_%d", idx+1)
			writeEvent("response.output_item.added", map[string]interface{}{
				"output_index": outputIndex,
				"item": map[string]interface{}{
					"id":        fcId,
					"type":      "function_call",
					"status":    "in_progress",
					"call_id":   tc.Id,
					"name":      tc.Name,
					"arguments": "",
				},
			})

			if res.StreamChunkDelayMs > 0 {
				time.Sleep(time.Duration(res.StreamChunkDelayMs) * time.Millisecond)
			}

			writeEvent("response.function_call_arguments.delta", map[string]interface{}{
				"item_id":      fcId,
				"output_index": outputIndex,
				"call_id":      tc.Id,
				"name":         tc.Name,
				"delta":        tc.Arguments,
			})

			writeEvent("response.function_call_arguments.done", map[string]interface{}{
				"item_id":      fcId,
				"output_index": outputIndex,
				"call_id":      tc.Id,
				"name":         tc.Name,
				"arguments":    tc.Arguments,
			})

			writeEvent("response.output_item.done", map[string]interface{}{
				"output_index": outputIndex,
				"item": map[string]interface{}{
					"id":        fcId,
					"type":      "function_call",
					"status":    "completed",
					"call_id":   tc.Id,
					"name":      tc.Name,
					"arguments": tc.Arguments,
				},
			})
			outputIndex++
		}
	}

	if len(res.ToolCalls) == 0 || res.Text != "" {
		writeEvent("response.output_item.added", map[string]interface{}{
			"output_index": outputIndex,
			"item": map[string]interface{}{
				"id":      msgId,
				"type":    "message",
				"status":  "in_progress",
				"role":    "assistant",
				"content": []interface{}{},
			},
		})

		writeEvent("response.content_part.added", map[string]interface{}{
			"item_id":       msgId,
			"output_index":  outputIndex,
			"content_index": 0,
			"part": map[string]interface{}{
				"type":        "output_text",
				"text":        "",
				"annotations": []interface{}{},
			},
		})

		chunks := splitIntoChunks(res.Text, res.StreamChunkSize)
		for _, chunk := range chunks {
			if res.StreamChunkDelayMs > 0 {
				time.Sleep(time.Duration(res.StreamChunkDelayMs) * time.Millisecond)
			}
			writeEvent("response.output_text.delta", map[string]interface{}{
				"item_id":       msgId,
				"output_index":  outputIndex,
				"content_index": 0,
				"delta":         chunk,
			})
		}

		writeEvent("response.output_text.done", map[string]interface{}{
			"item_id":       msgId,
			"output_index":  outputIndex,
			"content_index": 0,
			"text":          res.Text,
		})

		completedPart := map[string]interface{}{
			"type":        "output_text",
			"text":        res.Text,
			"annotations": []interface{}{},
		}

		writeEvent("response.content_part.done", map[string]interface{}{
			"item_id":       msgId,
			"output_index":  outputIndex,
			"content_index": 0,
			"part":          completedPart,
		})

		writeEvent("response.output_item.done", map[string]interface{}{
			"output_index": outputIndex,
			"item": map[string]interface{}{
				"id":      msgId,
				"type":    "message",
				"status":  "completed",
				"role":    "assistant",
				"content": []interface{}{completedPart},
			},
		})
	}

	writeEvent("response.completed", map[string]interface{}{
		"response": buildFullResponse("completed", outputItems, true),
	})

	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
