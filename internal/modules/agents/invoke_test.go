package agents

// RED PHASE — all tests in this file MUST FAIL until Kou implements the new
// SSEEvent{Type, Data string} design and the updated processEvent signature.
//
// Compilation errors are expected: SSEEvent currently has Payload json.RawMessage,
// not Data string. That is intentional — the test file encodes the target contract.

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

// ---------------------------------------------------------------------------
// invocationIdentifier
// ---------------------------------------------------------------------------

func TestProcessEvent_InvocationIdentifier(t *testing.T) {
	result := &InvokeResult{}
	event := api.SSEEvent{Type: "invocationIdentifier", Data: "test-uuid"}
	if err := processEvent(event, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.InvocationID != "test-uuid" {
		t.Errorf("want InvocationID %q, got %q", "test-uuid", result.InvocationID)
	}
}

// ---------------------------------------------------------------------------
// execution — plain-text chunks that accumulate into Output
// ---------------------------------------------------------------------------

func TestProcessEvent_Execution_Single(t *testing.T) {
	result := &InvokeResult{}
	event := api.SSEEvent{Type: "execution", Data: "hola"}
	if err := processEvent(event, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "hola" {
		t.Errorf("want Output %q, got %q", "hola", result.Output)
	}
}

func TestProcessEvent_Execution_Accumulates(t *testing.T) {
	result := &InvokeResult{}

	e1 := api.SSEEvent{Type: "execution", Data: "hola "}
	if err := processEvent(e1, result); err != nil {
		t.Fatalf("unexpected error on first chunk: %v", err)
	}

	e2 := api.SSEEvent{Type: "execution", Data: "mundo"}
	if err := processEvent(e2, result); err != nil {
		t.Fatalf("unexpected error on second chunk: %v", err)
	}

	if result.Output != "hola mundo" {
		t.Errorf("want Output %q, got %q", "hola mundo", result.Output)
	}
}

// ---------------------------------------------------------------------------
// done — JSON with quota fields, NO output field
// ---------------------------------------------------------------------------

func TestProcessEvent_Done_SetsQuota(t *testing.T) {
	result := &InvokeResult{}
	event := api.SSEEvent{
		Type: "done",
		Data: `{"monthlyQuotaUsage":{"monthlyLimit":500,"remainingQuota":495},"rateLimitUsage":{},"contextUsage":{}}`,
	}
	if err := processEvent(event, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MonthlyQuotaUsage == nil {
		t.Fatal("want MonthlyQuotaUsage to be set, got nil")
	}
}

func TestProcessEvent_Done_NoOutputField(t *testing.T) {
	// The real "done" event carries no "output" field.
	// Processing it must NOT set result.Output.
	result := &InvokeResult{}
	// Simulate accumulated output from execution chunks.
	result.Output = "texto acumulado"
	event := api.SSEEvent{
		Type: "done",
		Data: `{"monthlyQuotaUsage":{}}`,
	}
	if err := processEvent(event, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// done must not overwrite the accumulated Output.
	if result.Output != "texto acumulado" {
		t.Errorf("want Output %q (unchanged), got %q", "texto acumulado", result.Output)
	}
}

// ---------------------------------------------------------------------------
// toolCall — JSON with "name" and "arguments" (not "toolName"/"args")
// ---------------------------------------------------------------------------

func TestProcessEvent_ToolCall_AskUserQuestions(t *testing.T) {
	result := &InvokeResult{}
	event := api.SSEEvent{
		Type: "toolCall",
		Data: `{"name":"ask_user_questions","arguments":{"questions":["¿Qué región?","¿Qué SKU?"]}}`,
	}
	if err := processEvent(event, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.AskUserQuestions) != 2 {
		t.Fatalf("want 2 questions, got %d", len(result.AskUserQuestions))
	}
	if result.AskUserQuestions[0] != "¿Qué región?" {
		t.Errorf("want first question %q, got %q", "¿Qué región?", result.AskUserQuestions[0])
	}
	if result.AskUserQuestions[1] != "¿Qué SKU?" {
		t.Errorf("want second question %q, got %q", "¿Qué SKU?", result.AskUserQuestions[1])
	}
}

func TestProcessEvent_ToolCall_OtherName(t *testing.T) {
	result := &InvokeResult{}
	event := api.SSEEvent{
		Type: "toolCall",
		Data: `{"name":"create_entity","arguments":{}}`,
	}
	if err := processEvent(event, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AskUserQuestions != nil {
		t.Errorf("want AskUserQuestions nil, got %v", result.AskUserQuestions)
	}
}

// ---------------------------------------------------------------------------
// waiting — data is the string literal "null"; must be a no-op
// ---------------------------------------------------------------------------

func TestProcessEvent_Waiting_NoChange(t *testing.T) {
	result := &InvokeResult{}
	event := api.SSEEvent{Type: "waiting", Data: "null"}
	if err := processEvent(event, result); err != nil {
		t.Fatalf("want no error for waiting event, got %v", err)
	}
	if result.Output != "" || result.InvocationID != "" || result.AskUserQuestions != nil {
		t.Error("want result unchanged after waiting event")
	}
}

// ---------------------------------------------------------------------------
// unknown event types — must be silently ignored
// ---------------------------------------------------------------------------

func TestProcessEvent_Unknown_NoError(t *testing.T) {
	result := &InvokeResult{}
	event := api.SSEEvent{Type: "unknown_type", Data: "anything"}
	if err := processEvent(event, result); err != nil {
		t.Errorf("want nil error for unknown event type, got %v", err)
	}
	if result.Output != "" || result.InvocationID != "" || result.AskUserQuestions != nil {
		t.Error("want result unchanged after unknown event")
	}
}

func TestProcessEvent_ToolCall_AskUserQuestions_MultipleAppend(t *testing.T) {
	result := &InvokeResult{}
	e1 := api.SSEEvent{
		Type: "toolCall",
		Data: `{"name":"ask_user_questions","arguments":{"questions":["¿Región?"]}}`,
	}
	e2 := api.SSEEvent{
		Type: "toolCall",
		Data: `{"name":"ask_user_questions","arguments":{"questions":["¿SKU?","¿Tamaño?"]}}`,
	}
	if err := processEvent(e1, result); err != nil {
		t.Fatalf("unexpected error on e1: %v", err)
	}
	if err := processEvent(e2, result); err != nil {
		t.Fatalf("unexpected error on e2: %v", err)
	}
	want := []string{"¿Región?", "¿SKU?", "¿Tamaño?"}
	if len(result.AskUserQuestions) != len(want) {
		t.Fatalf("got %d questions, want %d: %v", len(result.AskUserQuestions), len(want), result.AskUserQuestions)
	}
	for i, q := range result.AskUserQuestions {
		if q != want[i] {
			t.Errorf("question[%d]: got %q, want %q", i, q, want[i])
		}
	}
}
