package gateway

type MockRuntime struct{}

func (MockRuntime) Respond(agentID, sessionID, messageID, text string) []Envelope {
	return []Envelope{
		{Type: "message.started", AgentID: agentID, SessionID: sessionID, MessageID: messageID, Seq: 1},
		{Type: "message.delta", AgentID: agentID, SessionID: sessionID, MessageID: messageID, Seq: 2, Payload: map[string]any{"text": "Mock response: "}},
		{Type: "message.delta", AgentID: agentID, SessionID: sessionID, MessageID: messageID, Seq: 3, Payload: map[string]any{"text": text}},
		{Type: "message.done", AgentID: agentID, SessionID: sessionID, MessageID: messageID, Seq: 4},
	}
}
