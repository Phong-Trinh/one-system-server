package services

import (
    "context"
    "one-system-server/internal/domain/models"
)

type ProcurementService interface {
    // RecordExternalPurchase creates a new purchase record in PENDING state.
    RecordExternalPurchase(ctx context.Context, purchase models.ExternalPurchase) error
    
    // ApproveExternalPurchase marks a purchase as approved and:
    // 1. Creates an Expense Transaction.
    // 2. Increases local inventory stock.
    ApproveExternalPurchase(ctx context.Context, purchaseID string) error
    
    // RejectExternalPurchase marks a purchase as rejected.
    RejectExternalPurchase(ctx context.Context, purchaseID string, reason string) error
}
