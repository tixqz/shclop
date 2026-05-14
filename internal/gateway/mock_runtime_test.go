package gateway

import "testing"

func TestMockRuntimeStreamsResponse(t *testing.T) {
	r := MockRuntime{}
	events := r.Respond("agent-1", "session-1", "msg-1", "hello")
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events got %d", len(events))
	}
	if events[0].Type != "message.started" {
		t.Fatalf("unexpected first type %q", events[0].Type)
	}
	if events[len(events)-1].Type != "message.done" {
		t.Fatalf("unexpected final type %q", events[len(events)-1].Type)
	}
}
