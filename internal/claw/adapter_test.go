package claw

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDemoAdapterEmitsStructuredEvents(t *testing.T) {
	adapter := DemoAdapter{Flavor: "nanoclaw"}
	events, err := adapter.Run(context.Background(), Task{Text: "hello"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var types []EventType
	for event := range events {
		types = append(types, event.Type)
	}
	want := []EventType{EventStarted, EventDelta, EventDelta, EventDone}
	if len(types) != len(want) {
		t.Fatalf("got %v want %v", types, want)
	}
}

func TestSubprocessAdapterStreamsStdout(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("shell script test is for darwin/linux")
	}
	script := "printf '%s\\n' 'one' 'two'; printf '%s\\n' 'err' >&2"
	adapter := SubprocessAdapter{Binary: "sh", Args: []string{"-c", script}}
	events, err := adapter.Run(context.Background(), Task{Text: "hello"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []Event
	for event := range events {
		got = append(got, event)
	}
	if len(got) < 3 {
		t.Fatalf("got %v", got)
	}
	if got[0].Type != EventStarted {
		t.Fatalf("first event = %v", got[0])
	}
	if got[len(got)-1].Type != EventDone {
		t.Fatalf("last event = %v", got[len(got)-1])
	}
	combined := ""
	for _, event := range got[1 : len(got)-1] {
		combined += event.Text
	}
	if !strings.Contains(combined, "one\n") || !strings.Contains(combined, "two\n") || !strings.Contains(combined, "stderr: err\n") {
		t.Fatalf("missing expected output in %#v", got)
	}
}

func TestSubprocessAdapterStreamsLongOutput(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("shell script test is for darwin/linux")
	}
	script := "awk 'BEGIN{for(i=0;i<70000;i++)printf \"a\"; printf \"\\n\"}'"
	adapter := SubprocessAdapter{Binary: "sh", Args: []string{"-c", script}}
	events, err := adapter.Run(context.Background(), Task{Text: "hello"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var total int
	var sawStarted, sawDone bool
	for event := range events {
		switch event.Type {
		case EventStarted:
			sawStarted = true
		case EventDelta:
			total += len(event.Text)
		case EventDone:
			sawDone = true
		}
	}
	if !sawStarted || !sawDone {
		t.Fatalf("started=%v done=%v total=%d", sawStarted, sawDone, total)
	}
	if total < 70000 {
		t.Fatalf("short output: %d", total)
	}
}

func TestSubprocessAdapterReportsNonZeroExit(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("shell script test is for darwin/linux")
	}
	adapter := SubprocessAdapter{Binary: "sh", Args: []string{"-c", "printf '%s\\n' fail >&2; exit 7"}}
	events, err := adapter.Run(context.Background(), Task{Text: "hello"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var last Event
	for event := range events {
		last = event
	}
	if last.Type != EventError {
		t.Fatalf("last type = %v", last.Type)
	}
	if last.ExitCode != 7 {
		t.Fatalf("exit code = %d", last.ExitCode)
	}
}

func TestSubprocessAdapterClosesOnCanceledContext(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("shell script test is for darwin/linux")
	}
	ctx, cancel := context.WithCancel(context.Background())
	adapter := SubprocessAdapter{Binary: "sh", Args: []string{"-c", "sleep 1"}}
	events, err := adapter.Run(ctx, Task{Text: "hello"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	cancel()
	for {
		select {
		case _, ok := <-events:
			if ok {
				continue
			}
			return
		case <-time.After(2 * time.Second):
			t.Fatal("channel did not close")
		}
	}
}

func TestSubprocessAdapterClosesWhenGrandchildHoldsPipes(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("shell script test is for darwin/linux")
	}
	ctx, cancel := context.WithCancel(context.Background())
	adapter := SubprocessAdapter{Binary: "sh", Args: []string{"-c", "sleep 3600 &"}}
	events, err := adapter.Run(ctx, Task{Text: "hello"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	cancel()
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return
			}
		case <-time.After(2 * time.Second):
			t.Fatal("channel did not close")
		}
	}
}
