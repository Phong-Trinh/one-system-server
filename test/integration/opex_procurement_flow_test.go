package integration

import (
	"context"
	"testing"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/usecase"
)

func TestOpExProcurementBusinessFlow(t *testing.T) {
	ctx := context.Background()

	// Initialize mock or real MongoDB repositories and the facade
	tc := setupFacade()
	facade := tc.Facade

	t.Log("--- OPEX PROCUREMENT INTEGRATION TEST STARTED ---")
	orgID := "org_onesystem"
	hqNodeID := "node_hq"
	storeNodeID := "node_store_1"

	// Setup Nodes
	storeNode := &models.Node{
		ID:    storeNodeID,
		Name:  "Store 1",
		OrgID: orgID,
		Type:  models.NodeStore,
	}
	_ = tc.NodeRepo.Create(ctx, storeNode)

	// Setup Supplier
	supplierID := "supplier_123"
	supplier := &models.Supplier{
		ID:    supplierID,
		OrgID: orgID,
		Name:  "Acme Ingredients",
	}
	_ = tc.SupplierRepo.Create(ctx, supplier)

	t.Log("Step 1: Configure Store for External Procurement of Milk with ROP=50, SafetyStock=20.")
	err := tc.NodeItemConfigRepo.Upsert(ctx, &models.NodeItemConfig{
		ItemID:           "item_milk",
		NodeID:           storeNode.ID,
		SourcingStrategy: models.SourcingExternalProcurement,
		ReorderPoint:     50.0,
		SafetyStock:      20.0,
	})
	if err != nil {
		t.Fatalf("Failed to create NodeItemConfig: %v", err)
	}

	err = facade.Inventory.InitStock(ctx, storeNode.ID, "item_milk", 60.0)
	if err != nil {
		t.Fatalf("InitStock failed: %v", err)
	}

	t.Log("Step 2: Store consumes 20 units of milk, dropping stock to 40 (below ROP).")
	err = facade.StockOutWithROP(ctx, orgID, hqNodeID, storeNode.ID, "item_milk", 20.0)
	if err != nil {
		t.Fatalf("StockOutWithROP failed: %v", err)
	}

	t.Log("Step 3: Verify Auto Draft PO was created.")
	draftPOs, err := tc.PurORepo.FindByStatus(ctx, orgID, models.PurchaseOrderDraft)
	if err != nil {
		t.Fatalf("FindByStatus failed: %v", err)
	}
	if len(draftPOs) != 1 {
		t.Fatalf("Expected 1 Draft PO, got %d", len(draftPOs))
	}
	po := draftPOs[0]
	if po.TriggerType != models.PurOTriggerAutoDraft {
		t.Errorf("Expected PO Trigger Type AUTO_DRAFT, got %s", po.TriggerType)
	}

	t.Log("Step 4: HQ confirms the Draft PO.")
	itemID := "item_milk"
	lines := []usecase.PurOLineInput{
		{
			ItemID:     &itemID,
			QtyOrdered: 30.0, // (50 - 40) gap + 20 safety stock
			PkgUnit:    "liter",
			Conversion: 1.0,
			UnitPrice:  2.5,
		},
	}
	err = facade.PurO.ConfirmDraftPurO(ctx, po.ID, supplierID, "staff_hq", lines)
	if err != nil {
		t.Fatalf("ConfirmDraftPurO failed: %v", err)
	}

	poUpdated, _, err := facade.PurO.GetPurO(ctx, po.ID)
	if poUpdated.Status != models.PurchaseOrderConfirmed {
		t.Errorf("Expected PO status CONFIRMED, got %s", poUpdated.Status)
	}

	t.Log("Step 5: Supplier ships the goods.")
	err = facade.PurO.MarkShipped(ctx, po.ID)
	if err != nil {
		t.Fatalf("MarkShipped failed: %v", err)
	}

	t.Log("Step 6: Store receives goods via GoodsReceipt.")
	grLines := []usecase.GRLineInput{
		{ItemID: "item_milk", QtyExpected: 30.0, QtyReceived: 30.0},
	}
	gr, err := facade.GR.ConfirmPurOGoodsReceipt(ctx, po.ID, storeNodeID, "staff_store", grLines)
	if err != nil {
		t.Fatalf("ConfirmPurOGoodsReceipt failed: %v", err)
	}
	if gr.Status != models.GoodsReceiptConfirmed {
		t.Errorf("Expected GR status CONFIRMED, got %s", gr.Status)
	}

	// Verify stock increased
	stock, _ := tc.NodeStockRepo.Get(ctx, storeNodeID, "item_milk")
	if stock.QtyOnHand != 70.0 { // 40 + 30
		t.Errorf("Expected store stock 70, got %v", stock.QtyOnHand)
	}

	t.Log("Step 6: HQ receives and reconciles Supplier Invoice (3-way match).")
	invoiceLines := []usecase.InvoiceLineInput{
		{ItemID: &itemID, Qty: 30.0, UnitPrice: 2.5},
	}
	inv, err := facade.Invoice.RecordInvoice(ctx, orgID, po.ID, supplierID, "INV-1001", 75.0, 5.0, "http://example.com/inv.jpg", invoiceLines)
	if err != nil {
		t.Fatalf("RecordInvoice failed: %v", err)
	}

	_, err = facade.Invoice.PerformThreeWayMatch(ctx, inv.ID, gr.ID, "staff_hq")
	if err != nil {
		t.Fatalf("PerformThreeWayMatch failed: %v", err)
	}

	invUpdated, err := tc.InvoiceRepo.FindByID(ctx, inv.ID)
	if invUpdated.Status != models.SupplierInvoiceMatched {
		t.Errorf("Expected Invoice status MATCHED, got %s", invUpdated.Status)
	}
	
	// Check PO status
	poFinal, _, _ := facade.PurO.GetPurO(ctx, po.ID)
	if poFinal.Status != models.PurchaseOrderCompleted {
		t.Errorf("Expected PO status COMPLETED, got %s", poFinal.Status)
	}

	t.Log("OpEx Procurement flow successful.")
	t.Log("--- OPEX PROCUREMENT INTEGRATION TEST COMPLETED SUCCESSFULLY ---")
}
