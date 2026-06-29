package import_module

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/port-experimental/port-cli/internal/api"
)

type entityPartition struct {
	Blueprint string
	Path      string
}

type entityPartitionWriter struct {
	file    *os.File
	encoder *json.Encoder
}

type entityPartitions struct {
	dir   string
	files map[string]*entityPartitionWriter
	paths map[string]string
}

func newEntityPartitions() (*entityPartitions, error) {
	dir, err := os.MkdirTemp("", "port-cli-import-entities-*")
	if err != nil {
		return nil, err
	}
	return &entityPartitions{
		dir:   dir,
		files: make(map[string]*entityPartitionWriter),
		paths: make(map[string]string),
	}, nil
}

func (p *entityPartitions) write(entity api.Entity) error {
	bpID, _ := entity["blueprint"].(string)
	if bpID == "" {
		return nil
	}
	writer, ok := p.files[bpID]
	if !ok {
		path := filepath.Join(p.dir, fmt.Sprintf("%s.jsonl", safePartitionName(bpID)))
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		writer = &entityPartitionWriter{file: file, encoder: json.NewEncoder(file)}
		p.files[bpID] = writer
		p.paths[bpID] = path
	}
	if err := writer.encoder.Encode(entity); err != nil {
		return err
	}
	return nil
}

func (p *entityPartitions) close() error {
	var err error
	for _, writer := range p.files {
		if closeErr := writer.file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

func (p *entityPartitions) cleanup() error {
	return os.RemoveAll(p.dir)
}

func (p *entityPartitions) list() []entityPartition {
	result := make([]entityPartition, 0, len(p.paths))
	for bpID, path := range p.paths {
		result = append(result, entityPartition{
			Blueprint: bpID,
			Path:      path,
		})
	}
	return result
}

func safePartitionName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "entities"
	}
	return b.String()
}

func partitionEntities(inputPath string, opts Options) (*entityPartitions, error) {
	partitions, err := newEntityPartitions()
	if err != nil {
		return nil, err
	}
	loader := NewStreamLoader()
	deepSet := make(map[string]bool, len(opts.ExcludeBlueprints))
	for _, id := range opts.ExcludeBlueprints {
		deepSet[id] = true
	}
	err = loader.ForEachEntity(inputPath, func(entity api.Entity) error {
		bpID, _ := entity["blueprint"].(string)
		if bpID == "" {
			return nil
		}
		if opts.SkipSystemBlueprints && strings.HasPrefix(bpID, "_") {
			return nil
		}
		if deepSet[bpID] {
			return nil
		}
		return partitions.write(entity)
	})
	if closeErr := partitions.close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		partitions.cleanup()
		return nil, err
	}
	return partitions, nil
}

func forEachPartitionEntity(path string, yield func(api.Entity) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	dec := json.NewDecoder(bufio.NewReader(file))
	for {
		var entity api.Entity
		if err := dec.Decode(&entity); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := yield(entity); err != nil {
			return err
		}
	}
}

func (i *Importer) ImportEntitiesFromStream(ctx context.Context, inputPath string, opts Options, result *Result, dryRun bool) error {
	partitions, err := partitionEntities(inputPath, opts)
	if err != nil {
		return err
	}
	defer partitions.cleanup()

	inheritedOwnershipBPs, relationTargets := i.detectInheritedOwnershipBlueprints(ctx)
	blueprintsToSkip := make(map[string]bool)
	for bpID := range relationTargets {
		if blueprintRelatesToInheritedOwnership(bpID, inheritedOwnershipBPs, relationTargets) {
			blueprintsToSkip[bpID] = true
		}
	}

	for _, partition := range partitions.list() {
		if err := i.importEntityPartition(ctx, partition, opts, result, dryRun, inheritedOwnershipBPs, blueprintsToSkip); err != nil {
			return err
		}
	}
	return nil
}

func (i *Importer) importEntityPartition(
	ctx context.Context,
	partition entityPartition,
	opts Options,
	result *Result,
	dryRun bool,
	inheritedOwnershipBPs map[string]bool,
	blueprintsToSkip map[string]bool,
) error {
	currentMap := make(map[string]api.Entity)
	err := i.client.ForEachEntityPage(ctx, partition.Blueprint, map[string]interface{}{
		"combinator": "and",
		"rules":      []interface{}{},
	}, func(entities []api.Entity) error {
		for _, entity := range entities {
			id, _ := entity["identifier"].(string)
			if id != "" {
				currentMap[id] = entity
			}
		}
		return nil
	})
	if err != nil {
		if strings.Contains(err.Error(), "410 Gone") {
			currentMap = make(map[string]api.Entity)
		} else {
			return fmt.Errorf("failed to get current entities for blueprint %s: %w", partition.Blueprint, err)
		}
	}

	changedFile, err := os.CreateTemp(filepath.Dir(partition.Path), safePartitionName(partition.Blueprint)+"-changed-*.jsonl")
	if err != nil {
		return err
	}
	changedPath := changedFile.Name()
	defer os.Remove(changedPath)
	changedEncoder := json.NewEncoder(changedFile)
	changedCount := 0

	err = forEachPartitionEntity(partition.Path, func(entity api.Entity) error {
		bpID, _ := entity["blueprint"].(string)
		entityID, _ := entity["identifier"].(string)
		if bpID == "" || entityID == "" {
			return nil
		}
		if isProtectedBlueprint(bpID, opts.IncludeRuleResults) || inheritedOwnershipBPs[bpID] || blueprintsToSkip[bpID] {
			return nil
		}
		currentEntity, exists := currentMap[entityID]
		if exists && resourcesEqual(entity, currentEntity, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"}) {
			return nil
		}
		if dryRun {
			if exists {
				result.EntitiesUpdated++
			} else {
				result.EntitiesCreated++
			}
			return nil
		}
		changedCount++
		return changedEncoder.Encode(entity)
	})
	if closeErr := changedFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if dryRun || changedCount == 0 {
		return nil
	}

	i.reportProgress("Entities Phase 1", 0, changedCount)
	processedCount := 0
	var progressMu sync.Mutex
	successfulEntities := make(map[string]bool)
	var successMu sync.Mutex
	if err := i.processChangedEntityFile(ctx, changedPath, false, result, successfulEntities, &successMu, "Entities Phase 1", changedCount, &processedCount, &progressMu); err != nil {
		return err
	}

	relationCount, err := countSuccessfulRelationEntities(changedPath, successfulEntities, &successMu)
	if err != nil {
		return err
	}
	if relationCount == 0 {
		return nil
	}
	i.reportProgress("Entities Phase 2 (relations)", 0, relationCount)
	phase2Count := 0
	var phase2ProgressMu sync.Mutex
	return i.processChangedEntityFile(ctx, changedPath, true, nil, successfulEntities, &successMu, "Entities Phase 2 (relations)", relationCount, &phase2Count, &phase2ProgressMu)
}

func (i *Importer) processChangedEntityFile(
	ctx context.Context,
	path string,
	withRelations bool,
	result *Result,
	successfulEntities map[string]bool,
	successMu *sync.Mutex,
	phaseName string,
	total int,
	processedCount *int,
	progressMu *sync.Mutex,
) error {
	batches := make(map[string][]api.Entity)
	flush := func(bpID string) {
		chunk := batches[bpID]
		if len(chunk) == 0 {
			return
		}
		i.processBulkChunk(ctx, bpID, chunk, withRelations, result, successfulEntities, successMu, phaseName, total, processedCount, progressMu)
		batches[bpID] = nil
	}
	err := forEachPartitionEntity(path, func(entity api.Entity) error {
		bpID, _ := entity["blueprint"].(string)
		entityID, _ := entity["identifier"].(string)
		if bpID == "" || entityID == "" {
			return nil
		}
		if withRelations {
			successMu.Lock()
			success := successfulEntities[fmt.Sprintf("%s:%s", bpID, entityID)]
			successMu.Unlock()
			if !success || !HasEntityRelations(entity) {
				return nil
			}
		} else {
			entity = StripEntityRelations(entity)
		}
		batches[bpID] = append(batches[bpID], entity)
		if len(batches[bpID]) >= EntityBulkBatchSize {
			flush(bpID)
		}
		return nil
	})
	for bpID := range batches {
		flush(bpID)
	}
	return err
}

func countSuccessfulRelationEntities(path string, successfulEntities map[string]bool, successMu *sync.Mutex) (int, error) {
	count := 0
	err := forEachPartitionEntity(path, func(entity api.Entity) error {
		if !HasEntityRelations(entity) {
			return nil
		}
		bpID, _ := entity["blueprint"].(string)
		entityID, _ := entity["identifier"].(string)
		successMu.Lock()
		success := successfulEntities[fmt.Sprintf("%s:%s", bpID, entityID)]
		successMu.Unlock()
		if success {
			count++
		}
		return nil
	})
	return count, err
}
