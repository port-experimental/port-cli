package plan

import (
	"testing"

	"github.com/port-experimental/port-cli/internal/resources"
)

func TestIdentifiers_Sorted(t *testing.T) {
	p := &ExecutionPlan{Steps: []Step{
		{Kind: resources.KindActions, Operation: OpCreate, Identifier: "z"},
		{Kind: resources.KindActions, Operation: OpCreate, Identifier: "a"},
		{Kind: resources.KindActions, Operation: OpUpdate, Identifier: "b"},
	}}
	ids := Identifiers(p, resources.KindActions, OpCreate)
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "z" {
		t.Fatalf("unexpected ids: %#v", ids)
	}
}

func TestCount(t *testing.T) {
	p := &ExecutionPlan{Steps: []Step{
		{Kind: resources.KindPages, Operation: OpSkip, Identifier: "p1"},
		{Kind: resources.KindPages, Operation: OpSkip, Identifier: "p2"},
	}}
	if Count(p, resources.KindPages, OpSkip) != 2 {
		t.Fatalf("expected 2 skips, got %d", Count(p, resources.KindPages, OpSkip))
	}
}
