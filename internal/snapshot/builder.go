package snapshot

import (
	"nofx/config"
	derivsv1 "nofx/internal/features/derivs/v1"
	"nofx/pkg/types"
)

// Snapshot is attached to a market symbol and contains feature namespaces.
type Snapshot struct {
	Symbol   string          `json:"symbol"`
	Features SnapshotContent `json:"features"`
}

// SnapshotContent groups derivs and future feature namespaces.
type SnapshotContent struct {
	Derivs *types.DerivsFeatures `json:"derivs,omitempty"`
}

// Builder orchestrates feature namespaces for a given symbol.
type Builder struct {
	derivs *derivsv1.Builder
}

// NewBuilder instantiates the snapshot builder tree.
func NewBuilder(cfg *config.DerivsV1Config, store derivsv1.Store) *Builder {
	return &Builder{derivs: derivsv1.NewBuilder(cfg, store)}
}

// Build assembles a snapshot for the provided symbol.
func (b *Builder) Build(symbol string) (*Snapshot, error) {
	snap := &Snapshot{Symbol: symbol}
	if b == nil || b.derivs == nil {
		return snap, nil
	}
	derivs, err := b.derivs.Build(symbol)
	if err != nil {
		return nil, err
	}
	if derivs != nil {
		snap.Features.Derivs = derivs
	}
	return snap, nil
}
