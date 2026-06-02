package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// MachineRegistrationInput carries data needed to register an Asset as a Machine.
type MachineRegistrationInput struct {
	MachineID       string  // e.g., "M_FRYER_03" — caller-assigned unique ID
	EquipmentTypeID string  // FK → EquipmentType (should match Asset.EquipmentTypeID)
	MaxCapacity     float64 // Total capacity in EquipmentType.capacity_unit
}

// AssetUseCase manages the CapEx Asset lifecycle and its synchronization
// to the linked Machine in the Production domain.
//
// The Asset record is the financial and administrative anchor created after
// a PR_TRIGGERED PurchaseOrder is payment-settled (3-Way Matching done).
//
// Workflow (from PO to Machine):
//  1. POUseCase.SettlePayment (PR_TRIGGERED) → AutoCreateAsset (PENDING_REGISTRATION)
//  2. Node Manager → RegisterAsMachine → Asset IN_USE, Machine IDLE
//  3. Asset lifecycle events → SyncAssetStatus → Machine status synchronized
type AssetUseCase interface {
	// AutoCreateAsset is called by POUseCase.SettlePayment for PR_TRIGGERED POs.
	// Creates the Asset in PENDING_REGISTRATION status.
	// The Asset is not yet a Machine — the node manager must call RegisterAsMachine.
	AutoCreateAsset(ctx context.Context, purOID, grID string) (*models.Asset, error)

	// RegisterAsMachine links an Asset to a new Machine entry.
	// Validates that the Asset is in PENDING_REGISTRATION status.
	// On success:
	//   - Machine record is created (status = IDLE)
	//   - Asset.LinkedMachineID is set
	//   - Asset.Status transitions PENDING_REGISTRATION → IN_USE
	RegisterAsMachine(ctx context.Context, assetID string, input MachineRegistrationInput) (*models.Machine, error)

	// SyncAssetStatus propagates an Asset status change to the linked Machine.
	// Called by node managers or maintenance workflows.
	// Valid transitions:
	//   IN_USE            → UNDER_MAINTENANCE  (Machine.Status = UNDER_MAINTENANCE)
	//   UNDER_MAINTENANCE → IN_USE             (Machine.Status = IDLE)
	//   IN_USE | UNDER_MAINTENANCE → DECOMMISSIONED (Machine.Status = DECOMMISSIONED; excluded from allocation)
	SyncAssetStatus(ctx context.Context, assetID string, newStatus models.AssetStatus) error

	GetAsset(ctx context.Context, assetID string) (*models.Asset, error)
	ListAssetsByNode(ctx context.Context, nodeID string) ([]*models.Asset, error)
}

// assetUseCase implements AssetUseCase.
type assetUseCase struct {
	assetRepo   services.AssetRepository
	machineRepo services.MachineRepository
	purORepo    services.PurchaseOrderRepository
	grRepo      services.GoodsReceiptRepository
	prRepo      services.PurchaseRequisitionRepository
	prLineRepo  services.PRLineRepository
}

func newAssetUseCase(
	assetRepo services.AssetRepository,
	machineRepo services.MachineRepository,
	purORepo services.PurchaseOrderRepository,
	grRepo services.GoodsReceiptRepository,
	prRepo services.PurchaseRequisitionRepository,
	prLineRepo services.PRLineRepository,
) AssetUseCase {
	return &assetUseCase{
		assetRepo:   assetRepo,
		machineRepo: machineRepo,
		purORepo:    purORepo,
		grRepo:      grRepo,
		prRepo:      prRepo,
		prLineRepo:  prLineRepo,
	}
}

// AutoCreateAsset creates an Asset record after a PR_TRIGGERED PO is payment-settled.
// Reads the PR lines to derive the EquipmentTypeID for the asset.
func (uc *assetUseCase) AutoCreateAsset(ctx context.Context, purOID, grID string) (*models.Asset, error) {
	purO, err := uc.purORepo.FindByID(ctx, purOID)
	if err != nil || purO == nil {
		return nil, fmt.Errorf("asset: AutoCreateAsset: PO %s not found: %w", purOID, err)
	}
	if purO.TriggerType != models.PurOTriggerPR {
		return nil, fmt.Errorf("asset: AutoCreateAsset: PO %s is not PR_TRIGGERED (type: %s)", purOID, purO.TriggerType)
	}
	if purO.PRID == nil {
		return nil, fmt.Errorf("asset: AutoCreateAsset: PO %s has nil PR reference", purOID)
	}

	gr, err := uc.grRepo.FindByID(ctx, grID)
	if err != nil || gr == nil {
		return nil, fmt.Errorf("asset: AutoCreateAsset: GR %s not found: %w", grID, err)
	}

	// Find the EquipmentTypeID from the PR lines (first CapEx line).
	prLines, err := uc.prLineRepo.ListByPR(ctx, *purO.PRID)
	if err != nil {
		return nil, fmt.Errorf("asset: AutoCreateAsset: list PR lines: %w", err)
	}

	var equipmentTypeID string
	var maxCapacity float64
	for _, l := range prLines {
		if l.EquipmentTypeID != nil {
			equipmentTypeID = *l.EquipmentTypeID
			if l.ExpectedCapacity != nil {
				maxCapacity = *l.ExpectedCapacity
			}
			break
		}
	}
	if equipmentTypeID == "" {
		return nil, fmt.Errorf("asset: AutoCreateAsset: no EquipmentTypeID found on PR %s lines", *purO.PRID)
	}
	if maxCapacity <= 0 {
		return nil, fmt.Errorf("asset: AutoCreateAsset: PR %s has no expected_capacity — cannot auto-register machine", *purO.PRID)
	}

	now := time.Now()

	// System auto-generates the machine ID. Clients never assign machine IDs for CapEx assets.
	machineID := "MACH-" + uuid.NewString()[:8]
	machine := &models.Machine{
		ID:              machineID,
		EquipmentTypeID: equipmentTypeID,
		NodeID:          purO.DeliveryToNodeID,
		MaxCapacity:     maxCapacity,
		Status:          models.MachineIdle,
		LinkedAssetID:   nil, // back-linked after asset is persisted
	}
	if err := uc.machineRepo.Create(ctx, machine); err != nil {
		return nil, fmt.Errorf("asset: AutoCreateAsset: create machine: %w", err)
	}

	// Asset starts as IN_USE — it's already physically delivered and the machine is live.
	asset := &models.Asset{
		ID:               uuid.NewString(),
		OrgID:            purO.OrgID,
		EquipmentTypeID:  equipmentTypeID,
		NodeID:           purO.DeliveryToNodeID,
		LinkedPRID:       *purO.PRID,
		LinkedPurOID:     purOID,
		LinkedGRID:       grID,
		LinkedMachineID:  &machineID,
		AcquisitionDate:  gr.ReceivedAt.Truncate(24 * time.Hour),
		Status:           models.AssetInUse,
		Depreciation:     models.DepreciationStraightLine,
		UsefulLifeYears:  5,
		CurrentBookValue: 0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := uc.assetRepo.Create(ctx, asset); err != nil {
		return nil, fmt.Errorf("asset: AutoCreateAsset: persist asset: %w", err)
	}

	// Back-link machine → asset.
	machine.LinkedAssetID = &asset.ID
	if err := uc.machineRepo.Update(ctx, machine); err != nil {
		return nil, fmt.Errorf("asset: AutoCreateAsset: link machine to asset: %w", err)
	}

	return asset, nil
}

// RegisterAsMachine links an Asset to a new Machine and transitions the asset to IN_USE.
func (uc *assetUseCase) RegisterAsMachine(ctx context.Context, assetID string, input MachineRegistrationInput) (*models.Machine, error) {
	asset, err := uc.loadAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if asset.Status != models.AssetPendingRegistration {
		return nil, fmt.Errorf("asset: RegisterAsMachine: asset %s must be PENDING_REGISTRATION (current: %s)", assetID, asset.Status)
	}
	if input.MachineID == "" {
		return nil, fmt.Errorf("asset: RegisterAsMachine: machine_id is required")
	}
	if input.MaxCapacity <= 0 {
		return nil, fmt.Errorf("asset: RegisterAsMachine: max_capacity must be > 0")
	}

	eqTypeID := input.EquipmentTypeID
	if eqTypeID == "" {
		eqTypeID = asset.EquipmentTypeID
	}

	machine := &models.Machine{
		ID:              input.MachineID,
		EquipmentTypeID: eqTypeID,
		NodeID:          asset.NodeID,
		MaxCapacity:     input.MaxCapacity,
		Status:          models.MachineIdle,
		LinkedAssetID:   &assetID,
	}

	if err := uc.machineRepo.Create(ctx, machine); err != nil {
		return nil, fmt.Errorf("asset: RegisterAsMachine: create machine: %w", err)
	}

	// Link Asset ↔ Machine and transition to IN_USE.
	now := time.Now()
	asset.LinkedMachineID = &machine.ID
	asset.Status = models.AssetInUse
	asset.UpdatedAt = now

	if err := uc.assetRepo.Update(ctx, asset); err != nil {
		return nil, fmt.Errorf("asset: RegisterAsMachine: update asset: %w", err)
	}

	return machine, nil
}

// SyncAssetStatus propagates an Asset status change to the linked Machine.
// This keeps the production domain's Machine.Status in sync with the supply chain's Asset lifecycle.
func (uc *assetUseCase) SyncAssetStatus(ctx context.Context, assetID string, newStatus models.AssetStatus) error {
	asset, err := uc.loadAsset(ctx, assetID)
	if err != nil {
		return err
	}

	// Validate transition.
	switch newStatus {
	case models.AssetUnderMaintenance:
		if asset.Status != models.AssetInUse {
			return fmt.Errorf("asset: SyncAssetStatus: UNDER_MAINTENANCE requires current status IN_USE (current: %s)", asset.Status)
		}
	case models.AssetInUse:
		if asset.Status != models.AssetUnderMaintenance {
			return fmt.Errorf("asset: SyncAssetStatus: IN_USE transition requires current status UNDER_MAINTENANCE (current: %s)", asset.Status)
		}
	case models.AssetDecommissioned:
		if asset.Status == models.AssetPendingRegistration || asset.Status == models.AssetDecommissioned {
			return fmt.Errorf("asset: SyncAssetStatus: cannot decommission asset in status %s", asset.Status)
		}
	default:
		return fmt.Errorf("asset: SyncAssetStatus: unsupported target status %s", newStatus)
	}

	// Determine the corresponding Machine status.
	var machineStatus models.MachineStatus
	switch newStatus {
	case models.AssetUnderMaintenance:
		machineStatus = models.MachineUnderMaintenance
	case models.AssetInUse:
		machineStatus = models.MachineIdle
	case models.AssetDecommissioned:
		machineStatus = models.MachineDecommissioned
	}

	// Synchronize Machine status if a machine is linked.
	if asset.LinkedMachineID != nil {
		if err := uc.machineRepo.UpdateStatus(ctx, *asset.LinkedMachineID, machineStatus, nil); err != nil {
			return fmt.Errorf("asset: SyncAssetStatus: update machine %s status: %w", *asset.LinkedMachineID, err)
		}
	}

	// Update Asset status.
	now := time.Now()
	asset.Status = newStatus
	asset.UpdatedAt = now
	return uc.assetRepo.Update(ctx, asset)
}

func (uc *assetUseCase) GetAsset(ctx context.Context, assetID string) (*models.Asset, error) {
	return uc.loadAsset(ctx, assetID)
}

func (uc *assetUseCase) ListAssetsByNode(ctx context.Context, nodeID string) ([]*models.Asset, error) {
	return uc.assetRepo.FindByNode(ctx, nodeID)
}

func (uc *assetUseCase) loadAsset(ctx context.Context, id string) (*models.Asset, error) {
	asset, err := uc.assetRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("asset: load asset %s: %w", id, err)
	}
	if asset == nil {
		return nil, fmt.Errorf("asset: asset %s not found", id)
	}
	return asset, nil
}
