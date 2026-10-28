package main

import (
	"encoding/json"

	"github.com/goccy/go-yaml"
)

type ModelObject struct {
	Id      string `json:"id" yaml:"id"`
	Object  string `json:"object" yaml:"object"`
	Created int64  `json:"created" yaml:"created"`
	OwnedBy string `json:"owned_by" yaml:"owned_by"`
}

type SimulatedErrorConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	StatusCode int    `json:"status_code" yaml:"status_code"`
	Message    string `json:"message" yaml:"message"`
	Type       string `json:"type" yaml:"type"`
	Code       string `json:"code" yaml:"code"`
}

type FeaturesConfig struct {
	ChatCompletionsEnabled bool `json:"chat_completions_enabled" yaml:"chat_completions_enabled"`
	ResponsesEnabled       bool `json:"responses_enabled" yaml:"responses_enabled"`
	VertexEnabled          bool `json:"vertex_enabled" yaml:"vertex_enabled"`
	StrictModels           bool `json:"strict_models" yaml:"strict_models"`
	RequireAuth            bool `json:"require_auth" yaml:"require_auth"`
}

type RawFeaturesConfig struct {
	ChatCompletionsEnabled *bool `json:"chat_completions_enabled,omitempty" yaml:"chat_completions_enabled,omitempty"`
	ResponsesEnabled       *bool `json:"responses_enabled,omitempty" yaml:"responses_enabled,omitempty"`
	VertexEnabled          *bool `json:"vertex_enabled,omitempty" yaml:"vertex_enabled,omitempty"`
	StrictModels           *bool `json:"strict_models,omitempty" yaml:"strict_models,omitempty"`
	RequireAuth            *bool `json:"require_auth,omitempty" yaml:"require_auth,omitempty"`
}

type RawSimulatedErrorConfig struct {
	Enabled    *bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	StatusCode *int    `json:"status_code,omitempty" yaml:"status_code,omitempty"`
	Message    *string `json:"message,omitempty" yaml:"message,omitempty"`
	Type       *string `json:"type,omitempty" yaml:"type,omitempty"`
	Code       *string `json:"code,omitempty" yaml:"code,omitempty"`
}

type MatchCriteria struct {
	ApiType            string            `json:"api_type,omitempty" yaml:"api_type,omitempty"`
	Model              string            `json:"model,omitempty" yaml:"model,omitempty"`
	ModelPrefix        string            `json:"model_prefix,omitempty" yaml:"model_prefix,omitempty"`
	Role               string            `json:"role,omitempty" yaml:"role,omitempty"`
	MessageExact       string            `json:"message_exact,omitempty" yaml:"message_exact,omitempty"`
	MessageContains    string            `json:"message_contains,omitempty" yaml:"message_contains,omitempty"`
	MessageRegex       string            `json:"message_regex,omitempty" yaml:"message_regex,omitempty"`
	AnyMessageContains string            `json:"any_message_contains,omitempty" yaml:"any_message_contains,omitempty"`
	SystemContains     string            `json:"system_contains,omitempty" yaml:"system_contains,omitempty"`
	SystemRegex        string            `json:"system_regex,omitempty" yaml:"system_regex,omitempty"`
	HasTools           *bool             `json:"has_tools,omitempty" yaml:"has_tools,omitempty"`
	ToolName           string            `json:"tool_name,omitempty" yaml:"tool_name,omitempty"`
	ToolChoice         string            `json:"tool_choice,omitempty" yaml:"tool_choice,omitempty"`
	HasToolOutput      *bool             `json:"has_tool_output,omitempty" yaml:"has_tool_output,omitempty"`
	ToolCallId         string            `json:"tool_call_id,omitempty" yaml:"tool_call_id,omitempty"`
	ToolOutputContains string            `json:"tool_output_contains,omitempty" yaml:"tool_output_contains,omitempty"`
	ToolOutputRegex    string            `json:"tool_output_regex,omitempty" yaml:"tool_output_regex,omitempty"`
	Stream             *bool             `json:"stream,omitempty" yaml:"stream,omitempty"`
	HeaderMatch        map[string]string `json:"header_match,omitempty" yaml:"header_match,omitempty"`
}

type ToolCallConfig struct {
	Id        string `json:"id" yaml:"id"`
	Name      string `json:"name" yaml:"name"`
	Arguments string `json:"arguments" yaml:"arguments"`
}

type rawToolCallConfig struct {
	Id        string      `json:"id" yaml:"id"`
	Name      string      `json:"name" yaml:"name"`
	Arguments interface{} `json:"arguments" yaml:"arguments"`
}

func convertArgumentsToString(args interface{}) string {
	if args == nil {
		return "{}"
	}
	if s, ok := args.(string); ok {
		return s
	}
	b, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (tc *ToolCallConfig) UnmarshalJSON(data []byte) error {
	var raw rawToolCallConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	tc.Id = raw.Id
	tc.Name = raw.Name
	tc.Arguments = convertArgumentsToString(raw.Arguments)
	return nil
}

func (tc *ToolCallConfig) UnmarshalYAML(data []byte) error {
	var raw rawToolCallConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	tc.Id = raw.Id
	tc.Name = raw.Name
	tc.Arguments = convertArgumentsToString(raw.Arguments)
	return nil
}

type ResponseConfig struct {
	Text               string           `json:"text,omitempty" yaml:"text,omitempty"`
	Responses          []string         `json:"responses,omitempty" yaml:"responses,omitempty"`
	SelectionStrategy  string           `json:"selection_strategy,omitempty" yaml:"selection_strategy,omitempty"`
	ReasoningContent   string           `json:"reasoning_content,omitempty" yaml:"reasoning_content,omitempty"`
	ToolCalls          []ToolCallConfig `json:"tool_calls,omitempty" yaml:"tool_calls,omitempty"`
	FinishReason       string           `json:"finish_reason,omitempty" yaml:"finish_reason,omitempty"`
	DelayMs            int              `json:"delay_ms,omitempty" yaml:"delay_ms,omitempty"`
	StreamChunkDelayMs int              `json:"stream_chunk_delay_ms,omitempty" yaml:"stream_chunk_delay_ms,omitempty"`
	StreamChunkSize    int              `json:"stream_chunk_size,omitempty" yaml:"stream_chunk_size,omitempty"`
	StatusCode         int              `json:"status_code,omitempty" yaml:"status_code,omitempty"`
	ErrorMessage       string           `json:"error_message,omitempty" yaml:"error_message,omitempty"`
	ErrorType          string           `json:"error_type,omitempty" yaml:"error_type,omitempty"`
	ErrorCode          string           `json:"error_code,omitempty" yaml:"error_code,omitempty"`
}

type Matcher struct {
	Id       string         `json:"id" yaml:"id"`
	Priority int            `json:"priority" yaml:"priority"`
	Disabled bool           `json:"disabled" yaml:"disabled"`
	Match    MatchCriteria  `json:"match" yaml:"match"`
	Response ResponseConfig `json:"response" yaml:"response"`
	Order    int            `json:"-" yaml:"-"`
}

type ProviderConfigFile struct {
	Port               int                     `json:"port" yaml:"port"`
	AllowedOrigins     string                  `json:"allowed_origins,omitempty" yaml:"allowed_origins,omitempty"`
	DefaultResponse    string                  `json:"default_response,omitempty" yaml:"default_response,omitempty"`
	DefaultResponses   []string                `json:"default_responses,omitempty" yaml:"default_responses,omitempty"`
	SelectionStrategy  string                  `json:"selection_strategy,omitempty" yaml:"selection_strategy,omitempty"`
	DelayMs            int                     `json:"delay_ms,omitempty" yaml:"delay_ms,omitempty"`
	StreamChunkDelayMs int                     `json:"stream_chunk_delay_ms,omitempty" yaml:"stream_chunk_delay_ms,omitempty"`
	StreamChunkSize    int                     `json:"stream_chunk_size,omitempty" yaml:"stream_chunk_size,omitempty"`
	SimulatedError     RawSimulatedErrorConfig `json:"simulated_error,omitempty" yaml:"simulated_error,omitempty"`
	Features           RawFeaturesConfig       `json:"features,omitempty" yaml:"features,omitempty"`
	Models             []ModelObject           `json:"models,omitempty" yaml:"models,omitempty"`
	Matchers           []Matcher               `json:"matchers,omitempty" yaml:"matchers,omitempty"`
}

type RuntimeConfig struct {
	Port               int                  `json:"port"`
	AllowedOrigins     string               `json:"-"`
	DefaultResponse    string               `json:"default_response"`
	DefaultResponses   []string             `json:"default_responses"`
	SelectionStrategy  string               `json:"selection_strategy"`
	DelayMs            int                  `json:"delay_ms"`
	StreamChunkDelayMs int                  `json:"stream_chunk_delay_ms"`
	StreamChunkSize    int                  `json:"stream_chunk_size"`
	SimulatedError     SimulatedErrorConfig `json:"simulated_error"`
	Features           FeaturesConfig       `json:"features"`
}

type PatchRuntimeConfigRequest struct {
	DefaultResponse    *string                  `json:"default_response,omitempty"`
	DefaultResponses   *[]string                `json:"default_responses,omitempty"`
	SelectionStrategy  *string                  `json:"selection_strategy,omitempty"`
	DelayMs            *int                     `json:"delay_ms,omitempty"`
	StreamChunkDelayMs *int                     `json:"stream_chunk_delay_ms,omitempty"`
	StreamChunkSize    *int                     `json:"stream_chunk_size,omitempty"`
	SimulatedError     *RawSimulatedErrorConfig `json:"simulated_error,omitempty"`
	Features           *RawFeaturesConfig       `json:"features,omitempty"`
}

type NormalizedMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	Name       string           `json:"name,omitempty"`
	ToolCalls  []ToolCallConfig `json:"tool_calls,omitempty"`
	ToolCallId string           `json:"tool_call_id,omitempty"`
}

type NormalizedToolOutput struct {
	ToolCallId string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Content    string `json:"content"`
	Output     string `json:"output"`
}

type RecordedRequest struct {
	Id                string                 `json:"id"`
	Timestamp         string                 `json:"timestamp"`
	ApiType           string                 `json:"api_type"`
	Method            string                 `json:"method"`
	Path              string                 `json:"path"`
	Headers           map[string]string      `json:"headers"`
	Model             string                 `json:"model"`
	Stream            bool                   `json:"stream"`
	SystemPrompt      string                 `json:"system_prompt"`
	Messages          []NormalizedMessage    `json:"messages"`
	LastUserMessage   string                 `json:"last_user_message"`
	Tools             []string               `json:"tools"`
	ToolChoice        string                 `json:"tool_choice"`
	ToolOutputs       []NormalizedToolOutput `json:"tool_outputs"`
	RawBody           interface{}            `json:"raw_body"`
	MatchedRuleId     string                 `json:"matched_rule_id"`
	ResponseStatus    int                    `json:"response_status"`
	ResponseText      string                 `json:"response_text"`
	ResponseToolCalls []ToolCallConfig       `json:"response_tool_calls"`
}
