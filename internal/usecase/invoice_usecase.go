package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// InvoiceLineInput is a single line item on a supplier invoice.
type InvoiceLineInput struct {
	ItemID      *string // May be nil if the line cannot be matched to a catalog item
	RawLineText string  // Original text from OCR or supplier document
	Qty         float64
	UnitPrice   float64
}

// InvoiceUseCase manages supplier invoices and 3-Way Matching.
//
// 3-Way Matching: PurchaseOrder + SupplierInvoice + GoodsReceipt must all reconcile.
// On a successful match:
//   - Invoice status → MATCHED.
//   - A Transaction ledger entry (EXPENSE) is created for the node.
//   - PO.SettlePayment is called to mark the PO COMPLETED and trigger Asset creation (if CapEx).
type InvoiceUseCase interface {
	// RecordInvoice registers a supplier invoice linked to a PurchaseOrder.
	RecordInvoice(ctx context.Context, orgID, purOID, supplierID, invoiceNumber string, totalAmount, taxAmount float64, imageURL string, lines []InvoiceLineInput) (*models.SupplierInvoice, error)

	// PerformThreeWayMatch validates PO + Invoice + GR, creates a ledger expense entry,
	// and triggers PO payment settlement (which auto-creates Asset for CapEx POs).
	PerformThreeWayMatch(ctx context.Context, invoiceID, grID, matchedByStaffID string) (*models.Asset, error)

	// PerformPrepaymentMatch validates PurO + Invoice (Two-Way Match before GR is available),
	// creates a ledger expense entry, and marks Invoice as MATCHED.
	PerformPrepaymentMatch(ctx context.Context, invoiceID, matchedByStaffID string) error

	// LinkGoodsReceiptToPrepaidInvoice links a confirmed GoodsReceipt to an already MATCHED/PAID
	// prepaid invoice, and settles the PurO (which auto-creates Asset for CapEx).
	LinkGoodsReceiptToPrepaidInvoice(ctx context.Context, invoiceID, grID, matchedByStaffID string) (*models.Asset, error)

	// MarkPaid records that the supplier has been paid.
	MarkPaid(ctx context.Context, invoiceID string) error

	GetInvoice(ctx context.Context, invoiceID string) (*models.SupplierInvoice, []*models.SupplierInvoiceLine, error)
}

// invoiceUseCase implements InvoiceUseCase.
type invoiceUseCase struct {
	invoiceRepo services.SupplierInvoiceRepository
	invLineRepo services.SupplierInvoiceLineRepository
	txRepo      services.TransactionRepository
	purORepo    services.PurchaseOrderRepository
	grRepo      services.GoodsReceiptRepository
	// puroUC is set by the SupplyChainFacade post-construction to avoid circular deps.
	puroUC PurOUseCase
}

func newInvoiceUseCase(
	invoiceRepo services.SupplierInvoiceRepository,
	invLineRepo services.SupplierInvoiceLineRepository,
	txRepo services.TransactionRepository,
	purORepo services.PurchaseOrderRepository,
	grRepo services.GoodsReceiptRepository,
) *invoiceUseCase {
	return &invoiceUseCase{
		invoiceRepo: invoiceRepo,
		invLineRepo: invLineRepo,
		txRepo:      txRepo,
		purORepo:    purORepo,
		grRepo:      grRepo,
	}
}

func (uc *invoiceUseCase) setPurOUseCase(p PurOUseCase) {
	uc.puroUC = p
}

// RecordInvoice registers a supplier invoice in PENDING status.
// The invoice is linked to a PurchaseOrder; 3-Way Matching is performed separately.
func (uc *invoiceUseCase) RecordInvoice(ctx context.Context, orgID, purOID, supplierID, invoiceNumber string, totalAmount, taxAmount float64, imageURL string, lines []InvoiceLineInput) (*models.SupplierInvoice, error) {
	if invoiceNumber == "" {
		return nil, fmt.Errorf("invoice: RecordInvoice: invoice_number is required")
	}
	if totalAmount <= 0 {
		return nil, fmt.Errorf("invoice: RecordInvoice: total_amount must be > 0")
	}

	now := time.Now()
	inv := &models.SupplierInvoice{
		ID:              uuid.NewString(),
		OrgID:           orgID,
		PurchaseOrderID: purOID,
		SupplierID:      supplierID,
		InvoiceNumber:   invoiceNumber,
		TotalAmount:     totalAmount,
		TaxAmount:       taxAmount,
		ImageURL:        imageURL,
		Status:          models.SupplierInvoicePending,
		InvoiceDate:     now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := uc.invoiceRepo.Create(ctx, inv); err != nil {
		return nil, fmt.Errorf("invoice: RecordInvoice: persist: %w", err)
	}

	for i, l := range lines {
		line := &models.SupplierInvoiceLine{
			ID:          uuid.NewString(),
			InvoiceID:   inv.ID,
			ItemID:      l.ItemID,
			RawLineText: l.RawLineText,
			Qty:         l.Qty,
			UnitPrice:   l.UnitPrice,
			LineTotal:   l.Qty * l.UnitPrice,
		}
		if err := uc.invLineRepo.AddLine(ctx, line); err != nil {
			return nil, fmt.Errorf("invoice: RecordInvoice: add line %d: %w", i, err)
		}
	}

	return inv, nil
}

// PerformThreeWayMatch validates the PO, Invoice, and GR against each other.
// On success:
//  1. Invoice status → MATCHED (with matched_by and matched_at set).
//  2. A Transaction (EXPENSE, ref_type=SUPPLIER_INVOICE) is written to the ledger.
//  3. POUseCase.SettlePayment is called to finalize the PO and trigger Asset creation (CapEx).
func (uc *invoiceUseCase) PerformThreeWayMatch(ctx context.Context, invoiceID, grID, matchedByStaffID string) (*models.Asset, error) {
	inv, err := uc.loadInvoice(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv.Status != models.SupplierInvoicePending {
		return nil, fmt.Errorf("invoice: 3WayMatch: invoice %s is not PENDING (current: %s)", invoiceID, inv.Status)
	}

	purO, err := uc.purORepo.FindByID(ctx, inv.PurchaseOrderID)
	if err != nil || purO == nil {
		return nil, fmt.Errorf("invoice: 3WayMatch: PO %s not found: %w", inv.PurchaseOrderID, err)
	}

	gr, err := uc.grRepo.FindByID(ctx, grID)
	if err != nil || gr == nil {
		return nil, fmt.Errorf("invoice: 3WayMatch: GR %s not found: %w", grID, err)
	}

	// Validate that GR is linked to the same PO.
	if gr.RefID != purO.ID {
		return nil, fmt.Errorf("invoice: 3WayMatch: GR %s is linked to %s, not to PO %s", grID, gr.RefID, purO.ID)
	}

	// Link GR to invoice and mark as MATCHED.
	now := time.Now()
	inv.GRID = &grID
	inv.Status = models.SupplierInvoiceMatched
	inv.MatchedBy = &matchedByStaffID
	inv.MatchedAt = &now
	inv.UpdatedAt = now

	if err := uc.invoiceRepo.Update(ctx, inv); err != nil {
		return nil, fmt.Errorf("invoice: 3WayMatch: update invoice: %w", err)
	}

	// Write ledger expense entry for the node receiving goods.
	tx := &models.Transaction{
		ID:          uuid.NewString(),
		NodeID:      purO.DeliveryToNodeID,
		OrgID:       purO.OrgID,
		Amount:      inv.TotalAmount,
		Type:        models.TxExpense,
		RefType:     models.TxRefSupplierInvoice,
		ReferenceID: inv.ID,
		Description: fmt.Sprintf("Supplier payment: Invoice %s (PO %s)", inv.InvoiceNumber, purO.ID),
		Timestamp:   now,
	}
	if err := uc.txRepo.Create(ctx, tx); err != nil {
		return nil, fmt.Errorf("invoice: 3WayMatch: create ledger entry: %w", err)
	}

	// Settle the PO — triggers Asset creation for CapEx POs.
	if uc.puroUC != nil {
		asset, err := uc.puroUC.SettlePayment(ctx, purO.ID, invoiceID, grID, matchedByStaffID)
		if err != nil {
			return nil, fmt.Errorf("invoice: 3WayMatch: settle PO: %w", err)
		}
		return asset, nil
	}

	return nil, nil
}

// PerformPrepaymentMatch validates the PurO and Invoice against each other.
// On success:
//  1. Invoice status → MATCHED (with matched_by and matched_at set).
//  2. A Transaction (EXPENSE, ref_type=SUPPLIER_INVOICE) is written to the ledger.
func (uc *invoiceUseCase) PerformPrepaymentMatch(ctx context.Context, invoiceID, matchedByStaffID string) error {
	inv, err := uc.loadInvoice(ctx, invoiceID)
	if err != nil {
		return err
	}
	if inv.Status != models.SupplierInvoicePending {
		return fmt.Errorf("invoice: PrepaymentMatch: invoice %s is not PENDING (current: %s)", invoiceID, inv.Status)
	}

	purO, err := uc.purORepo.FindByID(ctx, inv.PurchaseOrderID)
	if err != nil || purO == nil {
		return fmt.Errorf("invoice: PrepaymentMatch: PurO %s not found: %w", inv.PurchaseOrderID, err)
	}

	// Mark as MATCHED.
	now := time.Now()
	inv.Status = models.SupplierInvoiceMatched
	inv.MatchedBy = &matchedByStaffID
	inv.MatchedAt = &now
	inv.UpdatedAt = now

	if err := uc.invoiceRepo.Update(ctx, inv); err != nil {
		return fmt.Errorf("invoice: PrepaymentMatch: update invoice: %w", err)
	}

	// Write ledger expense entry for the node receiving goods.
	tx := &models.Transaction{
		ID:          uuid.NewString(),
		NodeID:      purO.DeliveryToNodeID,
		OrgID:       purO.OrgID,
		Amount:      inv.TotalAmount,
		Type:        models.TxExpense,
		RefType:     models.TxRefSupplierInvoice,
		ReferenceID: inv.ID,
		Description: fmt.Sprintf("Supplier Pre-payment: Invoice %s (PurO %s)", inv.InvoiceNumber, purO.ID),
		Timestamp:   now,
	}
	if err := uc.txRepo.Create(ctx, tx); err != nil {
		return fmt.Errorf("invoice: PrepaymentMatch: create ledger entry: %w", err)
	}

	return nil
}

// LinkGoodsReceiptToPrepaidInvoice links a confirmed GR to an already prepaid invoice,
// and completes the PurO (triggering Asset creation for CapEx).
func (uc *invoiceUseCase) LinkGoodsReceiptToPrepaidInvoice(ctx context.Context, invoiceID, grID, matchedByStaffID string) (*models.Asset, error) {
	inv, err := uc.loadInvoice(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv.Status != models.SupplierInvoiceMatched && inv.Status != models.SupplierInvoicePaid {
		return nil, fmt.Errorf("invoice: LinkGR: invoice %s must be MATCHED or PAID (current: %s)", invoiceID, inv.Status)
	}

	purO, err := uc.purORepo.FindByID(ctx, inv.PurchaseOrderID)
	if err != nil || purO == nil {
		return nil, fmt.Errorf("invoice: LinkGR: PurO %s not found: %w", inv.PurchaseOrderID, err)
	}

	gr, err := uc.grRepo.FindByID(ctx, grID)
	if err != nil || gr == nil {
		return nil, fmt.Errorf("invoice: LinkGR: GR %s not found: %w", grID, err)
	}

	// Validate that GR is linked to the same PurO.
	if gr.RefID != purO.ID {
		return nil, fmt.Errorf("invoice: LinkGR: GR %s is linked to %s, not to PurO %s", grID, gr.RefID, purO.ID)
	}

	// Link GR to invoice.
	inv.GRID = &grID
	inv.UpdatedAt = time.Now()
	if err := uc.invoiceRepo.Update(ctx, inv); err != nil {
		return nil, fmt.Errorf("invoice: LinkGR: update invoice: %w", err)
	}

	// Settle the PurO — triggers Asset creation for CapEx PurOs.
	if uc.puroUC != nil {
		asset, err := uc.puroUC.SettlePayment(ctx, purO.ID, invoiceID, grID, matchedByStaffID)
		if err != nil {
			return nil, fmt.Errorf("invoice: LinkGR: settle PurO: %w", err)
		}
		return asset, nil
	}

	return nil, nil
}

// MarkPaid records that payment has been sent to the supplier.
func (uc *invoiceUseCase) MarkPaid(ctx context.Context, invoiceID string) error {
	inv, err := uc.loadInvoice(ctx, invoiceID)
	if err != nil {
		return err
	}
	if inv.Status != models.SupplierInvoiceMatched {
		return fmt.Errorf("invoice: MarkPaid: invoice %s must be MATCHED before marking paid (current: %s)", invoiceID, inv.Status)
	}

	now := time.Now()
	inv.Status = models.SupplierInvoicePaid
	inv.PaidAt = &now
	inv.UpdatedAt = now
	return uc.invoiceRepo.Update(ctx, inv)
}

func (uc *invoiceUseCase) GetInvoice(ctx context.Context, invoiceID string) (*models.SupplierInvoice, []*models.SupplierInvoiceLine, error) {
	inv, err := uc.loadInvoice(ctx, invoiceID)
	if err != nil {
		return nil, nil, err
	}
	lines, err := uc.invLineRepo.ListByInvoice(ctx, invoiceID)
	return inv, lines, err
}

func (uc *invoiceUseCase) loadInvoice(ctx context.Context, id string) (*models.SupplierInvoice, error) {
	inv, err := uc.invoiceRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("invoice: load invoice %s: %w", id, err)
	}
	if inv == nil {
		return nil, fmt.Errorf("invoice: invoice %s not found", id)
	}
	return inv, nil
}
