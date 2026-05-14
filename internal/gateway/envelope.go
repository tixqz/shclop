package gateway

type Envelope struct {
	Type      string         `json:"type"`
	AgentID   string         `json:"agent_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	MessageID string         `json:"message_id,omitempty"`
	Seq       int64          `json:"seq"`
	Payload   map[string]any `json:"payload,omitempty"`
}
