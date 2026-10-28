package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type EvaluationResult struct {
	MatchedRuleId      string
	Text               string
	ReasoningContent   string
	ToolCalls          []ToolCallConfig
	FinishReason       string
	DelayMs            int
	StreamChunkDelayMs int
	StreamChunkSize    int
	StatusCode         int
	ErrorMessage       string
	ErrorType          string
	ErrorCode          string
}

func validateMatchCriteriaRegex(mc MatchCriteria) error {
	if mc.MessageRegex != "" {
		if _, err := regexp.Compile(mc.MessageRegex); err != nil {
			return fmt.Errorf("invalid message_regex: %w", err)
		}
	}
	if mc.SystemRegex != "" {
		if _, err := regexp.Compile(mc.SystemRegex); err != nil {
			return fmt.Errorf("invalid system_regex: %w", err)
		}
	}
	if mc.ToolOutputRegex != "" {
		if _, err := regexp.Compile(mc.ToolOutputRegex); err != nil {
			return fmt.Errorf("invalid tool_output_regex: %w", err)
		}
	}
	return nil
}

func roleMatches(msgRole string, targetRole string) bool {
	if strings.EqualFold(msgRole, targetRole) {
		return true
	}
	if (strings.EqualFold(targetRole, "assistant") || strings.EqualFold(targetRole, "model")) &&
		(strings.EqualFold(msgRole, "assistant") || strings.EqualFold(msgRole, "model")) {
		return true
	}
	return false
}

func matchesCriteria(mc MatchCriteria, req *RecordedRequest, httpHeaders http.Header) bool {
	if mc.ApiType != "" && !strings.EqualFold(mc.ApiType, req.ApiType) {
		return false
	}
	if mc.Model != "" && mc.Model != req.Model {
		return false
	}
	if mc.ModelPrefix != "" && !strings.HasPrefix(req.Model, mc.ModelPrefix) {
		return false
	}
	if mc.Stream != nil && *mc.Stream != req.Stream {
		return false
	}

	// Determine target message and verify role if specified
	targetMessage := req.LastUserMessage
	if targetMessage == "" && len(req.Messages) > 0 {
		targetMessage = req.Messages[len(req.Messages)-1].Content
	}

	if mc.Role != "" {
		foundRole := false
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if roleMatches(req.Messages[i].Role, mc.Role) {
				targetMessage = req.Messages[i].Content
				foundRole = true
				break
			}
		}
		if !foundRole && (strings.EqualFold(mc.Role, "system") || strings.EqualFold(mc.Role, "developer")) && req.SystemPrompt != "" {
			targetMessage = req.SystemPrompt
			foundRole = true
		}
		if !foundRole {
			return false
		}
	}

	if mc.MessageExact != "" && targetMessage != mc.MessageExact {
		return false
	}
	if mc.MessageContains != "" && !strings.Contains(targetMessage, mc.MessageContains) {
		return false
	}
	if mc.MessageRegex != "" {
		matched, err := regexp.MatchString(mc.MessageRegex, targetMessage)
		if err != nil || !matched {
			return false
		}
	}

	if mc.AnyMessageContains != "" {
		foundAny := false
		for _, m := range req.Messages {
			if strings.Contains(m.Content, mc.AnyMessageContains) {
				foundAny = true
				break
			}
		}
		if !foundAny && strings.Contains(req.SystemPrompt, mc.AnyMessageContains) {
			foundAny = true
		}
		if !foundAny {
			return false
		}
	}

	if mc.SystemContains != "" && !strings.Contains(req.SystemPrompt, mc.SystemContains) {
		return false
	}
	if mc.SystemRegex != "" {
		matched, err := regexp.MatchString(mc.SystemRegex, req.SystemPrompt)
		if err != nil || !matched {
			return false
		}
	}

	if mc.HasTools != nil {
		hasTools := len(req.Tools) > 0
		if hasTools != *mc.HasTools {
			return false
		}
	}

	if mc.ToolName != "" {
		foundTool := false
		for _, t := range req.Tools {
			if t == mc.ToolName {
				foundTool = true
				break
			}
		}
		if !foundTool {
			return false
		}
	}

	if mc.ToolChoice != "" && !strings.EqualFold(mc.ToolChoice, req.ToolChoice) {
		return false
	}

	if mc.HasToolOutput != nil {
		hasOut := len(req.ToolOutputs) > 0
		if hasOut != *mc.HasToolOutput {
			return false
		}
	}

	candidateOutputs := req.ToolOutputs
	if mc.ToolCallId != "" {
		filtered := make([]NormalizedToolOutput, 0)
		for _, out := range req.ToolOutputs {
			if out.ToolCallId == mc.ToolCallId || out.Name == mc.ToolCallId {
				filtered = append(filtered, out)
			}
		}
		if len(filtered) == 0 {
			return false
		}
		candidateOutputs = filtered
	}

	if mc.ToolOutputContains != "" {
		foundOut := false
		for _, out := range candidateOutputs {
			if strings.Contains(out.Content, mc.ToolOutputContains) {
				foundOut = true
				break
			}
		}
		if !foundOut {
			return false
		}
	}

	if mc.ToolOutputRegex != "" {
		foundOut := false
		for _, out := range candidateOutputs {
			matched, err := regexp.MatchString(mc.ToolOutputRegex, out.Content)
			if err == nil && matched {
				foundOut = true
				break
			}
		}
		if !foundOut {
			return false
		}
	}

	if len(mc.HeaderMatch) > 0 {
		for k, expectedVal := range mc.HeaderMatch {
			actualVal := httpHeaders.Get(k)
			if actualVal != expectedVal {
				return false
			}
		}
	}

	return true
}

func selectFromCandidates(candidates []string, strategy string, counter *int) string {
	if len(candidates) == 0 {
		return ""
	}
	switch strings.ToLower(strategy) {
	case "random":
		return candidates[rand.Intn(len(candidates))]
	case "first":
		return candidates[0]
	default: // "sequential"
		idx := *counter % len(candidates)
		*counter = *counter + 1
		return candidates[idx]
	}
}

func evaluateAndRecordRequest(req *RecordedRequest, httpHeaders http.Header) EvaluationResult {
	AppContext.mu.Lock()

	AppContext.RequestIdCounter++
	req.Id = fmt.Sprintf("req_%d", AppContext.RequestIdCounter)
	req.Timestamp = time.Now().UTC().Format(time.RFC3339)

	cfg := AppContext.Config
	res := EvaluationResult{
		MatchedRuleId:      "__default__",
		DelayMs:            cfg.DelayMs,
		StreamChunkDelayMs: cfg.StreamChunkDelayMs,
		StreamChunkSize:    cfg.StreamChunkSize,
		StatusCode:         http.StatusOK,
	}
	if res.StreamChunkSize <= 0 {
		res.StreamChunkSize = 20
	}

	var matched *Matcher
	for i := range AppContext.Matchers {
		m := &AppContext.Matchers[i]
		if m.Disabled {
			continue
		}
		if matchesCriteria(m.Match, req, httpHeaders) {
			matched = m
			break
		}
	}

	if matched != nil {
		res.MatchedRuleId = matched.Id
		rc := matched.Response

		if len(rc.Responses) > 0 {
			strat := rc.SelectionStrategy
			if strat == "" {
				strat = cfg.SelectionStrategy
			}
			c := AppContext.MatcherCounters[matched.Id]
			res.Text = selectFromCandidates(rc.Responses, strat, &c)
			AppContext.MatcherCounters[matched.Id] = c
		} else {
			res.Text = rc.Text
		}

		res.ReasoningContent = rc.ReasoningContent
		if len(rc.ToolCalls) > 0 {
			res.ToolCalls = make([]ToolCallConfig, len(rc.ToolCalls))
			for i, tc := range rc.ToolCalls {
				if tc.Id == "" {
					tc.Id = fmt.Sprintf("call_%d", i+1)
				}
				if tc.Arguments == "" {
					tc.Arguments = "{}"
				}
				res.ToolCalls[i] = tc
			}
		}

		res.FinishReason = rc.FinishReason
		if res.FinishReason == "" {
			if len(res.ToolCalls) > 0 {
				res.FinishReason = "tool_calls"
			} else {
				res.FinishReason = "stop"
			}
		}

		if rc.DelayMs > 0 {
			res.DelayMs = rc.DelayMs
		}
		if rc.StreamChunkDelayMs > 0 {
			res.StreamChunkDelayMs = rc.StreamChunkDelayMs
		}
		if rc.StreamChunkSize > 0 {
			res.StreamChunkSize = rc.StreamChunkSize
		}
		if rc.StatusCode >= 400 {
			res.StatusCode = rc.StatusCode
			res.ErrorMessage = rc.ErrorMessage
			if res.ErrorMessage == "" {
				res.ErrorMessage = "Simulated error from matcher " + matched.Id
			}
			res.ErrorType = rc.ErrorType
			if res.ErrorType == "" {
				res.ErrorType = "api_error"
			}
			res.ErrorCode = rc.ErrorCode
		}
	} else {
		if len(cfg.DefaultResponses) > 0 {
			res.Text = selectFromCandidates(cfg.DefaultResponses, cfg.SelectionStrategy, &AppContext.DefaultCounter)
		} else {
			res.Text = cfg.DefaultResponse
		}
		res.FinishReason = "stop"
	}

	// Global simulated error applies if enabled and matcher didn't already set a specific error
	if cfg.SimulatedError.Enabled && res.StatusCode < 400 {
		res.StatusCode = cfg.SimulatedError.StatusCode
		if res.StatusCode < 400 {
			res.StatusCode = http.StatusInternalServerError
		}
		res.ErrorMessage = cfg.SimulatedError.Message
		if res.ErrorMessage == "" {
			res.ErrorMessage = "Simulated error"
		}
		res.ErrorType = cfg.SimulatedError.Type
		if res.ErrorType == "" {
			res.ErrorType = "api_error"
		}
		res.ErrorCode = cfg.SimulatedError.Code
	}

	req.MatchedRuleId = res.MatchedRuleId
	req.ResponseStatus = res.StatusCode
	if res.StatusCode < 400 {
		req.ResponseText = res.Text
		if len(res.ToolCalls) > 0 {
			req.ResponseToolCalls = make([]ToolCallConfig, len(res.ToolCalls))
			copy(req.ResponseToolCalls, res.ToolCalls)
		} else {
			req.ResponseToolCalls = make([]ToolCallConfig, 0)
		}
	} else {
		req.ResponseText = ""
		req.ResponseToolCalls = make([]ToolCallConfig, 0)
	}

	AppContext.RecordedRequests = append(AppContext.RecordedRequests, *req)
	AppContext.mu.Unlock()

	if res.DelayMs > 0 {
		time.Sleep(time.Duration(res.DelayMs) * time.Millisecond)
	}

	return res
}

func modelExists(modelId string) bool {
	AppContext.mu.RLock()
	defer AppContext.mu.RUnlock()
	for _, m := range AppContext.Models {
		if m.Id == modelId {
			return true
		}
	}
	return false
}

func nextChatCompletionId() string {
	AppContext.mu.Lock()
	defer AppContext.mu.Unlock()
	AppContext.ChatCmplIdCounter++
	return fmt.Sprintf("chatcmpl-%d", AppContext.ChatCmplIdCounter)
}

func nextResponseIds() (string, string) {
	AppContext.mu.Lock()
	defer AppContext.mu.Unlock()
	AppContext.ResponseIdCounter++
	AppContext.MessageIdCounter++
	return fmt.Sprintf("resp_%d", AppContext.ResponseIdCounter), fmt.Sprintf("msg_%d", AppContext.MessageIdCounter)
}

func splitIntoChunks(text string, chunkSize int) []string {
	if chunkSize <= 0 {
		chunkSize = 20
	}
	if text == "" {
		return []string{}
	}
	runes := []rune(text)
	chunks := make([]string, 0, (len(runes)+chunkSize-1)/chunkSize)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

func estimateTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 1
	}
	n := (len([]rune(trimmed)) + 3) / 4
	if n < 1 {
		return 1
	}
	return n
}

func extractRequestHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string, len(r.Header))
	for k, vals := range r.Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	return headers
}
