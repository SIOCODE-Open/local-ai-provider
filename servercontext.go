package main

import (
	"sort"
	"sync"
)

type AppServerContext struct {
	mu                sync.RWMutex
	Config            RuntimeConfig
	Models            []ModelObject
	Matchers          []Matcher
	MatcherCounters   map[string]int
	DefaultCounter    int
	NextMatcherOrder  int
	RecordedRequests  []RecordedRequest
	RequestIdCounter  int
	ChatCmplIdCounter int
	ResponseIdCounter int
	MessageIdCounter  int
}

var AppContext *AppServerContext

func NewAppContext(file *ProviderConfigFile) *AppServerContext {
	ctx := &AppServerContext{}
	ctx.Reset(file)
	return ctx
}

func (ctx *AppServerContext) Reset(file *ProviderConfigFile) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	ctx.Config = buildRuntimeConfig(file)

	ctx.Models = make([]ModelObject, len(file.Models))
	copy(ctx.Models, file.Models)

	ctx.Matchers = make([]Matcher, len(file.Matchers))
	for i, m := range file.Matchers {
		cloned := cloneMatcher(m)
		cloned.Order = i
		ctx.Matchers[i] = cloned
	}
	ctx.sortMatchersLocked()

	ctx.MatcherCounters = make(map[string]int)
	ctx.DefaultCounter = 0
	ctx.NextMatcherOrder = len(file.Matchers)
	ctx.RecordedRequests = make([]RecordedRequest, 0)
	ctx.RequestIdCounter = 0
	ctx.ChatCmplIdCounter = 0
	ctx.ResponseIdCounter = 0
	ctx.MessageIdCounter = 0
}

func cloneMatcher(m Matcher) Matcher {
	cloned := m
	if m.Match.HasTools != nil {
		v := *m.Match.HasTools
		cloned.Match.HasTools = &v
	}
	if m.Match.HasToolOutput != nil {
		v := *m.Match.HasToolOutput
		cloned.Match.HasToolOutput = &v
	}
	if m.Match.Stream != nil {
		v := *m.Match.Stream
		cloned.Match.Stream = &v
	}
	if m.Match.HeaderMatch != nil {
		cloned.Match.HeaderMatch = make(map[string]string, len(m.Match.HeaderMatch))
		for k, v := range m.Match.HeaderMatch {
			cloned.Match.HeaderMatch[k] = v
		}
	}
	if m.Response.Responses != nil {
		cloned.Response.Responses = make([]string, len(m.Response.Responses))
		copy(cloned.Response.Responses, m.Response.Responses)
	}
	if m.Response.ToolCalls != nil {
		cloned.Response.ToolCalls = make([]ToolCallConfig, len(m.Response.ToolCalls))
		copy(cloned.Response.ToolCalls, m.Response.ToolCalls)
	}
	return cloned
}

func (ctx *AppServerContext) sortMatchersLocked() {
	sort.SliceStable(ctx.Matchers, func(i, j int) bool {
		if ctx.Matchers[i].Priority != ctx.Matchers[j].Priority {
			return ctx.Matchers[i].Priority > ctx.Matchers[j].Priority
		}
		return ctx.Matchers[i].Order < ctx.Matchers[j].Order
	})
}
