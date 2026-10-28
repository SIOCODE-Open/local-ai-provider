package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func GET_testing_requests(w http.ResponseWriter, r *http.Request) {
	modelFilter := r.URL.Query().Get("model")
	apiTypeFilter := r.URL.Query().Get("api_type")
	ruleIdFilter := r.URL.Query().Get("matched_rule_id")
	limitStr := r.URL.Query().Get("limit")

	limit := 0
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	AppContext.mu.RLock()
	filtered := make([]RecordedRequest, 0, len(AppContext.RecordedRequests))
	for _, req := range AppContext.RecordedRequests {
		if modelFilter != "" && req.Model != modelFilter {
			continue
		}
		if apiTypeFilter != "" && !strings.EqualFold(req.ApiType, apiTypeFilter) {
			continue
		}
		if ruleIdFilter != "" && req.MatchedRuleId != ruleIdFilter {
			continue
		}
		filtered = append(filtered, req)
	}
	AppContext.mu.RUnlock()

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"requests": filtered,
		"count":    len(filtered),
	})
}

func GET_testing_requests_id(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	AppContext.mu.RLock()
	var found *RecordedRequest
	for i := range AppContext.RecordedRequests {
		if AppContext.RecordedRequests[i].Id == id {
			reqCopy := AppContext.RecordedRequests[i]
			found = &reqCopy
			break
		}
	}
	AppContext.mu.RUnlock()

	if found == nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"error":   "Recorded request not found",
			"message": "Recorded request not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, found)
}

func DELETE_testing_requests(w http.ResponseWriter, r *http.Request) {
	AppContext.mu.Lock()
	AppContext.RecordedRequests = make([]RecordedRequest, 0)
	AppContext.RequestIdCounter = 0
	AppContext.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Recorded requests cleared",
	})
}

func GET_testing_matchers(w http.ResponseWriter, r *http.Request) {
	AppContext.mu.RLock()
	matchers := make([]Matcher, len(AppContext.Matchers))
	for i, m := range AppContext.Matchers {
		matchers[i] = cloneMatcher(m)
	}
	AppContext.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"matchers": matchers,
	})
}

func GET_testing_matchers_id(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	AppContext.mu.RLock()
	var found *Matcher
	for i := range AppContext.Matchers {
		if AppContext.Matchers[i].Id == id {
			m := cloneMatcher(AppContext.Matchers[i])
			found = &m
			break
		}
	}
	AppContext.mu.RUnlock()

	if found == nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"error":   "Matcher not found",
			"message": "Matcher not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, found)
}

func PUT_testing_matchers_id(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "Failed to read request body",
			"message": "Failed to read request body",
		})
		return
	}

	var matcher Matcher
	if err := json.Unmarshal(bodyBytes, &matcher); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "Invalid JSON body",
			"message": "Invalid JSON body",
		})
		return
	}

	if err := validateMatchCriteriaRegex(matcher.Match); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   err.Error(),
			"message": err.Error(),
		})
		return
	}

	matcher.Id = id
	for i := range matcher.Response.ToolCalls {
		if matcher.Response.ToolCalls[i].Id == "" {
			matcher.Response.ToolCalls[i].Id = fmt.Sprintf("call_%d", i+1)
		}
		if matcher.Response.ToolCalls[i].Arguments == "" {
			matcher.Response.ToolCalls[i].Arguments = "{}"
		}
	}

	AppContext.mu.Lock()
	existingIdx := -1
	for i := range AppContext.Matchers {
		if AppContext.Matchers[i].Id == id {
			existingIdx = i
			break
		}
	}

	status := http.StatusCreated
	if existingIdx >= 0 {
		matcher.Order = AppContext.Matchers[existingIdx].Order
		AppContext.Matchers[existingIdx] = cloneMatcher(matcher)
		status = http.StatusOK
	} else {
		matcher.Order = AppContext.NextMatcherOrder
		AppContext.NextMatcherOrder++
		AppContext.Matchers = append(AppContext.Matchers, cloneMatcher(matcher))
	}
	AppContext.MatcherCounters[id] = 0
	AppContext.sortMatchersLocked()
	AppContext.mu.Unlock()

	writeJSON(w, status, matcher)
}

func DELETE_testing_matchers_id(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	AppContext.mu.Lock()
	foundIdx := -1
	for i := range AppContext.Matchers {
		if AppContext.Matchers[i].Id == id {
			foundIdx = i
			break
		}
	}
	if foundIdx >= 0 {
		AppContext.Matchers = append(AppContext.Matchers[:foundIdx], AppContext.Matchers[foundIdx+1:]...)
		delete(AppContext.MatcherCounters, id)
	}
	AppContext.mu.Unlock()

	if foundIdx < 0 {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"error":   "Matcher not found",
			"message": "Matcher not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Matcher deleted",
	})
}

func PUT_testing_models_id(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "Failed to read request body",
			"message": "Failed to read request body",
		})
		return
	}

	var model ModelObject
	if len(strings.TrimSpace(string(bodyBytes))) > 0 {
		if err := json.Unmarshal(bodyBytes, &model); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"error":   "Invalid JSON body",
				"message": "Invalid JSON body",
			})
			return
		}
	}

	model.Id = id
	model.Object = "model"
	if model.Created == 0 {
		model.Created = time.Now().Unix()
	}
	if model.OwnedBy == "" {
		model.OwnedBy = "local"
	}

	AppContext.mu.Lock()
	existingIdx := -1
	for i := range AppContext.Models {
		if AppContext.Models[i].Id == id {
			existingIdx = i
			break
		}
	}
	status := http.StatusCreated
	if existingIdx >= 0 {
		AppContext.Models[existingIdx] = model
		status = http.StatusOK
	} else {
		AppContext.Models = append(AppContext.Models, model)
	}
	AppContext.mu.Unlock()

	writeJSON(w, status, model)
}

func DELETE_testing_models_id(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	AppContext.mu.Lock()
	foundIdx := -1
	for i := range AppContext.Models {
		if AppContext.Models[i].Id == id {
			foundIdx = i
			break
		}
	}
	if foundIdx >= 0 {
		AppContext.Models = append(AppContext.Models[:foundIdx], AppContext.Models[foundIdx+1:]...)
	}
	AppContext.mu.Unlock()

	if foundIdx < 0 {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"error":   "Model not found",
			"message": "Model not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Model deleted",
	})
}

func GET_testing_config(w http.ResponseWriter, r *http.Request) {
	AppContext.mu.RLock()
	cfg := AppContext.Config
	cfgCopy := cfg
	cfgCopy.DefaultResponses = make([]string, len(cfg.DefaultResponses))
	copy(cfgCopy.DefaultResponses, cfg.DefaultResponses)
	AppContext.mu.RUnlock()

	writeJSON(w, http.StatusOK, cfgCopy)
}

func PATCH_testing_config(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "Failed to read request body",
			"message": "Failed to read request body",
		})
		return
	}

	var patch PatchRuntimeConfigRequest
	if err := json.Unmarshal(bodyBytes, &patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "Invalid JSON body",
			"message": "Invalid JSON body",
		})
		return
	}

	AppContext.mu.Lock()
	if patch.DefaultResponse != nil {
		AppContext.Config.DefaultResponse = *patch.DefaultResponse
		AppContext.DefaultCounter = 0
	}
	if patch.DefaultResponses != nil {
		respSlice := make([]string, len(*patch.DefaultResponses))
		copy(respSlice, *patch.DefaultResponses)
		AppContext.Config.DefaultResponses = respSlice
		AppContext.DefaultCounter = 0
	}
	if patch.SelectionStrategy != nil && *patch.SelectionStrategy != "" {
		AppContext.Config.SelectionStrategy = *patch.SelectionStrategy
		AppContext.DefaultCounter = 0
	}
	if patch.DelayMs != nil {
		AppContext.Config.DelayMs = *patch.DelayMs
	}
	if patch.StreamChunkDelayMs != nil {
		AppContext.Config.StreamChunkDelayMs = *patch.StreamChunkDelayMs
	}
	if patch.StreamChunkSize != nil && *patch.StreamChunkSize > 0 {
		AppContext.Config.StreamChunkSize = *patch.StreamChunkSize
	}
	if patch.SimulatedError != nil {
		if patch.SimulatedError.Enabled != nil {
			AppContext.Config.SimulatedError.Enabled = *patch.SimulatedError.Enabled
		}
		if patch.SimulatedError.StatusCode != nil && *patch.SimulatedError.StatusCode > 0 {
			AppContext.Config.SimulatedError.StatusCode = *patch.SimulatedError.StatusCode
		}
		if patch.SimulatedError.Message != nil {
			AppContext.Config.SimulatedError.Message = *patch.SimulatedError.Message
		}
		if patch.SimulatedError.Type != nil {
			AppContext.Config.SimulatedError.Type = *patch.SimulatedError.Type
		}
		if patch.SimulatedError.Code != nil {
			AppContext.Config.SimulatedError.Code = *patch.SimulatedError.Code
		}
	}
	if patch.Features != nil {
		if patch.Features.ChatCompletionsEnabled != nil {
			AppContext.Config.Features.ChatCompletionsEnabled = *patch.Features.ChatCompletionsEnabled
		}
		if patch.Features.ResponsesEnabled != nil {
			AppContext.Config.Features.ResponsesEnabled = *patch.Features.ResponsesEnabled
		}
		if patch.Features.VertexEnabled != nil {
			AppContext.Config.Features.VertexEnabled = *patch.Features.VertexEnabled
		}
		if patch.Features.StrictModels != nil {
			AppContext.Config.Features.StrictModels = *patch.Features.StrictModels
		}
		if patch.Features.RequireAuth != nil {
			AppContext.Config.Features.RequireAuth = *patch.Features.RequireAuth
		}
	}

	cfgCopy := AppContext.Config
	cfgCopy.DefaultResponses = make([]string, len(AppContext.Config.DefaultResponses))
	copy(cfgCopy.DefaultResponses, AppContext.Config.DefaultResponses)
	AppContext.mu.Unlock()

	writeJSON(w, http.StatusOK, cfgCopy)
}

func POST_testing_reset(w http.ResponseWriter, r *http.Request) {
	AppContext.Reset(InitialConfigFile)
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "State reset to initial configuration",
	})
}
