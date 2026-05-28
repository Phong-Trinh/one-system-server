package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ── Input DTOs ────────────────────────────────────────────────────────────────

// ITOLineInput is a single line item when creating an ITO.
type ITOLineInput struct {
	ItemID     string
	QtyOrdered float64 // In packaging units
	PkgUnit    string
	Conversion float64 // Base units per pkg_unit
}

// GoodsIssueInput carries dispatch details when the provider confirms a GoodsIssue.
type GoodsIssueInput struct {
	// Cross-site: driver info and media evidence are required.
	// Same-site: these fields are ignored (auto-generated GI with is_same_site=true).
	DriverName   string
	DriverPhone  string
	VehiclePlate string
	MediaURL     string
	ShippingFee  float64
	// Lines maps ITOLine.ItemID → quantity dispatched in base units.
	// Allows partial dispatch; remaining quantities stay on the ITO.
	Lines []GILineInput
}

// GILineInput is a single item dispatched in a GoodsIssue.
type GILineInput struct {
	ItemID    string
	QtyIssued float64 // Base units dispatched
}

// GoodsReceiptInput carries receipt details when the requester confirms receipt.
type GoodsReceiptInput struct {
	ReceivedByStaffID string
	Lines             []GRLineInput
}

// GRLineInput is a single item received in a GoodsReceipt.
type GRLineInput struct {
	ItemID      string
	QtyExpected float64 // Base units expected (from GI)
	QtyReceived float64 // Base units actually received (may be less due to transit damage)
}

// ── Interface ─────────────────────────────────────────────────────────────────

// ITOUseCase manages the Internal Transfer Order lifecycle.
//
// Two trigger paths:
//   - AUTO: ROP engine calls CreateAutoITO (status = AUTO_APPROVED immediately).
//   - MANUAL: Store Manager calls CreateManualITO (status = PENDING_APPROVAL by default,
//     or AUTO_APPROVED if the node policy allows it).
//
// Two logistics paths:
//   - Cross-site (provider.SiteID ≠ requester.SiteID): GI → IN_TRANSIT → GR.
//   - Same-site (same SiteID): 1-click, system auto-generates GI + GR simultaneously,
//     ITO jumps directly to COMPLETED (no IN_TRANSIT phase).
type ITOUseCase interface {
	// CreateAutoITO is called by the ROP engine when a node's stock hits SourcingInternalTransfer.
	CreateAutoITO(ctx context.Context, cfg *models.NodeItemConfig, qtyBU float64) (*models.InternalTransferOrder, error)

	// CreateManualITO allows a Store Manager to explicitly request a transfer.
	CreateManualITO(ctx context.Context, orgID, requesterNodeID, providerNodeID, staffID string, lines []ITOLineInput) (*models.InternalTransferOrder, error)

	// ApproveManualITO is called by a Factory/Area Manager to approve a pending manual ITO.
	ApproveManualITO(ctx context.Context, itoID string) error

	// RejectManualITO rejects a pending manual ITO.
	RejectManualITO(ctx context.Context, itoID, reason string) error

	// ConfirmGoodsIssue is called by the provider node to dispatch goods.
	// For cross-site: driver info + media URL required; triggers StockOut at provider.
	// For same-site: auto-generates a simplified GI + GR; immediately completes ITO.
	ConfirmGoodsIssue(ctx context.Context, itoID string, input GoodsIssueInput) (*models.GoodsIssue, error)

	// ConfirmGoodsReceipt is called by the requester node upon receiving goods.
	// Triggers StockIn for the received quantity.
	// If any line has qty_received < qty_expected → auto-creates a DiscrepancyTicket.
	ConfirmGoodsReceipt(ctx context.Context, itoID string, giID string, input GoodsReceiptInput) (*models.GoodsReceipt, error)

	GetITO(ctx context.Context, itoID string) (*models.InternalTransferOrder, []*models.ITOLine, error)
	ListITOsByNode(ctx context.Context, nodeID string) ([]*models.InternalTransferOrder, error)
	HasActiveITO(ctx context.Context, requesterNodeID, itemID string) (bool, error)
}

// ── Implementation ────────────────────────────────────────────────────────────

type itoUseCase struct {
	itoRepo  services.InternalTransferOrderRepository
	lineRepo services.ITOLineRepository
	giRepo   services.GoodsIssueRepository
	giLine   services.GoodsIssueLineRepository
	grRepo   services.GoodsReceiptRepository
	grLine   services.GoodsReceiptLineRepository
	dtRepo   services.DiscrepancyTicketRepository
	nodeRepo services.NodeRepository
	inv      services.InventoryService
}

func newITOUseCase(
	itoRepo services.InternalTransferOrderRepository,
	lineRepo services.ITOLineRepository,
	giRepo services.GoodsIssueRepository,
	giLine services.GoodsIssueLineRepository,
	grRepo services.GoodsReceiptRepository,
	grLine services.GoodsReceiptLineRepository,
	dtRepo services.DiscrepancyTicketRepository,
	nodeRepo services.NodeRepository,
	inv services.InventoryService,
) ITOUseCase {
	return &itoUseCase{
		itoRepo:  itoRepo,
		lineRepo: lineRepo,
		giRepo:   giRepo,
		giLine:   giLine,
		grRepo:   grRepo,
		grLine:   grLine,
		dtRepo:   dtRepo,
		nodeRepo: nodeRepo,
		inv:      inv,
	}
}

// ── CreateAutoITO ─────────────────────────────────────────────────────────────

func (uc *itoUseCase) CreateAutoITO(ctx context.Context, cfg *models.NodeItemConfig, qtyBU float64) (*models.InternalTransferOrder, error) {
	if cfg.ProviderNodeID == nil {
		return nil, fmt.Errorf("ito: CreateAutoITO: NodeItemConfig for node=%s item=%s has no provider_node_id set",
			cfg.NodeID, cfg.ItemID)
	}

	isSameSite, err := uc.isSameSite(ctx, cfg.NodeID, *cfg.ProviderNodeID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	ito := &models.InternalTransferOrder{
		ID:              uuid.NewString(),
		OrgID:           "", // populated by facade which has org context
		RequesterNodeID: cfg.NodeID,
		ProviderNodeID:  *cfg.ProviderNodeID,
		Trigger:         models.ITOTriggerROP,
		Status:          models.ITOAutoApproved,
		IsSameSite:      isSameSite,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := uc.itoRepo.Create(ctx, ito); err != nil {
		return nil, fmt.Errorf("ito: CreateAutoITO: persist: %w", err)
	}

	// Create a single line for the triggered item.
	line := &models.ITOLine{
		ID:           uuid.NewString(),
		ITOID:        ito.ID,
		ItemID:       cfg.ItemID,
		QtyOrdered:   qtyBU, // For auto ITOs we track in base units directly (PkgUnit = BU)
		PkgUnit:      "base_unit",
		Conversion:   1.0,
		QtyOrderedBU: qtyBU,
	}
	if err := uc.lineRepo.AddLine(ctx, line); err != nil {
		return nil, fmt.Errorf("ito: CreateAutoITO: add line: %w", err)
	}

	return ito, nil
}

// ── CreateManualITO ───────────────────────────────────────────────────────────

func (uc *itoUseCase) CreateManualITO(ctx context.Context, orgID, requesterNodeID, providerNodeID, staffID string, lines []ITOLineInput) (*models.InternalTransferOrder, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("ito: CreateManualITO: at least one line required")
	}

	isSameSite, err := uc.isSameSite(ctx, requesterNodeID, providerNodeID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	ito := &models.InternalTransferOrder{
		ID:              uuid.NewString(),
		OrgID:           orgID,
		RequesterNodeID: requesterNodeID,
		ProviderNodeID:  providerNodeID,
		Trigger:         models.ITOTriggerManual,
		// Manual ITOs start in PENDING_APPROVAL; provider can configure auto-approval per node policy.
		// For now, always PENDING_APPROVAL for manual requests.
		Status:      models.ITOPendingApproval,
		IsSameSite:  isSameSite,
		RequestedBy: &staffID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := uc.itoRepo.Create(ctx, ito); err != nil {
		return nil, fmt.Errorf("ito: CreateManualITO: persist: %w", err)
	}

	for _, l := range lines {
		if l.Conversion <= 0 {
			return nil, fmt.Errorf("ito: CreateManualITO: line item %s has invalid conversion %v", l.ItemID, l.Conversion)
		}
		line := &models.ITOLine{
			ID:           uuid.NewString(),
			ITOID:        ito.ID,
			ItemID:       l.ItemID,
			QtyOrdered:   l.QtyOrdered,
			PkgUnit:      l.PkgUnit,
			Conversion:   l.Conversion,
			QtyOrderedBU: l.QtyOrdered * l.Conversion,
		}
		if err := uc.lineRepo.AddLine(ctx, line); err != nil {
			return nil, fmt.Errorf("ito: CreateManualITO: add line %s: %w", l.ItemID, err)
		}
	}

	return ito, nil
}

// ── ApproveManualITO / RejectManualITO ───────────────────────────────────────

func (uc *itoUseCase) ApproveManualITO(ctx context.Context, itoID string) error {
	ito, err := uc.loadITO(ctx, itoID)
	if err != nil {
		return err
	}
	if ito.Status != models.ITOPendingApproval {
		return fmt.Errorf("ito: ApproveManualITO: ITO %s is not in PENDING_APPROVAL (current: %s)", itoID, ito.Status)
	}
	return uc.itoRepo.UpdateStatus(ctx, itoID, models.ITOAutoApproved)
}

func (uc *itoUseCase) RejectManualITO(ctx context.Context, itoID, reason string) error {
	ito, err := uc.loadITO(ctx, itoID)
	if err != nil {
		return err
	}
	if ito.Status != models.ITOPendingApproval {
		return fmt.Errorf("ito: RejectManualITO: ITO %s is not in PENDING_APPROVAL (current: %s)", itoID, ito.Status)
	}
	return uc.itoRepo.UpdateStatus(ctx, itoID, models.ITOCancelled)
}

// ── ConfirmGoodsIssue ────────────────────────────────────────────────────────

// ConfirmGoodsIssue dispatches goods from the provider.
// Same-site path: auto-generates GI + GR and completes the ITO immediately (1-click).
// Cross-site path: creates a GI, fires StockOut at provider, transitions ITO to IN_TRANSIT.
func (uc *itoUseCase) ConfirmGoodsIssue(ctx context.Context, itoID string, input GoodsIssueInput) (*models.GoodsIssue, error) {
	ito, err := uc.loadITO(ctx, itoID)
	if err != nil {
		return nil, err
	}
	if ito.Status != models.ITOAutoApproved {
		return nil, fmt.Errorf("ito: ConfirmGoodsIssue: ITO %s must be in AUTO_APPROVED status (current: %s)", itoID, ito.Status)
	}

	// Validate cross-site requirements.
	if !ito.IsSameSite {
		if input.DriverName == "" || input.VehiclePlate == "" || input.MediaURL == "" {
			return nil, fmt.Errorf("ito: ConfirmGoodsIssue: cross-site dispatch requires driver_name, vehicle_plate, and media_url")
		}
	}

	now := time.Now()
	gi := &models.GoodsIssue{
		ID:            uuid.NewString(),
		RefType:       models.GoodsIssueRefITO,
		RefID:         itoID,
		IssuingNodeID: ito.ProviderNodeID,
		DriverName:    input.DriverName,
		DriverPhone:   input.DriverPhone,
		VehiclePlate:  input.VehiclePlate,
		MediaURL:      input.MediaURL,
		ShippingFee:   input.ShippingFee,
		Status:        models.GoodsIssueConfirmed,
		IssuedAt:      &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if ito.IsSameSite {
		gi.ShippingFee = 0
	}

	if err := uc.giRepo.Create(ctx, gi); err != nil {
		return nil, fmt.Errorf("ito: ConfirmGoodsIssue: persist GI: %w", err)
	}

	// Create GI lines and trigger StockOut at the provider for each item.
	for _, l := range input.Lines {
		giLine := &models.GoodsIssueLine{
			ID:        uuid.NewString(),
			GIID:      gi.ID,
			ItemID:    l.ItemID,
			QtyIssued: l.QtyIssued,
		}
		if err := uc.giLine.AddLine(ctx, giLine); err != nil {
			return nil, fmt.Errorf("ito: ConfirmGoodsIssue: add GI line %s: %w", l.ItemID, err)
		}
		// StockOut at provider — ROP result returned to facade for document creation.
		// We discard it here; the facade's trigger path handles it when needed.
		if _, err := uc.inv.StockOut(ctx, ito.ProviderNodeID, l.ItemID, l.QtyIssued); err != nil {
			return nil, fmt.Errorf("ito: ConfirmGoodsIssue: StockOut provider node %s item %s: %w",
				ito.ProviderNodeID, l.ItemID, err)
		}
	}

	// Same-site: auto-generate GR and complete ITO immediately (1-click path).
	if ito.IsSameSite {
		grLines := make([]GRLineInput, 0, len(input.Lines))
		for _, l := range input.Lines {
			grLines = append(grLines, GRLineInput{
				ItemID:      l.ItemID,
				QtyExpected: l.QtyIssued,
				QtyReceived: l.QtyIssued, // Same-site: no transit loss possible
			})
		}
		autoGRInput := GoodsReceiptInput{
			ReceivedByStaffID: "system", // auto-generated
			Lines:             grLines,
		}
		if _, err := uc.ConfirmGoodsReceipt(ctx, itoID, gi.ID, autoGRInput); err != nil {
			return nil, fmt.Errorf("ito: ConfirmGoodsIssue: same-site auto-GR: %w", err)
		}
		// ITO → COMPLETED is handled inside ConfirmGoodsReceipt for same-site.
	} else {
		// Cross-site: ITO moves to IN_TRANSIT; GR will be created separately.
		if err := uc.itoRepo.UpdateStatus(ctx, itoID, models.ITOInTransit); err != nil {
			return nil, fmt.Errorf("ito: ConfirmGoodsIssue: update ITO status: %w", err)
		}
	}

	return gi, nil
}

// ── ConfirmGoodsReceipt ───────────────────────────────────────────────────────

// ConfirmGoodsReceipt is called by the requester when goods arrive.
// Triggers StockIn for received quantity. Auto-creates DiscrepancyTicket if qty_received < qty_expected.
func (uc *itoUseCase) ConfirmGoodsReceipt(ctx context.Context, itoID string, giID string, input GoodsReceiptInput) (*models.GoodsReceipt, error) {
	ito, err := uc.loadITO(ctx, itoID)
	if err != nil {
		return nil, err
	}

	hasDiscrepancy := false
	for _, l := range input.Lines {
		if l.QtyReceived < l.QtyExpected {
			hasDiscrepancy = true
			break
		}
	}

	grStatus := models.GoodsReceiptConfirmed
	if hasDiscrepancy {
		grStatus = models.GoodsReceiptDiscrepancy
	}

	now := time.Now()
	gr := &models.GoodsReceipt{
		ID:              uuid.NewString(),
		RefType:         models.GoodsReceiptRefITO,
		RefID:           itoID,
		ReceivingNodeID: ito.RequesterNodeID,
		Status:          grStatus,
		ReceivedBy:      input.ReceivedByStaffID,
		ReceivedAt:      &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := uc.grRepo.Create(ctx, gr); err != nil {
		return nil, fmt.Errorf("ito: ConfirmGoodsReceipt: persist GR: %w", err)
	}

	for _, l := range input.Lines {
		grLine := &models.GoodsReceiptLine{
			ID:          uuid.NewString(),
			GRID:        gr.ID,
			ItemID:      l.ItemID,
			QtyExpected: l.QtyExpected,
			QtyReceived: l.QtyReceived,
		}
		if err := uc.grLine.AddLine(ctx, grLine); err != nil {
			return nil, fmt.Errorf("ito: ConfirmGoodsReceipt: add GR line %s: %w", l.ItemID, err)
		}

		// StockIn at requester using the actually received quantity.
		if l.QtyReceived > 0 {
			if err := uc.inv.StockIn(ctx, ito.RequesterNodeID, l.ItemID, l.QtyReceived); err != nil {
				return nil, fmt.Errorf("ito: ConfirmGoodsReceipt: StockIn item %s: %w", l.ItemID, err)
			}
		}

		// Auto-create DiscrepancyTicket for any line with a shortage.
		if l.QtyReceived < l.QtyExpected {
			dt := &models.DiscrepancyTicket{
				ID:         uuid.NewString(),
				GRID:       gr.ID,
				ItemID:     l.ItemID,
				QtyMissing: l.QtyExpected - l.QtyReceived,
				QtyDamaged: 0,
				Status:     models.DiscrepancyOpen,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if err := uc.dtRepo.Create(ctx, dt); err != nil {
				return nil, fmt.Errorf("ito: ConfirmGoodsReceipt: create DiscrepancyTicket item %s: %w", l.ItemID, err)
			}
		}
	}

	// Transition ITO to COMPLETED or DISCREPANCY.
	finalStatus := models.ITOCompleted
	if hasDiscrepancy {
		// ITO is still COMPLETED — the missing/damaged qty is tracked in the DiscrepancyTicket.
		// The received portion has already been stocked in.
		finalStatus = models.ITOCompleted
	}
	if err := uc.itoRepo.UpdateStatus(ctx, itoID, finalStatus); err != nil {
		return nil, fmt.Errorf("ito: ConfirmGoodsReceipt: update ITO status: %w", err)
	}

	return gr, nil
}

// ── Getters ───────────────────────────────────────────────────────────────────

func (uc *itoUseCase) GetITO(ctx context.Context, itoID string) (*models.InternalTransferOrder, []*models.ITOLine, error) {
	ito, err := uc.loadITO(ctx, itoID)
	if err != nil {
		return nil, nil, err
	}
	lines, err := uc.lineRepo.ListByITO(ctx, itoID)
	return ito, lines, err
}

func (uc *itoUseCase) ListITOsByNode(ctx context.Context, nodeID string) ([]*models.InternalTransferOrder, error) {
	return uc.itoRepo.FindByNode(ctx, nodeID)
}

func (uc *itoUseCase) HasActiveITO(ctx context.Context, requesterNodeID, itemID string) (bool, error) {
	itos, err := uc.itoRepo.FindByNode(ctx, requesterNodeID)
	if err != nil {
		return false, err
	}
	for _, ito := range itos {
		if ito.RequesterNodeID != requesterNodeID {
			continue // Only care about ITOs where this node is requesting the stock
		}
		if ito.Status == models.ITOCompleted || ito.Status == models.ITOCancelled {
			continue // Not active
		}
		
		lines, err := uc.lineRepo.ListByITO(ctx, ito.ID)
		if err != nil {
			return false, err
		}
		for _, l := range lines {
			if l.ItemID == itemID {
				return true, nil
			}
		}
	}
	return false, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (uc *itoUseCase) loadITO(ctx context.Context, id string) (*models.InternalTransferOrder, error) {
	ito, err := uc.itoRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ito: load ITO %s: %w", id, err)
	}
	if ito == nil {
		return nil, fmt.Errorf("ito: ITO %s not found", id)
	}
	return ito, nil
}

// isSameSite compares the SiteID of two nodes. Returns false if either node has no SiteID set.
func (uc *itoUseCase) isSameSite(ctx context.Context, nodeAID, nodeBID string) (bool, error) {
	a, err := uc.nodeRepo.FindByID(ctx, nodeAID)
	if err != nil || a == nil {
		return false, fmt.Errorf("ito: isSameSite: load node %s: %w", nodeAID, err)
	}
	b, err := uc.nodeRepo.FindByID(ctx, nodeBID)
	if err != nil || b == nil {
		return false, fmt.Errorf("ito: isSameSite: load node %s: %w", nodeBID, err)
	}
	if a.SiteID == nil || b.SiteID == nil {
		return false, nil
	}
	return *a.SiteID == *b.SiteID, nil
}
