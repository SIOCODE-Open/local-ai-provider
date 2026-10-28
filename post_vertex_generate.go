package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func extractVertexSystemText(rawBody map[string]interface{}) string {
	sysRaw := rawBody["systemInstruction"]
	if sysRaw == nil {
		sysRaw = rawBody["system_instruction"]
	}
	if sysRaw == nil {
		return ""
	}
	sysMap, ok := sysRaw.(map[string]interface{})
	if !ok {
		return ""
	}
	partsRaw, ok := sysMap["parts"].([]interface{})
	if !ok {
		return ""
	}
	var texts []string
	for _, p := range partsRaw {
		if pm, ok := p.(map[string]interface{}); ok {
			if t, ok := pm["text"].(string); ok && t != "" {
				texts = append(texts, t)
			}
		}
	}
	return strings.Join(texts, "\n")
}

func extractVertexTools(rawBody map[string]interface{}) []string {
	toolsRaw, ok := rawBody["tools"].([]interface{})
	if !ok {
		return []string{}
	}
	names := make([]string, 0)
	for _, tr := range toolsRaw {
		tm, ok := tr.(map[string]interface{})
		if !ok {
			continue
		}
		declsRaw := tm["functionDeclarations"]
		if declsRaw == nil {
			declsRaw = tm["function_declarations"]
		}
		if decls, ok := declsRaw.([]interface{}); ok {
			for _, dr := range decls {
				if dm, ok := dr.(map[string]interface{}); ok {
					if name, ok := dm["name"].(string); ok && name != "" {
						names = append(names, name)
					}
				}
			}
		}
	}
	return names
}

func extractVertexToolChoice(rawBody map[string]interface{}, hasTools bool) string {
	tcRaw := rawBody["toolConfig"]
	if tcRaw == nil {
		tcRaw = rawBody["tool_config"]
	}
	if tcMap, ok := tcRaw.(map[string]interface{}); ok {
		fccRaw := tcMap["functionCallingConfig"]
		if fccRaw == nil {
			fccRaw = tcMap["function_calling_config"]
		}
		if fccMap, ok := fccRaw.(map[string]interface{}); ok {
			if mode, ok := fccMap["mode"].(string); ok && mode != "" {
				switch strings.ToUpper(mode) {
				case "AUTO":
					return "auto"
				case "ANY":
					return "required"
				case "NONE":
					return "none"
				default:
					return strings.ToLower(mode)
				}
			}
		}
	}
	if hasTools {
		return "auto"
	}
	return ""
}

func POST_vertex_model_action(w http.ResponseWriter, r *http.Request) {
	AppContext.mu.RLock()
	features := AppContext.Config.Features
	AppContext.mu.RUnlock()

	if !features.VertexEnabled {
		writeVertexError(w, http.StatusNotFound, "The Vertex AI / Gemini endpoint is disabled", "invalid_request_error", "endpoint_disabled")
		return
	}

	if features.RequireAuth && !checkVertexAuth(r) {
		writeVertexError(w, http.StatusUnauthorized, "Request is missing required authentication credential", "authentication_error", "unauthenticated")
		return
	}

	vars := mux.Vars(r)
	modelAction := vars["model_action"]
	colonIdx := strings.LastIndex(modelAction, ":")
	if colonIdx <= 0 {
		writeVertexError(w, http.StatusNotFound, "Invalid Vertex AI model action path", "invalid_request_error", "not_found")
		return
	}
	model := strings.TrimPrefix(modelAction[:colonIdx], "models/")
	action := modelAction[colonIdx+1:]

	var isStream bool
	switch action {
	case "generateContent":
		isStream = false
	case "streamGenerateContent":
		isStream = true
	default:
		writeVertexError(w, http.StatusNotFound, fmt.Sprintf("Unsupported model action '%s'", action), "invalid_request_error", "not_found")
		return
	}

	if strings.TrimSpace(model) == "" {
		writeVertexError(w, http.StatusBadRequest, "Missing model identifier in URL path", "invalid_request_error", "missing_model")
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeVertexError(w, http.StatusBadRequest, "Failed to read request body", "invalid_request_error", "invalid_body")
		return
	}

	var rawBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &rawBody); err != nil {
		writeVertexError(w, http.StatusBadRequest, "Malformed JSON in request body", "invalid_request_error", "invalid_json")
		return
	}

	contentsRaw, ok := rawBody["contents"].([]interface{})
	if !ok || len(contentsRaw) == 0 {
		writeVertexError(w, http.StatusBadRequest, "Missing or empty required field: 'contents'", "invalid_request_error", "missing_contents")
		return
	}

	if features.StrictModels && !modelExists(model) {
		writeVertexError(
			w,
			http.StatusNotFound,
			fmt.Sprintf("Model '%s' was not found", model),
			"invalid_request_error",
			"model_not_found",
		)
		return
	}

	systemPrompt := extractVertexSystemText(rawBody)
	normMessages := make([]NormalizedMessage, 0)
	toolOutputs := make([]NormalizedToolOutput, 0)
	lastUserMessage := ""
	promptTextBuilder := strings.Builder{}

	if systemPrompt != "" {
		normMessages = append(normMessages, NormalizedMessage{
			Role:    "system",
			Content: systemPrompt,
		})
		promptTextBuilder.WriteString(systemPrompt)
		promptTextBuilder.WriteString(" ")
	}

	for _, cItem := range contentsRaw {
		cMap, ok := cItem.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := cMap["role"].(string)
		if role == "" {
			role = "user"
		}
		partsRaw, _ := cMap["parts"].([]interface{})

		var textParts []string
		var msgToolCalls []ToolCallConfig
		hasFuncResp := false

		for pIdx, pItem := range partsRaw {
			pMap, ok := pItem.(map[string]interface{})
			if !ok {
				continue
			}
			if t, ok := pMap["text"].(string); ok {
				textParts = append(textParts, t)
			}
			fcRaw := pMap["functionCall"]
			if fcRaw == nil {
				fcRaw = pMap["function_call"]
			}
			if fcMap, ok := fcRaw.(map[string]interface{}); ok {
				fcId, _ := fcMap["id"].(string)
				if fcId == "" {
					fcId = fmt.Sprintf("call_%d", pIdx+1)
				}
				fcName, _ := fcMap["name"].(string)
				fcArgs := convertArgumentsToString(fcMap["args"])
				msgToolCalls = append(msgToolCalls, ToolCallConfig{
					Id:        fcId,
					Name:      fcName,
					Arguments: fcArgs,
				})
				promptTextBuilder.WriteString(fcName + " " + fcArgs + " ")
			}

			frRaw := pMap["functionResponse"]
			if frRaw == nil {
				frRaw = pMap["function_response"]
			}
			if frMap, ok := frRaw.(map[string]interface{}); ok {
				hasFuncResp = true
				frId, _ := frMap["id"].(string)
				frName, _ := frMap["name"].(string)
				if frId == "" {
					frId = frName
				}
				respStr := convertArgumentsToString(frMap["response"])
				toolOutputs = append(toolOutputs, NormalizedToolOutput{
					ToolCallId: frId,
					Name:       frName,
					Content:    respStr,
					Output:     respStr,
				})
				normMessages = append(normMessages, NormalizedMessage{
					Role:       "tool",
					Content:    respStr,
					Name:       frName,
					ToolCallId: frId,
				})
				promptTextBuilder.WriteString(respStr + " ")
			}
		}

		combinedText := strings.Join(textParts, "")
		if combinedText != "" || len(msgToolCalls) > 0 || !hasFuncResp {
			normMessages = append(normMessages, NormalizedMessage{
				Role:      role,
				Content:   combinedText,
				ToolCalls: msgToolCalls,
			})
			promptTextBuilder.WriteString(combinedText + " ")
			if role == "user" && combinedText != "" {
				lastUserMessage = combinedText
			}
		}
	}

	toolNames := extractVertexTools(rawBody)
	toolChoice := extractVertexToolChoice(rawBody, len(toolNames) > 0)

	recordedReq := RecordedRequest{
		ApiType:         "vertex",
		Method:          r.Method,
		Path:            r.URL.Path,
		Headers:         extractRequestHeaders(r),
		Model:           model,
		Stream:          isStream,
		SystemPrompt:    systemPrompt,
		Messages:        normMessages,
		LastUserMessage: lastUserMessage,
		Tools:           toolNames,
		ToolChoice:      toolChoice,
		ToolOutputs:     toolOutputs,
		RawBody:         rawBody,
	}

	res := evaluateAndRecordRequest(&recordedReq, r.Header)
	if res.StatusCode >= 400 {
		writeVertexError(w, res.StatusCode, res.ErrorMessage, res.ErrorType, res.ErrorCode)
		return
	}

	promptTokens := estimateTokens(promptTextBuilder.String())
	completionSource := res.Text
	if completionSource == "" && len(res.ToolCalls) > 0 {
		for _, tc := range res.ToolCalls {
			completionSource += tc.Name + " " + tc.Arguments + " "
		}
	}
	candidateTokens := estimateTokens(completionSource)
	usageMetadata := map[string]interface{}{
		"promptTokenCount":     promptTokens,
		"candidatesTokenCount": candidateTokens,
		"totalTokenCount":      promptTokens + candidateTokens,
	}

	finishReason := "STOP"
	if res.FinishReason != "" && res.FinishReason != "stop" && res.FinishReason != "tool_calls" {
		finishReason = strings.ToUpper(res.FinishReason)
	}

	buildToolCallParts := func() []map[string]interface{} {
		parts := make([]map[string]interface{}, 0, len(res.ToolCalls))
		for _, tc := range res.ToolCalls {
			var parsedArgs interface{}
			if err := json.Unmarshal([]byte(tc.Arguments), &parsedArgs); err != nil || parsedArgs == nil {
				parsedArgs = map[string]interface{}{}
			}
			parts = append(parts, map[string]interface{}{
				"functionCall": map[string]interface{}{
					"id":   tc.Id,
					"name": tc.Name,
					"args": parsedArgs,
				},
			})
		}
		return parts
	}

	if !isStream {
		parts := make([]map[string]interface{}, 0)
		if len(res.ToolCalls) == 0 || res.Text != "" {
			parts = append(parts, map[string]interface{}{
				"text": res.Text,
			})
		}
		if len(res.ToolCalls) > 0 {
			parts = append(parts, buildToolCallParts()...)
		}

		respPayload := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"role":  "model",
						"parts": parts,
					},
					"finishReason": finishReason,
					"index":        0,
				},
			},
			"usageMetadata": usageMetadata,
			"modelVersion":  model,
		}
		writeJSON(w, http.StatusOK, respPayload)
		return
	}

	// Streaming response (:streamGenerateContent)
	alt := strings.ToLower(r.URL.Query().Get("alt"))
	accept := strings.ToLower(r.Header.Get("Accept"))
	useJSONArray := alt == "json" && !strings.Contains(accept, "text/event-stream")

	streamChunks := make([]map[string]interface{}, 0)
	if len(res.ToolCalls) > 0 && res.Text == "" {
		streamChunks = append(streamChunks, map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"role":  "model",
						"parts": buildToolCallParts(),
					},
					"finishReason": finishReason,
					"index":        0,
				},
			},
			"usageMetadata": usageMetadata,
			"modelVersion":  model,
		})
	} else {
		textChunks := splitIntoChunks(res.Text, res.StreamChunkSize)
		if len(textChunks) == 0 {
			textChunks = []string{""}
		}
		for i, chunkStr := range textChunks {
			isLast := i == len(textChunks)-1
			parts := []map[string]interface{}{
				{"text": chunkStr},
			}
			if isLast && len(res.ToolCalls) > 0 {
				parts = append(parts, buildToolCallParts()...)
			}
			cand := map[string]interface{}{
				"content": map[string]interface{}{
					"role":  "model",
					"parts": parts,
				},
				"index": 0,
			}
			if isLast {
				cand["finishReason"] = finishReason
			}
			chunkObj := map[string]interface{}{
				"candidates":   []map[string]interface{}{cand},
				"modelVersion": model,
			}
			if isLast {
				chunkObj["usageMetadata"] = usageMetadata
			}
			streamChunks = append(streamChunks, chunkObj)
		}
	}

	if useJSONArray {
		if res.StreamChunkDelayMs > 0 && len(streamChunks) > 1 {
			time.Sleep(time.Duration(res.StreamChunkDelayMs*(len(streamChunks)-1)) * time.Millisecond)
		}
		writeJSON(w, http.StatusOK, streamChunks)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	for i, chunkObj := range streamChunks {
		if i > 0 && res.StreamChunkDelayMs > 0 {
			time.Sleep(time.Duration(res.StreamChunkDelayMs) * time.Millisecond)
		}
		b, _ := json.Marshal(chunkObj)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}
