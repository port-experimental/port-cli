package api

import (
	"encoding/json"
	"testing"
)

func TestParseSSELine_ValidDataLine(t *testing.T) {
	line := `data: {"type":"execution","payload":{"content":"thinking..."}}`
	event, err := ParseSSELine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected non-nil event, got nil")
	}
	if event.Type != "execution" {
		t.Errorf("expected type %q, got %q", "execution", event.Type)
	}
}

func TestParseSSELine_WaitingEvent(t *testing.T) {
	line := `data: {"type":"waiting","payload":{}}`
	event, err := ParseSSELine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected non-nil event, got nil")
	}
	if event.Type != "waiting" {
		t.Errorf("expected type %q, got %q", "waiting", event.Type)
	}
}

func TestParseSSELine_BlankLine(t *testing.T) {
	event, err := ParseSSELine("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event for blank line, got %+v", event)
	}
}

func TestParseSSELine_EventFieldLine(t *testing.T) {
	event, err := ParseSSELine("event: message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event for event: field, got %+v", event)
	}
}

func TestParseSSELine_CommentLine(t *testing.T) {
	event, err := ParseSSELine(": foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event for comment line, got %+v", event)
	}
}

func TestParseSSELine_DoneMarker(t *testing.T) {
	event, err := ParseSSELine("data: [DONE]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event for [DONE], got %+v", event)
	}
}

func TestParseSSELine_EmptyDataAfterPrefix(t *testing.T) {
	event, err := ParseSSELine("data: ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event for empty data, got %+v", event)
	}
}

func TestParseSSELine_NonJSONPayload(t *testing.T) {
	event, err := ParseSSELine("data: not-json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event for non-JSON data, got %+v", event)
	}
}

func TestParseSSELine_DoneEventWithOutput(t *testing.T) {
	line := `data: {"type":"done","payload":{"output":"hello"}}`
	event, err := ParseSSELine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected non-nil event, got nil")
	}
	if event.Type != "done" {
		t.Errorf("expected type %q, got %q", "done", event.Type)
	}
	// Verify the payload is present and contains the output field.
	var payload struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if payload.Output != "hello" {
		t.Errorf("expected output %q, got %q", "hello", payload.Output)
	}
}

func TestParseSSELine_InvocationIdentifierEvent(t *testing.T) {
	line := `data: {"type":"invocationIdentifier","payload":{"invocationIdentifier":"inv_abc"}}`
	event, err := ParseSSELine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected non-nil event, got nil")
	}
	if event.Type != "invocationIdentifier" {
		t.Errorf("expected type %q, got %q", "invocationIdentifier", event.Type)
	}
}

// ---------------------------------------------------------------------------
// ParseSSEBlock tests — RED PHASE
// ParseSSEBlock does not exist yet; all tests below must FAIL until Kou
// implements it.
// ---------------------------------------------------------------------------

func TestParseSSEBlock_InvocationIdentifier(t *testing.T) {
	lines := []string{
		"event: invocationIdentifier",
		"data: 29ca2424-ee5d-418f-9edb-6696510b9701",
	}
	got := ParseSSEBlock(lines)
	if got == nil {
		t.Fatal("want non-nil SSEEvent, got nil")
	}
	if got.Type != "invocationIdentifier" {
		t.Errorf("want Type %q, got %q", "invocationIdentifier", got.Type)
	}
	if got.Data != "29ca2424-ee5d-418f-9edb-6696510b9701" {
		t.Errorf("want Data %q, got %q", "29ca2424-ee5d-418f-9edb-6696510b9701", got.Data)
	}
}

func TestParseSSEBlock_Execution(t *testing.T) {
	lines := []string{
		"event: execution",
		"data: texto del agente",
	}
	got := ParseSSEBlock(lines)
	if got == nil {
		t.Fatal("want non-nil SSEEvent, got nil")
	}
	if got.Type != "execution" {
		t.Errorf("want Type %q, got %q", "execution", got.Type)
	}
	if got.Data != "texto del agente" {
		t.Errorf("want Data %q, got %q", "texto del agente", got.Data)
	}
}

func TestParseSSEBlock_Done(t *testing.T) {
	doneJSON := `{"rateLimitUsage":{},"monthlyQuotaUsage":{"monthlyLimit":500,"remainingQuota":495},"contextUsage":{}}`
	lines := []string{
		"event: done",
		"data: " + doneJSON,
	}
	got := ParseSSEBlock(lines)
	if got == nil {
		t.Fatal("want non-nil SSEEvent, got nil")
	}
	if got.Type != "done" {
		t.Errorf("want Type %q, got %q", "done", got.Type)
	}
	if got.Data != doneJSON {
		t.Errorf("want Data %q, got %q", doneJSON, got.Data)
	}
}

func TestParseSSEBlock_WaitingNullData(t *testing.T) {
	lines := []string{
		"event: waiting",
		"data: null",
	}
	got := ParseSSEBlock(lines)
	if got == nil {
		t.Fatal("want non-nil SSEEvent, got nil")
	}
	if got.Type != "waiting" {
		t.Errorf("want Type %q, got %q", "waiting", got.Type)
	}
	if got.Data != "null" {
		t.Errorf("want Data %q, got %q", "null", got.Data)
	}
}

func TestParseSSEBlock_MultiLineData(t *testing.T) {
	lines := []string{
		"event: execution",
		"data: primera línea",
		"data: segunda línea",
	}
	got := ParseSSEBlock(lines)
	if got == nil {
		t.Fatal("want non-nil SSEEvent, got nil")
	}
	want := "primera línea\nsegunda línea"
	if got.Data != want {
		t.Errorf("want Data %q, got %q", want, got.Data)
	}
}

func TestParseSSEBlock_CommentIgnored(t *testing.T) {
	lines := []string{
		": this is a comment",
		"event: execution",
		"data: respuesta",
	}
	got := ParseSSEBlock(lines)
	if got == nil {
		t.Fatal("want non-nil SSEEvent, got nil")
	}
	if got.Type != "execution" {
		t.Errorf("want Type %q, got %q", "execution", got.Type)
	}
	if got.Data != "respuesta" {
		t.Errorf("want Data %q, got %q", "respuesta", got.Data)
	}
}

func TestParseSSEBlock_EmptyLines(t *testing.T) {
	got := ParseSSEBlock([]string{})
	if got != nil {
		t.Errorf("want nil for empty input, got %+v", got)
	}
}

func TestParseSSEBlock_OnlyEventNoData(t *testing.T) {
	lines := []string{
		"event: execution",
	}
	got := ParseSSEBlock(lines)
	if got != nil {
		t.Errorf("want nil when block has no data field, got %+v", got)
	}
}

func TestParseSSEBlock_Ping(t *testing.T) {
	lines := []string{
		"event: ping",
		"data: {}",
	}
	got := ParseSSEBlock(lines)
	if got == nil {
		t.Fatal("want non-nil SSEEvent, got nil")
	}
	if got.Type != "ping" {
		t.Errorf("want Type %q, got %q", "ping", got.Type)
	}
	if got.Data != "{}" {
		t.Errorf("want Data %q, got %q", "{}", got.Data)
	}
}
