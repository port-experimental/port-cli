package import_module

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/port-experimental/port-cli/internal/api"
)

func TestPartitionEntities_FiltersByBlueprintOptions(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "export.json")
	content := `{
  "entities": [
    {"identifier":"svc-1","blueprint":"service"},
    {"identifier":"hidden-1","blueprint":"hidden"},
    {"identifier":"user-1","blueprint":"_user"}
  ]
}`
	if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write export: %v", err)
	}

	partitions, err := partitionEntities(inputPath, Options{
		SkipSystemBlueprints: true,
		ExcludeBlueprints:    []string{"hidden"},
	})
	if err != nil {
		t.Fatalf("partitionEntities error: %v", err)
	}
	defer partitions.cleanup()

	list := partitions.list()
	if len(list) != 1 {
		t.Fatalf("expected 1 partition, got %d: %v", len(list), list)
	}
	if list[0].Blueprint != "service" {
		t.Fatalf("expected service partition, got %q", list[0].Blueprint)
	}

	var entities []api.Entity
	if err := forEachPartitionEntity(list[0].Path, func(entity api.Entity) error {
		entities = append(entities, entity)
		return nil
	}); err != nil {
		t.Fatalf("read partition entities: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}
	if entities[0]["identifier"] != "svc-1" {
		t.Fatalf("unexpected entity: %v", entities[0])
	}
}
