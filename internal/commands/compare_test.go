package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompareIncludeEntitiesValid(t *testing.T) {
	root := &cobra.Command{}
	RegisterCompare(root)
	root.SetArgs([]string{"compare", "--source", "src", "--target", "tgt", "--include", "entities"})
	// We just want to verify "entities" passes validation and doesn't return
	// "invalid resource: entities" — the command will fail for other reasons (no real org)
	err := root.Execute()
	if err != nil && strings.Contains(err.Error(), "invalid resource: entities") {
		t.Errorf("'entities' should be a valid --include resource, got: %v", err)
	}
}
