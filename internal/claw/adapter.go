package claw

import "context"

type Task struct {
	Text      string
	SessionID string
}

type EventType string

const (
	EventStarted EventType = "started"
	EventDelta   EventType = "delta"
	EventDone    EventType = "done"
	EventError   EventType = "error"
)

type Event struct {
	Type     EventType
	Text     string
	Err      error
	ExitCode int
}

type Adapter interface {
	Run(ctx context.Context, task Task) (<-chan Event, error)
}

type DemoAdapter struct{ Flavor string }

func (a DemoAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {
	out := make(chan Event, 4)
	go func() {
		defer close(out)
		flavor := a.Flavor
		if flavor == "" {
			flavor = "nanoclaw"
		}
		events := []Event{
			{Type: EventStarted},
			{Type: EventDelta, Text: flavor + " runtime received: " + task.Text + "\n"},
			{Type: EventDelta, Text: "workspace=/workspace memory=/memory\n"},
			{Type: EventDone},
		}
		for _, event := range events {
			select {
			case <-ctx.Done():
				return
			case out <- event:
			}
		}
	}()
	return out, nil
}
