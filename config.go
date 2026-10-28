package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/goccy/go-yaml"
)

var InitialConfigFile *ProviderConfigFile
var ConfigPathFlag string

func init() {
	flag.StringVar(&ConfigPathFlag, "config-path", "", "Path to configuration file")
	flag.StringVar(&ConfigPathFlag, "c", "", "Path to configuration file (shorthand)")
}

func LoadConfig() *ProviderConfigFile {
	config := new(ProviderConfigFile)

	configPath := ConfigPathFlag
	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
	}

	if configPath == "" {
		configPath = "/config.yaml"
	}

	if data, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(data, config); err != nil {
			log.Printf("Error parsing config file: %v", err)
		}
	} else {
		log.Printf("Error reading config file: %v", err)
	}

	if portEnv := os.Getenv("PORT"); portEnv != "" {
		if p, err := strconv.Atoi(portEnv); err == nil {
			config.Port = p
		}
	}

	if config.Port == 0 {
		config.Port = 8080
	}

	if config.AllowedOrigins == "" {
		config.AllowedOrigins = "*"
	}

	if config.DefaultResponse == "" && len(config.DefaultResponses) == 0 {
		config.DefaultResponse = "I am a local AI provider default response."
	}

	if config.SelectionStrategy == "" {
		config.SelectionStrategy = "sequential"
	}

	if config.StreamChunkSize <= 0 {
		config.StreamChunkSize = 20
	}

	for i := range config.Models {
		config.Models[i].Object = "model"
		if config.Models[i].Created == 0 {
			config.Models[i].Created = 1715367049
		}
		if config.Models[i].OwnedBy == "" {
			config.Models[i].OwnedBy = "local"
		}
	}

	for i := range config.Matchers {
		if config.Matchers[i].Id == "" {
			config.Matchers[i].Id = fmt.Sprintf("matcher_%d", i+1)
		}
		config.Matchers[i].Order = i
		for j := range config.Matchers[i].Response.ToolCalls {
			if config.Matchers[i].Response.ToolCalls[j].Id == "" {
				config.Matchers[i].Response.ToolCalls[j].Id = fmt.Sprintf("call_%d", j+1)
			}
			if config.Matchers[i].Response.ToolCalls[j].Arguments == "" {
				config.Matchers[i].Response.ToolCalls[j].Arguments = "{}"
			}
		}
	}

	return config
}

func buildRuntimeConfig(file *ProviderConfigFile) RuntimeConfig {
	defaultResponses := make([]string, 0)
	if len(file.DefaultResponses) > 0 {
		defaultResponses = append(defaultResponses, file.DefaultResponses...)
	}

	simErr := SimulatedErrorConfig{
		Enabled:    false,
		StatusCode: 500,
		Message:    "",
		Type:       "api_error",
		Code:       "",
	}
	if file.SimulatedError.Enabled != nil {
		simErr.Enabled = *file.SimulatedError.Enabled
	}
	if file.SimulatedError.StatusCode != nil && *file.SimulatedError.StatusCode > 0 {
		simErr.StatusCode = *file.SimulatedError.StatusCode
	}
	if file.SimulatedError.Message != nil {
		simErr.Message = *file.SimulatedError.Message
	}
	if file.SimulatedError.Type != nil && *file.SimulatedError.Type != "" {
		simErr.Type = *file.SimulatedError.Type
	}
	if file.SimulatedError.Code != nil {
		simErr.Code = *file.SimulatedError.Code
	}

	features := FeaturesConfig{
		ChatCompletionsEnabled: true,
		ResponsesEnabled:       true,
		VertexEnabled:          true,
		StrictModels:           false,
		RequireAuth:            false,
	}
	if file.Features.ChatCompletionsEnabled != nil {
		features.ChatCompletionsEnabled = *file.Features.ChatCompletionsEnabled
	}
	if file.Features.ResponsesEnabled != nil {
		features.ResponsesEnabled = *file.Features.ResponsesEnabled
	}
	if file.Features.VertexEnabled != nil {
		features.VertexEnabled = *file.Features.VertexEnabled
	}
	if file.Features.StrictModels != nil {
		features.StrictModels = *file.Features.StrictModels
	}
	if file.Features.RequireAuth != nil {
		features.RequireAuth = *file.Features.RequireAuth
	}

	return RuntimeConfig{
		Port:               file.Port,
		AllowedOrigins:     file.AllowedOrigins,
		DefaultResponse:    file.DefaultResponse,
		DefaultResponses:   defaultResponses,
		SelectionStrategy:  file.SelectionStrategy,
		DelayMs:            file.DelayMs,
		StreamChunkDelayMs: file.StreamChunkDelayMs,
		StreamChunkSize:    file.StreamChunkSize,
		SimulatedError:     simErr,
		Features:           features,
	}
}
