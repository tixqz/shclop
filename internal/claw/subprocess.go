package claw

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
)

type SubprocessAdapter struct {
	Binary string
	Args   []string
	Env    []string
}

func (a SubprocessAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {
	if strings.TrimSpace(a.Binary) == "" {
		return nil, errors.New("binary is required")
	}
	out := make(chan Event, 32)
	done := make(chan struct{})
	cmd := exec.CommandContext(ctx, a.Binary, a.Args...)
	if len(a.Env) > 0 {
		cmd.Env = append([]string(nil), a.Env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		defer func() { _ = stdin.Close() }()
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, _ = io.WriteString(stdin, task.Text)
	}()
	select {
	case out <- Event{Type: EventStarted}:
	default:
	}

	var wg sync.WaitGroup
	var closeOnce sync.Once
	var doneOnce sync.Once
	closeDone := func() { doneOnce.Do(func() { close(done) }) }
	closeOut := func() { closeOnce.Do(func() { close(out) }) }
	go func() {
		select {
		case <-ctx.Done():
			_ = stdin.Close()
			_ = stdout.Close()
			_ = stderr.Close()
		case <-done:
		}
	}()
	stream := func(r io.Reader, prefix string) {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				e := Event{Type: EventDelta, Text: prefix + string(buf[:n])}
				select {
				case <-ctx.Done():
					return
				case out <- e:
				}
			}
			if readErr != nil {
				if ctx.Err() == nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, context.Canceled) {
					select {
					case <-ctx.Done():
						return
					case out <- Event{Type: EventError, Err: readErr}:
					}
				}
				return
			}
		}
	}
	wg.Add(2)
	go stream(stdout, "")
	go stream(stderr, "stderr: ")

	go func() {
		defer closeDone()
		wg.Wait()
		waitErr := cmd.Wait()
		if waitErr == nil {
			select {
			case <-ctx.Done():
			case out <- Event{Type: EventDone}:
			}
			closeOut()
			return
		}
		exitCode := 0
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		select {
		case <-ctx.Done():
		case out <- Event{Type: EventError, Err: waitErr, ExitCode: exitCode}:
		}
		closeOut()
	}()

	return out, nil
}
