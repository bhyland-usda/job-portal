package messaging

import (
	"testing"
	"time"
)

// TestBrokerDistinguishesTypingFromMessage verifies BUG-A: typing and new-message
// notifications must be distinguishable on the wire. The broker now carries an
// event kind, so a subscriber can tell a "typing" signal from an "update" signal.
func TestBrokerDistinguishesTypingFromMessage(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe("user-1")
	defer b.Unsubscribe("user-1")

	// New message -> "update"
	b.Notify("user-1")
	select {
	case ev := <-ch:
		if ev != EventMessage {
			t.Errorf("expected EventMessage (%q) for Notify, got %q", EventMessage, ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message event")
	}

	// Typing -> "typing"
	b.NotifyTyping("user-1")
	select {
	case ev := <-ch:
		if ev != EventTyping {
			t.Errorf("expected EventTyping (%q) for NotifyTyping, got %q", EventTyping, ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for typing event")
	}

	// The two event kinds must not be equal, otherwise the stream can't tell
	// them apart.
	if EventMessage == EventTyping {
		t.Fatal("EventMessage and EventTyping must be distinct values")
	}
}

// TestBrokerNoSubscriberDoesNotBlock verifies Notify/NotifyTyping are safe no-ops
// when nobody is listening (e.g. recipient offline).
func TestBrokerNoSubscriberDoesNotBlock(t *testing.T) {
	b := NewBroker()
	done := make(chan struct{})
	go func() {
		b.Notify("ghost")
		b.NotifyTyping("ghost")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Notify/NotifyTyping blocked with no subscriber")
	}
}

// TestSSEEventLine verifies the SSE data line produced for each event kind is
// distinct, matching what the chat-drawer client parses ("data: update" vs
// "data: typing").
func TestSSEEventLine(t *testing.T) {
	if sseDataLine(EventMessage) == sseDataLine(EventTyping) {
		t.Fatal("SSE lines for message and typing must differ")
	}
	if sseDataLine(EventMessage) != "data: update\n\n" {
		t.Errorf("unexpected message SSE line: %q", sseDataLine(EventMessage))
	}
	if sseDataLine(EventTyping) != "data: typing\n\n" {
		t.Errorf("unexpected typing SSE line: %q", sseDataLine(EventTyping))
	}
}
