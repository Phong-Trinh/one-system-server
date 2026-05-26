package services

import (
	"context"

	"one-system-server/internal/domain/models"
)

// PRService defines operations for Purchase Requisition management.
//
// PRs are the entry point for CapEx procurement (equipment, long-term assets).
// Routine OpEx replenishment bypasses PR entirely — it goes directly to an
// auto-draft PurchaseOrder when the ROP engine fires.
//
// Status flow:
//
//	DRAFT → PENDING_HQ_APPROVAL → APPROVED → CONVERTED_TO_PO
//	                            ↘ REJECTED
type PRService interface {
	// SubmitPR is called by a Store or Factory Manager to submit a CapEx request to HQ.
	// At least one line must be provided; each line must have either ItemID or EquipmentTypeID.
	SubmitPR(ctx context.Context, req SubmitPRRequest) (*models.PurchaseRequisition, error)

	// ApprovePR is called by HQ to approve a pending PR.
	// Sets status = APPROVED. Does NOT create a PO — that is a separate step.
	ApprovePR(ctx context.Context, prID, reviewerStaffID string, note *string) error

	// RejectPR is called by HQ to reject a pending PR with a mandatory reason.
	RejectPR(ctx context.Context, prID, reviewerStaffID, reason string) error

	// GetPR returns the PR header and its line items.
	GetPR(ctx context.Context, prID string) (*models.PurchaseRequisition, []*models.PRLine, error)

	// ListPRsByNode returns all PRs submitted from a given requester node.
	ListPRsByNode(ctx context.Context, nodeID string) ([]*models.PurchaseRequisition, error)

	// ListPendingByOrg returns all PRs awaiting HQ approval — used by the HQ dashboard.
	ListPendingByOrg(ctx context.Context, orgID string) ([]*models.PurchaseRequisition, error)
}

// SubmitPRRequest carries the data needed to create a new PurchaseRequisition.
type SubmitPRRequest struct {
	OrgID           string
	RequesterNodeID string
	StaffID         string
	Justification   string
	Lines           []PRLineInput
}

// PRLineInput is a single line item within a new PR submission.
// Either ItemID or EquipmentTypeID must be set — not both.
type PRLineInput struct {
	ItemID                *string // FK → Item (OpEx exceptional request)
	EquipmentTypeID       *string // FK → EquipmentType (CapEx asset)
	ProposedEquipmentName *string // Name of the brand new equipment type if not yet registered
	ProposedCapacityUnit  *string // Capacity unit (e.g., "tray", "liter") of the new equipment type
	ExpectedCapacity      *float64 // Expected capacity size of the requested machine to buy
	Qty                   float64
	UnitOfMeasure         string
	EstimatedUnitPrice    float64
	Justification         string
}
