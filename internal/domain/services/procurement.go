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
	OrgID           string        `json:"org_id"`
	RequesterNodeID string        `json:"requester_node_id"`
	StaffID         string        `json:"staff_id"`
	Justification   string        `json:"justification"`
	Lines           []PRLineInput `json:"lines"`
}

// PRLineInput is a single line item within a new PR submission.
// Either ItemID or EquipmentTypeID must be set — not both.
type PRLineInput struct {
	ItemID                *string  `json:"item_id"`
	EquipmentTypeID       *string  `json:"equipment_type_id"`
	ProposedEquipmentName *string  `json:"proposed_equipment_name"`
	ProposedCapacityUnit  *string  `json:"proposed_capacity_unit"`
	ExpectedCapacity      *float64 `json:"expected_capacity"`
	Qty                   float64  `json:"qty"`
	UnitOfMeasure         string   `json:"unit_of_measure"`
	EstimatedUnitPrice    float64  `json:"estimated_unit_price"`
	Justification         string   `json:"justification"`
}
