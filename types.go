package rustyclaw

// Role represents a chat message role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
	RoleDeveloper Role = "developer"
)

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	Name       *string    `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID *string    `json:"tool_call_id,omitempty"`
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	Model       string           `json:"model"`
	Messages    []ChatMessage    `json:"messages"`
	MaxTokens   *int             `json:"max_tokens,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	TopP        *float64         `json:"top_p,omitempty"`
	Stream      bool             `json:"stream"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	ToolChoice  interface{}      `json:"tool_choice,omitempty"`
}

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"`
}

// ChatChoice represents a single completion choice.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason *string     `json:"finish_reason,omitempty"`
}

// Usage represents token usage statistics.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatChunk represents a streaming chat completion chunk.
type ChatChunk struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []ChatChunkChoice `json:"choices"`
}

// ChatChunkChoice represents a single streaming choice.
type ChatChunkChoice struct {
	Index        int       `json:"index"`
	Delta        ChatDelta `json:"delta"`
	FinishReason *string   `json:"finish_reason,omitempty"`
}

// ChatDelta represents incremental content in a streaming chunk.
type ChatDelta struct {
	Role      *Role           `json:"role,omitempty"`
	Content   *string         `json:"content,omitempty"`
	ToolCalls []ToolCallDelta `json:"tool_calls,omitempty"`
}

// ToolCall represents a tool call in a message.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents the function details in a tool call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCallDelta represents an incremental tool call in streaming.
type ToolCallDelta struct {
	Index    int                `json:"index"`
	ID       *string            `json:"id,omitempty"`
	Type     *string            `json:"type,omitempty"`
	Function *FunctionCallDelta `json:"function,omitempty"`
}

// FunctionCallDelta represents incremental function call data.
type FunctionCallDelta struct {
	Name      *string `json:"name,omitempty"`
	Arguments *string `json:"arguments,omitempty"`
}

// ToolDefinition represents a tool that can be used by the model.
type ToolDefinition struct {
	Type     string                  `json:"type"`
	Function FunctionDefinitionInner `json:"function"`
}

// FunctionDefinitionInner represents the function details in a tool definition.
type FunctionDefinitionInner struct {
	Name        string      `json:"name"`
	Description *string     `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// Resource represents the API resource being accessed.
type Resource struct {
	URL    string `json:"url"`
	Method string `json:"method"`
}

// PaymentAccept describes an accepted payment scheme.
type PaymentAccept struct {
	Scheme            string  `json:"scheme"`
	Network           string  `json:"network"`
	Amount            string  `json:"amount"`
	Asset             string  `json:"asset"`
	PayTo             string  `json:"pay_to"`
	MaxTimeoutSeconds int     `json:"max_timeout_seconds"`
	EscrowProgramID   *string `json:"escrow_program_id,omitempty"`
}

// PaymentRequired represents a 402 Payment Required response.
type PaymentRequired struct {
	X402Version   int             `json:"x402_version"`
	Resource      Resource        `json:"resource"`
	Accepts       []PaymentAccept `json:"accepts"`
	CostBreakdown CostBreakdown   `json:"cost_breakdown"`
	Error         string          `json:"error"`
}

// CostBreakdown provides the cost details for a request.
type CostBreakdown struct {
	ProviderCost string `json:"provider_cost"`
	PlatformFee  string `json:"platform_fee"`
	Total        string `json:"total"`
	Currency     string `json:"currency"`
	FeePercent   int    `json:"fee_percent"`
}

// SolanaPayload is the payload for exact Solana payments.
type SolanaPayload struct {
	Transaction string `json:"transaction"`
}

// EscrowPayload is the payload for escrow-based payments.
type EscrowPayload struct {
	DepositTx   string `json:"deposit_tx"`
	ServiceID   string `json:"service_id"`
	AgentPubkey string `json:"agent_pubkey"`
}

// PaymentPayload represents a payment attached to a request.
type PaymentPayload struct {
	X402Version int           `json:"x402_version"`
	Resource    Resource      `json:"resource"`
	Accepted    PaymentAccept `json:"accepted"`
	Payload     interface{}   `json:"payload"`
}

// ModelInfo describes a model available on the gateway.
type ModelInfo struct {
	ID                       string  `json:"id"`
	Provider                 string  `json:"provider"`
	ModelID                  string  `json:"model_id"`
	DisplayName              string  `json:"display_name"`
	InputCostPerMillion      float64 `json:"input_cost_per_million"`
	OutputCostPerMillion     float64 `json:"output_cost_per_million"`
	ContextWindow            int     `json:"context_window"`
	SupportsStreaming         bool    `json:"supports_streaming"`
	SupportsTools            bool    `json:"supports_tools"`
	SupportsVision           bool    `json:"supports_vision"`
	Reasoning                bool    `json:"reasoning"`
	SupportsStructuredOutput bool    `json:"supports_structured_output"`
	SupportsBatch            bool    `json:"supports_batch"`
	MaxOutputTokens          *int    `json:"max_output_tokens,omitempty"`
}
