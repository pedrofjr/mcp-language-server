package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

type blockedWrite struct {
	data    []byte
	release chan struct{}
}

func (w *blockedWrite) unblock() {
	close(w.release)
}

type frameBarrierWriteCloser struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	writes chan *blockedWrite
	closed bool
}

func newFrameBarrierWriteCloser() *frameBarrierWriteCloser {
	return &frameBarrierWriteCloser{
		writes: make(chan *blockedWrite, 16),
	}
}

func (w *frameBarrierWriteCloser) Write(p []byte) (int, error) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	_, _ = w.buf.Write(p)
	w.mu.Unlock()

	write := &blockedWrite{
		data:    append([]byte(nil), p...),
		release: make(chan struct{}),
	}
	w.writes <- write
	<-write.release

	return len(p), nil
}

func (w *frameBarrierWriteCloser) Close() error {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	return nil
}

func (w *frameBarrierWriteCloser) Bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.buf.Bytes()...)
}

func waitForBlockedWrite(t *testing.T, w *frameBarrierWriteCloser, timeout time.Duration) *blockedWrite {
	t.Helper()

	select {
	case write := <-w.writes:
		return write
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for client write after %s", timeout)
		return nil
	}
}

func waitForBlockedWriteMaybe(w *frameBarrierWriteCloser, timeout time.Duration) (*blockedWrite, bool) {
	select {
	case write := <-w.writes:
		return write, true
	case <-time.After(timeout):
		return nil, false
	}
}

func isContentLengthHeader(data []byte) bool {
	return bytes.HasPrefix(data, []byte("Content-Length: "))
}

func drainBlockedWrites(t *testing.T, w *frameBarrierWriteCloser, count int) {
	t.Helper()

	for i := 0; i < count; i++ {
		waitForBlockedWrite(t, w, time.Second).unblock()
	}
}

func readMessagesFromWire(t *testing.T, wire []byte, want int) []*Message {
	t.Helper()

	reader := bufio.NewReader(bytes.NewReader(wire))
	messages := make([]*Message, 0, want)
	for i := 0; i < want; i++ {
		msg, err := ReadMessage(reader)
		if err != nil {
			t.Fatalf("expected %d parseable LSP frame(s), but frame %d failed to parse: %v\nwire=%q", want, i+1, err, wire)
		}
		messages = append(messages, msg)
	}

	return messages
}

func waitForErr(t *testing.T, done <-chan error, timeout time.Duration, label string) error {
	t.Helper()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %s after %s", label, timeout)
		return nil
	}
}

func waitForPendingHandler(t *testing.T, c *Client, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.handlersMu.RLock()
		count := len(c.handlers)
		c.handlersMu.RUnlock()
		if count > 0 {
			return
		}
		time.Sleep(1 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for Call to register a pending response handler after %s", timeout)
}

func TestNotify_ConcurrentSendsProduceTwoParseableFrames(t *testing.T) {
	writer := newFrameBarrierWriteCloser()
	client := &Client{stdin: writer}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- client.Notify(ctx, "test/one", map[string]string{"message": "first"})
	}()

	firstHeader := waitForBlockedWrite(t, writer, time.Second)
	if !isContentLengthHeader(firstHeader.data) {
		t.Fatalf("expected first blocked write to be an LSP header, got %q", firstHeader.data)
	}

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- client.Notify(ctx, "test/two", map[string]string{"message": "second"})
	}()

	secondHeader, interleaved := waitForBlockedWriteMaybe(writer, 150*time.Millisecond)
	if interleaved {
		if !isContentLengthHeader(secondHeader.data) {
			t.Fatalf("expected second blocked write to also be an LSP header, got %q", secondHeader.data)
		}
		firstHeader.unblock()
		secondHeader.unblock()
		drainBlockedWrites(t, writer, 2)
	} else {
		firstHeader.unblock()
		drainBlockedWrites(t, writer, 3)
	}

	if err := waitForErr(t, firstDone, time.Second, "first Notify to finish"); err != nil {
		t.Fatalf("expected first Notify to succeed, got error: %v", err)
	}
	if err := waitForErr(t, secondDone, time.Second, "second Notify to finish"); err != nil {
		t.Fatalf("expected second Notify to succeed, got error: %v", err)
	}

	messages := readMessagesFromWire(t, writer.Bytes(), 2)
	if messages[0].Method != "test/one" {
		t.Fatalf("expected first frame method test/one, got %q", messages[0].Method)
	}
	if messages[1].Method != "test/two" {
		t.Fatalf("expected second frame method test/two, got %q", messages[1].Method)
	}
	if messages[0].ID != nil || messages[1].ID != nil {
		t.Fatalf("expected Notify frames to be notifications without IDs, got ids %v and %v", messages[0].ID, messages[1].ID)
	}
}

func TestHandleMessages_ResponseAndConcurrentNotifyProduceTwoParseableFrames(t *testing.T) {
	writer := newFrameBarrierWriteCloser()
	stdoutReader, stdoutWriter := io.Pipe()
	client := &Client{
		stdin:                 writer,
		stdout:                bufio.NewReader(stdoutReader),
		handlers:              make(map[string]chan *Message),
		serverRequestHandlers: make(map[string]ServerRequestHandler),
		notificationHandlers:  make(map[string]NotificationHandler),
	}

	client.RegisterServerRequestHandler("workspace/configuration", func(params json.RawMessage) (any, error) {
		return map[string]any{"items": []any{}}, nil
	})

	handleDone := make(chan struct{})
	go func() {
		client.handleMessages()
		close(handleDone)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	notifyDone := make(chan error, 1)
	go func() {
		notifyDone <- client.Notify(ctx, "workspace/didChangeConfiguration", map[string]bool{"test": true})
	}()

	firstHeader := waitForBlockedWrite(t, writer, time.Second)
	if !isContentLengthHeader(firstHeader.data) {
		t.Fatalf("expected Notify to start by writing an LSP header, got %q", firstHeader.data)
	}

	request, err := NewRequest(int32(99), "workspace/configuration", []map[string]string{{"scopeUri": "file:///workspace"}})
	if err != nil {
		t.Fatalf("failed to create fake server request: %v", err)
	}

	serverWriteDone := make(chan error, 1)
	go func() {
		serverWriteDone <- WriteMessage(stdoutWriter, request)
		_ = stdoutWriter.Close()
	}()

	secondHeader, interleaved := waitForBlockedWriteMaybe(writer, 150*time.Millisecond)
	if interleaved {
		if !isContentLengthHeader(secondHeader.data) {
			t.Fatalf("expected server response path to reach a header write, got %q", secondHeader.data)
		}
		firstHeader.unblock()
		secondHeader.unblock()
		drainBlockedWrites(t, writer, 2)
	} else {
		firstHeader.unblock()
		drainBlockedWrites(t, writer, 3)
	}

	if err := waitForErr(t, notifyDone, time.Second, "Notify to finish"); err != nil {
		t.Fatalf("expected Notify to succeed, got error: %v", err)
	}
	if err := waitForErr(t, serverWriteDone, time.Second, "fake server request write to finish"); err != nil {
		t.Fatalf("expected fake server request write to succeed, got error: %v", err)
	}

	select {
	case <-handleDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for handleMessages to stop after fake server closed stdout")
	}

	messages := readMessagesFromWire(t, writer.Bytes(), 2)
	if messages[0].Method != "workspace/didChangeConfiguration" {
		t.Fatalf("expected first frame to be the client notification, got method %q", messages[0].Method)
	}
	if messages[1].ID == nil || messages[1].ID.Value != int32(99) {
		t.Fatalf("expected second frame to be the response to request id 99, got id %v", messages[1].ID)
	}
	if messages[1].Error != nil {
		t.Fatalf("expected server request response to succeed, got rpc error: %+v", messages[1].Error)
	}
}

func assertCallFailsSoonAfterReadLoopStops(t *testing.T, ctx context.Context, assertReturnedErr func(error)) {
	t.Helper()

	stdoutReader, stdoutWriter := io.Pipe()
	client := &Client{
		stdin:                 discardWriteCloser{},
		stdout:                bufio.NewReader(stdoutReader),
		handlers:              make(map[string]chan *Message),
		serverRequestHandlers: make(map[string]ServerRequestHandler),
		notificationHandlers:  make(map[string]NotificationHandler),
	}

	readLoopDone := make(chan struct{})
	go func() {
		client.handleMessages()
		close(readLoopDone)
	}()

	callDone := make(chan error, 1)
	go func() {
		var result json.RawMessage
		callDone <- client.Call(ctx, "test/pending", map[string]string{"state": "waiting"}, &result)
	}()

	waitForPendingHandler(t, client, time.Second)

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("failed to close fake server stdout: %v", err)
	}

	select {
	case <-readLoopDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for read loop to stop after closing server stdout")
	}

	select {
	case err := <-callDone:
		if err == nil {
			t.Fatal("expected Call to return a transport-related error after read loop stopped, got nil")
		}
		if assertReturnedErr != nil {
			assertReturnedErr(err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("expected Call to fail after read loop EOF, but it remained blocked in the pending-response path: %s", fmt.Sprintf("resp := <-ch"))
	}
}

func TestCall_ReturnsTransportErrorWhenReadLoopStopsBeforeResponseWithoutDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	assertCallFailsSoonAfterReadLoopStops(t, ctx, nil)
}

func TestCall_ReturnsErrorWhenReadLoopStopsBeforeResponse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	assertCallFailsSoonAfterReadLoopStops(t, ctx, func(err error) {
		if ctx.Err() != nil {
			t.Fatalf("expected Call to fail before the context deadline after read loop EOF, got err=%v", err)
		}
	})
}