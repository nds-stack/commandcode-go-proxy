package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/nds-stack/commandcode-go-proxy/internal/api"
	"github.com/nds-stack/commandcode-go-proxy/internal/version"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const defaultBaseURL = "https://api.commandcode.ai"
const defaultTimeout = 600 * time.Second
const maxRetries = 10
const retryBaseDelay = 500 * time.Millisecond
const retryMaxDelay = 30 * time.Second
const retryRateLimitBase = 5 * time.Second
const retryRateLimitMax = 60 * time.Second
const streamIdleTimeout = 2 * time.Minute
const debugLogLimit = 20000

func truncateLog(s string) string {
	if len(s) <= debugLogLimit {
		return s
	}
	return s[:debugLogLimit] + fmt.Sprintf("... [truncated %d bytes]", len(s)-debugLogLimit)
}

func retryDelay(attempt int) time.Duration {
	d := retryBaseDelay * time.Duration(1<<(attempt-1))
	if d > retryMaxDelay {
		d = retryMaxDelay
	}
	return d
}

func retryDelayRateLimit(attempt int) time.Duration {
	d := retryRateLimitBase * time.Duration(1<<(attempt-1))
	if d > retryRateLimitMax {
		d = retryRateLimitMax
	}
	return d
}

// getSessionID finds a session ID from client headers
// Priority: x-session-id > x-session-affinity > hash of static headers > generated
func getSessionID(clientHeaders http.Header, apiKey string) string {
	// 1. Check specific headers
	if id := clientHeaders.Get("X-Session-Id"); id != "" {
		return id
	}
	if id := clientHeaders.Get("X-Session-Affinity"); id != "" {
		return id
	}

	// 2. Hash all static headers (exclude dynamic ones)
	exclude := map[string]bool{
		"content-length":  true,
		"cookie":          true,
		"cookies":         true,
		"set-cookie":      true,
		"date":            true,
		"if-modified-since": true,
		"if-none-match":   true,
	}
	var parts []string
	for name, values := range clientHeaders {
		if exclude[strings.ToLower(name)] {
			continue
		}
		for _, v := range values {
			parts = append(parts, name+"="+v)
		}
	}
	if len(parts) > 0 {
		hash := sha256.Sum256([]byte(strings.Join(parts, "|")))
		return "sess_" + fmt.Sprintf("%x", hash[:8])
	}

	// 3. Fallback: generate from API key
	hash := sha256.Sum256([]byte(apiKey))
	return "sess_" + fmt.Sprintf("%x", hash[:8])
}

func (p *Proxy) debugClientf(format string, args ...any) {
}

func (p *Proxy) debugServerf(format string, args ...any) {
}

func (p *Proxy) writeOpenAIError(w http.ResponseWriter, status int, message, errType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.OpenAIErrorResponse{Error: api.OpenAIError{
		Message: message,
		Type:    errType,
		Param:   nil,
		Code:    nil,
	}})
}

func normalizeFinishReason(reason string) string {
	switch reason {
	case "tool_calls", "tool-calls":
		return "tool_calls"
	case "length", "max_tokens":
		return "length"
	case "content_filter", "content-filter":
		return "content_filter"
	default:
		return "stop"
	}
}

// Proxy struct
type Proxy struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
	Debug   bool
}

// NewProxy creates a new proxy instance
func NewProxy(apiKey string, timeout time.Duration) *Proxy {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	transport := &http.Transport{
		MaxIdleConns:        20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	return &Proxy{
		APIKey:  apiKey,
		BaseURL: defaultBaseURL,
		Client:  &http.Client{Timeout: timeout, Transport: transport},
	}
}

// BuildRequest builds the CommandCode request body
func (p *Proxy) BuildRequest(openAIReq api.OpenAIChatRequest) (api.CCRequestBody, error) {
	model := MapModel(openAIReq.Model)
	system, msgs := ExtractSystem(openAIReq.Messages)
	ccMessages := ConvertMessages(msgs)

	temperature := 0.3
	maxTokens := 64000
	if openAIReq.Temperature != nil {
		temperature = *openAIReq.Temperature
	}
	if openAIReq.MaxTokens != nil {
		maxTokens = *openAIReq.MaxTokens
	}
	if openAIReq.MaxCompletionTokens != nil {
		maxTokens = *openAIReq.MaxCompletionTokens
	}

	tools := ConvertTools(openAIReq.Tools)

	ccBody := api.CCRequestBody{
		Config: api.CCConfig{
			WorkingDir:    ".",
			Date:          time.Now().Format("2006-01-02"),
			Environment:   "cli",
			Structure:     []string{},
			IsGitRepo:     false,
			CurrentBranch: "",
			MainBranch:    "main",
			GitStatus:     "",
			RecentCommits: []string{},
		},
		PermissionMode: "standard",
		Params: api.CCChatParams{
			Model:            model,
			Messages:         ccMessages,
			Tools:            tools,
			System:           system,
			MaxTokens:        maxTokens,
			Temperature:      temperature,
			Stream:           true,
			ReasoningEffort:  openAIReq.ReasoningEffort,
		},
	}

	return ccBody, nil
}

// CreateUpstreamRequest creates a new HTTP request to the CommandCode API
func (p *Proxy) CreateUpstreamRequest(ctx context.Context, ccBody api.CCRequestBody, apiKey string, sessionID string) (*http.Request, error) {
	reqJSON, err := json.Marshal(ccBody)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	p.debugClientf("[DEBUG] Outgoing CC request body: %s", truncateLog(string(reqJSON)))

	ccReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.BaseURL+"/alpha/generate", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}

	ccReq.Header.Set("Content-Type", "application/json")
	ccReq.Header.Set("Authorization", "Bearer "+apiKey)
	ccReq.Header.Set("x-command-code-version", version.GetCommandCodeVersion())
	ccReq.Header.Set("x-cli-environment", "production")
	ccReq.Header.Set("x-session-id", sessionID)
	ccReq.Header.Set("x-project-slug", ".")
	ccReq.Header.Set("x-taste-learning", "false")
	ccReq.Header.Set("x-co-flag", "false")
	ccReq.Header.Set("Accept", "text/event-stream")
	p.debugClientf("[DEBUG] Outgoing CC request headers:")
	for name, values := range ccReq.Header {
		p.debugClientf("[DEBUG]   %s: %s", name, strings.Join(values, ", "))
	}

	return ccReq, nil
}

// CallUpstream makes the request to CommandCode API
func (p *Proxy) CallUpstream(req *http.Request) (*http.Response, error) {
	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream error: %w", err)
	}
	return resp, nil
}

// HandleChatCompletions handles the /v1/chat/completions endpoint
func (p *Proxy) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.writeOpenAIError(w, http.StatusMethodNotAllowed, "Method not allowed", "invalid_request_error")
		return
	}

	// Get API key from client Authorization header or server default
	apiKey := r.Header.Get("Authorization")
	if apiKey != "" {
		apiKey = strings.TrimPrefix(apiKey, "Bearer ")
		apiKey = strings.TrimSpace(apiKey)
	} else if p.APIKey != "" {
		apiKey = p.APIKey
	} else {
		p.writeOpenAIError(w, http.StatusUnauthorized, "API key required. Set Authorization header.", "authentication_error")
		return
	}

	// Read request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.writeOpenAIError(w, http.StatusBadRequest, "Failed to read body", "invalid_request_error")
		return
	}

	p.debugClientf("[DEBUG] Incoming client request headers:")
	for name, values := range r.Header {
		p.debugClientf("[DEBUG]   %s: %s", name, strings.Join(values, ", "))
	}
	p.debugClientf("[DEBUG] Incoming client request body: %s", truncateLog(string(body)))

	var openAIReq api.OpenAIChatRequest
	if err := json.Unmarshal(body, &openAIReq); err != nil {
		p.writeOpenAIError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %s", err.Error()), "invalid_request_error")
		return
	}

	if len(openAIReq.Messages) == 0 {
		p.writeOpenAIError(w, http.StatusBadRequest, "messages array is required", "invalid_request_error")
		return
	}

	// Build CommandCode request
	ccBody, err := p.BuildRequest(openAIReq)
	if err != nil {
		p.writeOpenAIError(w, http.StatusInternalServerError, "Failed to build request", "server_error")
		return
	}

	// Get session ID from client or derive from API key
	sessionID := getSessionID(r.Header, apiKey)

	// Create upstream request
	ccReq, err := p.CreateUpstreamRequest(r.Context(), ccBody, apiKey, sessionID)
	if err != nil {
		p.writeOpenAIError(w, http.StatusInternalServerError, "Failed to create upstream request", "server_error")
		return
	}

	requestID := "chatcmpl-" + uuid.New().String()[:29]
	created := time.Now().Unix()

	// Call upstream with retry
	var ccResp *http.Response
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("\n  ⚠ Request failed — auto-retrying (%d/%d)...\n", attempt, maxRetries)
			log.Printf("[RETRY] Upstream call (attempt %d/%d)", attempt, maxRetries)
			time.Sleep(retryDelay(attempt))
		}

		ccReq, err = p.CreateUpstreamRequest(r.Context(), ccBody, apiKey, sessionID)
		if err != nil {
			continue
		}

		ccResp, err = p.CallUpstream(ccReq)
		if err != nil {
			log.Printf("[ERROR] Upstream call failed: %v", err)
			continue
		}

		if ccResp.StatusCode != http.StatusOK {
			errBody, _ := io.ReadAll(ccResp.Body)
			statusCode := ccResp.StatusCode
			ccResp.Body.Close()
			ccResp = nil
			message := fmt.Sprintf("Upstream error: %s", string(errBody))
			log.Printf("[ERROR] Upstream returned %d: %s", statusCode, string(errBody))
			if attempt < maxRetries {
				continue
			}
			status := http.StatusBadGateway
			if statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError {
				status = statusCode
			}
			p.writeOpenAIError(w, status, message, "api_error")
			return
		}

		break
	}

	if ccResp == nil {
		p.writeOpenAIError(w, http.StatusBadGateway, "All upstream attempts failed", "api_error")
		return
	}
	defer ccResp.Body.Close()

	p.debugServerf("[DEBUG] Upstream response status: %d", ccResp.StatusCode)
	p.debugServerf("[DEBUG] Upstream response headers:")
	for name, values := range ccResp.Header {
		p.debugServerf("[DEBUG]   %s: %s", name, strings.Join(values, ", "))
	}

	if openAIReq.Stream {
		p.StreamResponse(w, r, ccResp, requestID, ccBody.Params.Model, created, openAIReq, apiKey, sessionID)
	} else {
		p.NonStreamResponse(w, ccResp, requestID, ccBody.Params.Model, created)
	}
}

// StreamResponse handles streaming response from CommandCode to OpenAI SSE.
// Retries transparently on "Network connection lost" without the client noticing.
func (p *Proxy) StreamResponse(w http.ResponseWriter, r *http.Request, initialResp *http.Response, requestID, model string, created int64, openAIReq api.OpenAIChatRequest, apiKey string, sessionID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		p.writeOpenAIError(w, http.StatusInternalServerError, "Streaming not supported", "server_error")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	resp := initialResp
	sentRole := false
	toolCallIndex := 0
	toolCallIndexes := map[string]int{}
	var promptTokens, completionTokens int

	type decodeResult struct {
		event api.CCStreamEvent
		err   error
	}

streamLoop:
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("\n  ⚠ Connection lost or error — auto-retrying (%d/%d)...\n", attempt, maxRetries)
			log.Printf("[RETRY] Reconnecting stream (attempt %d/%d)", attempt, maxRetries)
			if r.Context().Err() != nil {
				return
			}
			time.Sleep(retryDelay(attempt))
			newBody, err := p.BuildRequest(openAIReq)
			if err != nil {
				continue
			}
			newReq, err := p.CreateUpstreamRequest(r.Context(), newBody, apiKey, sessionID)
			if err != nil {
				continue
			}
			resp.Body.Close()
			resp, err = p.CallUpstream(newReq)
			if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
				if resp != nil {
					resp.Body.Close()
				}
				continue
			}
			sentRole = false
			toolCallIndex = 0
			toolCallIndexes = map[string]int{}
		}

		decoder := json.NewDecoder(resp.Body)
		finished := false
		eventCh := make(chan decodeResult)

		go func() {
			for {
				var event api.CCStreamEvent
				err := decoder.Decode(&event)
				eventCh <- decodeResult{event, err}
				if err != nil {
					return
				}
			}
		}()

		idleTimer := time.NewTimer(streamIdleTimeout)

		for {
			select {
			case <-r.Context().Done():
				return

			case <-idleTimer.C:
				log.Printf("[WARN] Stream idle timeout after %v", streamIdleTimeout)
				if attempt < maxRetries {
					continue streamLoop
				}
				return

			case result := <-eventCh:
				if result.err != nil {
					if result.err == io.EOF {
						if !finished && attempt < maxRetries {
							log.Printf("[WARN] Stream truncated: ended without finish event")
							continue streamLoop
						}
						return
					}
					if r.Context().Err() != nil {
						return
					}
					log.Printf("[ERROR] Decode error: %v", result.err)
					if attempt < maxRetries {
						continue streamLoop
					}
					return
				}

				if !idleTimer.Stop() {
					select {
					case <-idleTimer.C:
					default:
					}
				}
				idleTimer.Reset(streamIdleTimeout)

				event := result.event

				switch event.Type {
				case "abort":
					continue

				case "reasoning-delta":
					delta := api.OpenAIDelta{ReasoningContent: event.Text}
					if !sentRole {
						delta.Role = "assistant"
						sentRole = true
					}
					p.WriteSSE(w, flusher, api.OpenAIChatResponse{
						ID:      requestID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []api.OpenAIChoice{{Index: 0, Delta: &delta}},
					})

				case "text-delta":
					delta := api.OpenAIDelta{Content: event.Text}
					if !sentRole {
						delta.Role = "assistant"
						sentRole = true
					}
					p.WriteSSE(w, flusher, api.OpenAIChatResponse{
						ID:      requestID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []api.OpenAIChoice{{Index: 0, Delta: &delta}},
					})

				case "tool-use":
					toolCalls := []api.OpenAIDeltaToolCall{{
						Index:    toolCallIndex,
						ID:       event.ToolCallID,
						Type:     "function",
						Function: &api.OpenAIDeltaFunction{Name: event.ToolName},
					}}
					delta := api.OpenAIDelta{ToolCalls: toolCalls}
					if !sentRole {
						delta.Role = "assistant"
						sentRole = true
					}
					p.WriteSSE(w, flusher, api.OpenAIChatResponse{
						ID:      requestID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []api.OpenAIChoice{{Index: 0, Delta: &delta}},
					})
					toolCallIndex++

				case "tool-delta":
					idx := toolCallIndex - 1
					if idx < 0 {
						idx = 0
					}
					toolCalls := []api.OpenAIDeltaToolCall{{
						Index:    idx,
						Function: &api.OpenAIDeltaFunction{Arguments: event.Text},
					}}
					p.WriteSSE(w, flusher, api.OpenAIChatResponse{
						ID:      requestID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []api.OpenAIChoice{{Index: 0, Delta: &api.OpenAIDelta{ToolCalls: toolCalls}}},
					})

				case "tool-input-start":
					if _, ok := toolCallIndexes[event.ID]; !ok {
						toolCallIndexes[event.ID] = toolCallIndex
						toolCallIndex++
					}
					delta := api.OpenAIDelta{ToolCalls: []api.OpenAIDeltaToolCall{{
						Index: toolCallIndexes[event.ID],
						ID:    event.ID,
						Type:  "function",
						Function: &api.OpenAIDeltaFunction{
							Name: event.ToolName,
						},
					}}}
					if !sentRole {
						delta.Role = "assistant"
						sentRole = true
					}
					p.WriteSSE(w, flusher, api.OpenAIChatResponse{
						ID:      requestID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []api.OpenAIChoice{{Index: 0, Delta: &delta}},
					})

				case "tool-input-delta":
					idx, ok := toolCallIndexes[event.ID]
					if !ok {
						idx = toolCallIndex
						toolCallIndexes[event.ID] = idx
						toolCallIndex++
					}
					p.WriteSSE(w, flusher, api.OpenAIChatResponse{
						ID:      requestID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []api.OpenAIChoice{{Index: 0, Delta: &api.OpenAIDelta{ToolCalls: []api.OpenAIDeltaToolCall{{
							Index:    idx,
							Function: &api.OpenAIDeltaFunction{Arguments: event.Delta},
						}}}}},
					})

				case "tool-call":
					if _, alreadyStreamed := toolCallIndexes[event.ToolCallID]; alreadyStreamed {
						continue
					}
					idx := toolCallIndex
					toolCallIndexes[event.ToolCallID] = idx
					toolCallIndex++
					args := ""
					if event.Input != nil {
						if data, err := json.Marshal(event.Input); err == nil {
							args = string(data)
						}
					}
					delta := api.OpenAIDelta{ToolCalls: []api.OpenAIDeltaToolCall{{
						Index: idx,
						ID:    event.ToolCallID,
						Type:  "function",
						Function: &api.OpenAIDeltaFunction{
							Name:      event.ToolName,
							Arguments: args,
						},
					}}}
					if !sentRole {
						delta.Role = "assistant"
						sentRole = true
					}
					p.WriteSSE(w, flusher, api.OpenAIChatResponse{
						ID:      requestID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []api.OpenAIChoice{{Index: 0, Delta: &delta}},
					})

				case "finish-step":
					if event.TotalUsage != nil {
						promptTokens = event.TotalUsage.InputTokens
						completionTokens = event.TotalUsage.OutputTokens
					}

				case "finish":
					finished = true
					if event.TotalUsage != nil {
						promptTokens = event.TotalUsage.InputTokens
						completionTokens = event.TotalUsage.OutputTokens
					}
					reason := normalizeFinishReason(event.FinishReason)
					usage := &api.OpenAIUsage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      promptTokens + completionTokens,
					}
					p.WriteSSE(w, flusher, api.OpenAIChatResponse{
						ID:      requestID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []api.OpenAIChoice{{
							Index:        0,
							Delta:        &api.OpenAIDelta{},
							FinishReason: &reason,
						}},
						Usage: usage,
					})
					fmt.Fprintf(w, "data: [DONE]\n\n")
					flusher.Flush()
					return

				case "error":
					log.Printf("[ERROR] Stream error: %v", event.Error)
					if event.Error != nil && attempt < maxRetries {
						msg := event.Error.Message
						if strings.Contains(msg, "Rate limit") {
							time.Sleep(retryDelayRateLimit(attempt))
							continue streamLoop
						}
						if strings.Contains(msg, "Network connection lost") ||
							strings.Contains(msg, "Gateway request failed") ||
							strings.Contains(msg, "timeout") ||
							strings.Contains(msg, "internal server error") ||
							strings.Contains(msg, "Service temporarily unavailable") ||
							strings.Contains(msg, "Connection error") {
							continue streamLoop
						}
					}
					return
				}
			}
		}
	}
}

// WriteSSE writes a Server-Sent Event
func (p *Proxy) WriteSSE(w io.Writer, flusher http.Flusher, resp api.OpenAIChatResponse) {
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// NonStreamResponse handles non-streaming response
func (p *Proxy) NonStreamResponse(w http.ResponseWriter, ccResp *http.Response, requestID, model string, created int64) {
	decoder := json.NewDecoder(ccResp.Body)

	var content strings.Builder
	var reasoning strings.Builder
	var inputTokens, outputTokens int
	var hasToolCalls bool
	var toolCalls []api.ToolCall
	toolCallByID := map[string]int{}
	toolInputBuffers := map[string]*strings.Builder{}

	for {
		var event api.CCStreamEvent
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("[ERROR] Decode error: %v", err)
			break
		}
		p.debugServerf("[DEBUG] CommandCode stream line: %s", truncateLog(event.Type))

		switch event.Type {
		case "reasoning-delta":
			reasoning.WriteString(event.Text)
		case "text-delta":
			content.WriteString(event.Text)
		case "tool-use":
			hasToolCalls = true
			toolCallByID[event.ToolCallID] = len(toolCalls)
			toolCalls = append(toolCalls, api.ToolCall{
				ID:   event.ToolCallID,
				Type: "function",
				Function: api.FunctionCall{
					Name:      event.ToolName,
					Arguments: "",
				},
			})
		case "tool-delta":
			if len(toolCalls) > 0 {
				toolCalls[len(toolCalls)-1].Function.Arguments += event.Text
			}
		case "tool-input-start":
			hasToolCalls = true
			toolCallByID[event.ID] = len(toolCalls)
			toolInputBuffers[event.ID] = &strings.Builder{}
			toolCalls = append(toolCalls, api.ToolCall{
				ID:   event.ID,
				Type: "function",
				Function: api.FunctionCall{
					Name:      event.ToolName,
					Arguments: "",
				},
			})
		case "tool-input-delta":
			if b := toolInputBuffers[event.ID]; b != nil {
				b.WriteString(event.Delta)
			}
			if idx, ok := toolCallByID[event.ID]; ok {
				toolCalls[idx].Function.Arguments += event.Delta
			}
		case "tool-call":
			hasToolCalls = true
			args := ""
			if event.Input != nil {
				if data, err := json.Marshal(event.Input); err == nil {
					args = string(data)
				}
			}
			if idx, ok := toolCallByID[event.ToolCallID]; ok {
				toolCalls[idx].Function.Name = event.ToolName
				if args != "" {
					toolCalls[idx].Function.Arguments = args
				}
			} else {
				toolCallByID[event.ToolCallID] = len(toolCalls)
				toolCalls = append(toolCalls, api.ToolCall{
					ID:   event.ToolCallID,
					Type: "function",
					Function: api.FunctionCall{
						Name:      event.ToolName,
						Arguments: args,
					},
				})
			}
		case "finish":
			if event.TotalUsage != nil {
				inputTokens = event.TotalUsage.InputTokens
				outputTokens = event.TotalUsage.OutputTokens
			}
		case "error":
			log.Printf("[ERROR] Stream error: %v", event.Error)
		}
	}

	msg := &api.OpenAIMessage{
		Role:             "assistant",
		Content:          content.String(),
		ReasoningContent: reasoning.String(),
	}
	finishReason := "stop"
	if hasToolCalls {
		msg.Content = nil
		msg.ToolCalls = toolCalls
		finishReason = "tool_calls"
	}

	response := api.OpenAIChatResponse{
		ID:      requestID,
		Object:  "chat.completion",
		Created: created,
		Model:   model,
		Choices: []api.OpenAIChoice{{
			Index:        0,
			Message:      msg,
			FinishReason: &finishReason,
		}},
		Usage: &api.OpenAIUsage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      inputTokens + outputTokens,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (p *Proxy) HandleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.writeOpenAIError(w, http.StatusMethodNotAllowed, "Method not allowed", "invalid_request_error")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.writeOpenAIError(w, http.StatusBadRequest, "Failed to read body", "invalid_request_error")
		return
	}

	p.debugClientf("[DEBUG] Incoming responses request body: %s", truncateLog(string(body)))

	var responsesReq api.OpenAIResponsesRequest
	if err := json.Unmarshal(body, &responsesReq); err != nil {
		p.writeOpenAIError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %s", err.Error()), "invalid_request_error")
		return
	}

	chatReq := responsesToChatRequest(responsesReq)
	rewritten, err := json.Marshal(chatReq)
	if err != nil {
		p.writeOpenAIError(w, http.StatusInternalServerError, "Failed to build request", "server_error")
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(rewritten))
	r.ContentLength = int64(len(rewritten))
	p.HandleChatCompletions(w, r)
}

func responsesToChatRequest(req api.OpenAIResponsesRequest) api.OpenAIChatRequest {
	messages := responsesInputToMessages(req.Input)
	if req.Instructions != nil {
		messages = append([]api.OpenAIMessage{{Role: "system", Content: req.Instructions}}, messages...)
	}

	maxTokens := req.MaxCompletionTokens
	if maxTokens == nil {
		maxTokens = req.MaxOutputTokens
	}
	if maxTokens == nil {
		maxTokens = req.MaxTokens
	}

	return api.OpenAIChatRequest{
		Model:               req.Model,
		Messages:            messages,
		Temperature:         req.Temperature,
		MaxTokens:           req.MaxTokens,
		MaxCompletionTokens: maxTokens,
		Stream:              req.Stream,
		Tools:               req.Tools,
		ToolChoice:          req.ToolChoice,
		ParallelToolCalls:   req.ParallelToolCalls,
		ResponseFormat:      req.ResponseFormat,
		Stop:                req.Stop,
		TopP:                req.TopP,
		User:                req.User,
	}
}

func responsesInputToMessages(input any) []api.OpenAIMessage {
	switch v := input.(type) {
	case nil:
		return nil
	case string:
		return []api.OpenAIMessage{{Role: "user", Content: v}}
	case []any:
		if messages := responseItemsToMessages(v); len(messages) > 0 {
			return messages
		}
		return []api.OpenAIMessage{{Role: "user", Content: v}}
	default:
		return []api.OpenAIMessage{{Role: "user", Content: v}}
	}
}

func responseItemsToMessages(items []any) []api.OpenAIMessage {
	messages := make([]api.OpenAIMessage, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role == "" {
			role = "user"
		}
		content := m["content"]
		if content == nil {
			content = m["text"]
		}
		if content == nil {
			content = m["input"]
		}
		messages = append(messages, api.OpenAIMessage{Role: role, Content: content})
	}
	return messages
}

// HandleModels handles the /v1/models endpoint
func (p *Proxy) HandleModels(w http.ResponseWriter, r *http.Request) {
	data := make([]api.OpenAIModel, len(Models))
	for i, m := range Models {
		data[i] = api.OpenAIModel{
			ID:      m.ID,
			Object:  "model",
			Created: 0,
			OwnedBy: m.Owner,
		}
	}
	models := api.OpenAIModelList{
		Object: "list",
		Data:   data,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}
