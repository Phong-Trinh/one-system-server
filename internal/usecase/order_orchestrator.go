package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// OrchestratorConfig holds the tunable knobs for auto-pooling behavior.
// These can be loaded from config.yaml and overridden per node in the future.
type OrchestratorConfig struct {
	// MaxPoolWaitSeconds: max time an order waits in pool before forced flush.
	// Default: 30s. Think of this as the max acceptable latency before the
	// kitchen even starts working on an order.
	MaxPoolWaitSeconds int

	// MinBatchWindowSeconds: minimum time to wait before flushing a pool,
	// allowing more orders to arrive and be grouped together.
	// Default: 8s. Prevents premature single-order decomposition.
	MinBatchWindowSeconds int

	// UrgencyFlushThreshold: fraction of MaxPoolWaitSeconds at which an order
	// is force-flushed regardless of batch potential. Range: [0.0, 1.0].
	// Default: 0.85 (flush when 85% of max wait time has elapsed).
	UrgencyFlushThreshold float64

	// TickIntervalSeconds: how often the background worker checks all pools.
	// Default: 5s.
	TickIntervalSeconds int
}

func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		MaxPoolWaitSeconds:    30,
		MinBatchWindowSeconds: 8,
		UrgencyFlushThreshold: 0.85,
		TickIntervalSeconds:   5,
	}
}

// poolEntry wraps a PO with its enqueue timestamp.
type poolEntry struct {
	po         *models.ProductionOrder
	enqueuedAt time.Time
}

// OrderPoolingOrchestrator manages the intelligent pooling and
// auto-decomposition of ProductionOrders before they enter the KDS.
//
// It runs a background goroutine that periodically evaluates the pool
// and flushes orders that are urgent enough or have been waiting long enough.
// It also exposes TriggerFlushForNode for immediate capacity-aware flushing
// when a machine completes a batch.
type OrderPoolingOrchestrator struct {
	mu           sync.Mutex
	pool         map[string][]*poolEntry // nodeID → pending entries
	cfg          OrchestratorConfig
	allocationUC AllocationUseCase
	poRepo       services.ProductionOrderRepository
}

// NewOrderPoolingOrchestrator constructs the orchestrator with all required deps.
func NewOrderPoolingOrchestrator(
	allocationUC AllocationUseCase,
	poRepo services.ProductionOrderRepository,
	cfg OrchestratorConfig,
) *OrderPoolingOrchestrator {
	// Apply defaults for zero values
	if cfg.MaxPoolWaitSeconds == 0 {
		cfg.MaxPoolWaitSeconds = 30
	}
	if cfg.MinBatchWindowSeconds == 0 {
		cfg.MinBatchWindowSeconds = 8
	}
	if cfg.UrgencyFlushThreshold == 0 {
		cfg.UrgencyFlushThreshold = 0.85
	}
	if cfg.TickIntervalSeconds == 0 {
		cfg.TickIntervalSeconds = 5
	}

	return &OrderPoolingOrchestrator{
		pool:         make(map[string][]*poolEntry),
		cfg:          cfg,
		allocationUC: allocationUC,
		poRepo:       poRepo,
	}
}

// Enqueue adds a newly created PO to the pool for its node.
// Must be called immediately after CreateProductionOrder succeeds.
// This is the single entry point — no manual "decompose" action needed.
func (o *OrderPoolingOrchestrator) Enqueue(po *models.ProductionOrder) {
	o.mu.Lock()
	defer o.mu.Unlock()

	entry := &poolEntry{
		po:         po,
		enqueuedAt: time.Now(),
	}
	o.pool[po.NodeID] = append(o.pool[po.NodeID], entry)

	log.Info().
		Str("po_id", po.ID).
		Str("node_id", po.NodeID).
		Int("pool_size", len(o.pool[po.NodeID])).
		Msg("[Orchestrator] Order enqueued into pool")
}

// PoolStatus returns a snapshot of pooled orders per node for the UI.
// Used by GET /kds/pool to render the "waiting" column.
func (o *OrderPoolingOrchestrator) PoolStatus() map[string][]*PooledOrderView {
	o.mu.Lock()
	defer o.mu.Unlock()

	now := time.Now()
	maxWait := time.Duration(o.cfg.MaxPoolWaitSeconds) * time.Second
	minWindow := time.Duration(o.cfg.MinBatchWindowSeconds) * time.Second
	result := make(map[string][]*PooledOrderView)

	for nodeID, entries := range o.pool {
		for _, e := range entries {
			waited := now.Sub(e.enqueuedAt)
			remaining := maxWait - waited
			if remaining < 0 {
				remaining = 0
			}
			result[nodeID] = append(result[nodeID], &PooledOrderView{
				POID:              e.po.ID,
				NodeID:            nodeID,
				EnqueuedAt:        e.enqueuedAt,
				SecondsUntilFlush: int(remaining.Seconds()),
				WaitedSeconds:     int(waited.Seconds()),
				MinWindowSeconds:  int(minWindow.Seconds()),
				MaxWaitSeconds:    int(maxWait.Seconds()),
			})
		}
	}
	return result
}

// PooledOrderView is the UI-friendly representation of a pooled order.
type PooledOrderView struct {
	POID              string    `json:"po_id"`
	NodeID            string    `json:"node_id"`
	EnqueuedAt        time.Time `json:"enqueued_at"`
	SecondsUntilFlush int       `json:"seconds_until_flush"` // Countdown for UI
	WaitedSeconds     int       `json:"waited_seconds"`      // Time already spent in pool
	MinWindowSeconds  int       `json:"min_window_seconds"`  // Batching window (e.g. 8s)
	MaxWaitSeconds    int       `json:"max_wait_seconds"`    // Absolute max wait (e.g. 30s)
}

// Start launches the background goroutine that periodically checks all pools.
// Call this once during application startup.
func (o *OrderPoolingOrchestrator) Start(ctx context.Context) {
	interval := time.Duration(o.cfg.TickIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()
		log.Info().
			Int("tick_interval_s", o.cfg.TickIntervalSeconds).
			Int("max_pool_wait_s", o.cfg.MaxPoolWaitSeconds).
			Int("min_batch_window_s", o.cfg.MinBatchWindowSeconds).
			Msg("[Orchestrator] Auto-flush background worker started")

		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("[Orchestrator] Background worker stopped")
				return
			case <-ticker.C:
				o.checkAndFlushAll(ctx)
			}
		}
	}()
}

// TriggerFlushForNode is called after a ConfirmCompletion to immediately
// re-evaluate whether pooled orders can fill the newly freed machine slot.
// Runs in a separate goroutine to not block the HTTP response.
func (o *OrderPoolingOrchestrator) TriggerFlushForNode(ctx context.Context, nodeID string) {
	go func() {
		log.Info().Str("node_id", nodeID).Msg("[Orchestrator] Machine-idle flush triggered")
		o.flushNode(ctx, nodeID)
	}()
}

// ── Internal ─────────────────────────────────────────────────────────────────

func (o *OrderPoolingOrchestrator) checkAndFlushAll(ctx context.Context) {
	o.mu.Lock()
	nodeIDs := make([]string, 0, len(o.pool))
	for nodeID, entries := range o.pool {
		if len(entries) > 0 {
			nodeIDs = append(nodeIDs, nodeID)
		}
	}
	o.mu.Unlock()

	for _, nodeID := range nodeIDs {
		o.flushNode(ctx, nodeID)
	}
}

func (o *OrderPoolingOrchestrator) flushNode(ctx context.Context, nodeID string) {
	o.mu.Lock()

	entries := o.pool[nodeID]
	if len(entries) == 0 {
		o.mu.Unlock()
		return
	}

	now := time.Now()
	maxWait := time.Duration(o.cfg.MaxPoolWaitSeconds) * time.Second
	minWindow := time.Duration(o.cfg.MinBatchWindowSeconds) * time.Second

	shouldFlushAll := false

	// Check if ANY order in the pool has reached a flush threshold
	for _, e := range entries {
		waitTime := now.Sub(e.enqueuedAt)

		if e.po.DeadlineAt != nil && now.After(*e.po.DeadlineAt) {
			log.Warn().Str("po_id", e.po.ID).Msg("[Orchestrator] Deadline exceeded — force flushing pool")
			shouldFlushAll = true
			break
		}

		urgency := float64(waitTime) / float64(maxWait)

		if urgency >= o.cfg.UrgencyFlushThreshold || waitTime >= maxWait || waitTime >= minWindow {
			shouldFlushAll = true
			break
		}
	}

	var toFlush []*poolEntry
	if shouldFlushAll {
		toFlush = entries
		o.pool[nodeID] = nil // Clear the pool
	} else {
		toFlush = nil
		// Keep everything in the pool
	}

	o.mu.Unlock()

	if len(toFlush) == 0 {
		return
	}

	log.Info().
		Str("node_id", nodeID).
		Int("count", len(toFlush)).
		Msg("[Orchestrator] Flushing ALL pooled orders to kitchen for batching")

	for _, e := range toFlush {
		// Update PO status to IN_PROGRESS before decomposing
		if err := o.poRepo.UpdateStatus(ctx, e.po.ID, models.POInProgress, nil); err != nil {
			log.Error().Err(err).Str("po_id", e.po.ID).Msg("[Orchestrator] Failed to set PO IN_PROGRESS — re-enqueuing")
			o.Enqueue(e.po)
			continue
		}
		// Refresh PO struct
		e.po.Status = models.POInProgress

		if err := o.allocationUC.DecomposePO(ctx, e.po.ID); err != nil {
			log.Error().Err(err).Str("po_id", e.po.ID).Msg("[Orchestrator] DecomposePO failed")
		} else {
			log.Info().Str("po_id", e.po.ID).Msg("[Orchestrator] ✅ PO decomposed and pushed to KDS")
		}
	}
}
