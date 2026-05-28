package integration

import (
	"context"
	"testing"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/usecase"
)

func TestITOBusinessFlow(t *testing.T) {
	ctx := context.Background()

	// Initialize mock or real MongoDB repositories and the facade
	tc := setupFacade()
	facade := tc.Facade

	t.Log("--- ITO BUSINESS INTEGRATION TEST STARTED ---")
	orgID := "org_onesystem"

	// =========================================================================
	// Setup Nodes
	// =========================================================================
	siteA := "site_a"
	siteB := "site_b"

	storeNode := &models.Node{
		ID:     "node_store_1",
		Name:   "Store 1",
		OrgID:  orgID,
		Type:   models.NodeStore,
		SiteID: &siteA, // Different site
	}
	factoryNode := &models.Node{
		ID:     "node_factory_1",
		Name:   "Factory 1",
		OrgID:  orgID,
		Type:   models.NodeFactory,
		SiteID: &siteB, // Different site
	}
	kitchenNode := &models.Node{
		ID:     "node_kitchen_1",
		Name:   "Kitchen 1",
		OrgID:  orgID,
		Type:   models.NodeStore, // e.g. a kitchen in the same building as storeNode
		SiteID: &siteA,           // SAME site as storeNode
	}

	_ = tc.NodeRepo.Create(ctx, storeNode)
	_ = tc.NodeRepo.Create(ctx, factoryNode)
	_ = tc.NodeRepo.Create(ctx, kitchenNode)

	// Seed inventory for provider nodes
	err := facade.Inventory.InitStock(ctx, factoryNode.ID, "item_flour", 1000.0)
	if err != nil {
		t.Fatalf("InitStock failed: %v", err)
	}
	err = facade.Inventory.InitStock(ctx, storeNode.ID, "item_salt", 50.0)
	if err != nil {
		t.Fatalf("InitStock failed: %v", err)
	}

	// =========================================================================
	// Scenario 1: Cross-Site Transfer with Transit Damage (Exception Path)
	// =========================================================================
	t.Log("Step 1.1: Store A requests 100 units of flour from Factory B (Cross-site).")
	lines := []usecase.ITOLineInput{
		{
			ItemID:     "item_flour",
			QtyOrdered: 100.0,
			PkgUnit:    "kg",
			Conversion: 1.0,
		},
	}
	ito1, err := facade.ITO.CreateManualITO(ctx, orgID, storeNode.ID, factoryNode.ID, "staff_store_mgr", lines)
	if err != nil {
		t.Fatalf("Failed to create manual ITO: %v", err)
	}
	if ito1.Status != models.ITOPendingApproval {
		t.Errorf("Expected ITO status PENDING_APPROVAL, got %s", ito1.Status)
	}

	t.Log("Step 1.2: Factory Manager approves the ITO.")
	err = facade.ITO.ApproveManualITO(ctx, ito1.ID)
	if err != nil {
		t.Fatalf("ApproveManualITO failed: %v", err)
	}
	approvedITO1, _ := tc.ITORepo.FindByID(ctx, ito1.ID)
	if approvedITO1.Status != models.ITOAutoApproved {
		t.Errorf("Expected ITO status AUTO_APPROVED, got %s", approvedITO1.Status)
	}

	t.Log("Step 1.3: Factory dispatches 100 units. ITO goes to IN_TRANSIT.")
	giInput := usecase.GoodsIssueInput{
		DriverName:   "John Trucker",
		VehiclePlate: "ABC-123",
		MediaURL:     "http://example.com/photo.jpg",
		Lines: []usecase.GILineInput{
			{ItemID: "item_flour", QtyIssued: 100.0},
		},
	}
	gi1, err := facade.ITO.ConfirmGoodsIssue(ctx, ito1.ID, giInput)
	if err != nil {
		t.Fatalf("ConfirmGoodsIssue failed: %v", err)
	}
	if gi1.Status != models.GoodsIssueConfirmed {
		t.Errorf("Expected GI status CONFIRMED, got %s", gi1.Status)
	}

	// Verify ITO is in transit
	updatedITO1, _ := tc.ITORepo.FindByID(ctx, ito1.ID)
	if updatedITO1.Status != models.ITOInTransit {
		t.Errorf("Expected ITO status IN_TRANSIT, got %s", updatedITO1.Status)
	}

	// Verify Factory stock reduced by 100 (1000 -> 900)
	stockF, _ := tc.NodeStockRepo.Get(ctx, factoryNode.ID, "item_flour")
	if stockF.QtyOnHand != 900.0 {
		t.Errorf("Expected Factory stock 900, got %v", stockF.QtyOnHand)
	}

	t.Log("Step 1.4: Store receives only 90 units due to transit damage.")
	grInput := usecase.GoodsReceiptInput{
		ReceivedByStaffID: "staff_store_recv",
		Lines: []usecase.GRLineInput{
			{ItemID: "item_flour", QtyExpected: 100.0, QtyReceived: 90.0}, // 10 missing
		},
	}
	gr1, err := facade.ITO.ConfirmGoodsReceipt(ctx, ito1.ID, gi1.ID, grInput)
	if err != nil {
		t.Fatalf("ConfirmGoodsReceipt failed: %v", err)
	}
	if gr1.Status != models.GoodsReceiptDiscrepancy {
		t.Errorf("Expected GR status DISCREPANCY, got %s", gr1.Status)
	}

	t.Log("Step 1.5: Verify DiscrepancyTicket is created for the missing 10 units.")
	tickets, err := tc.DTRepo.FindByGR(ctx, gr1.ID)
	if err != nil || len(tickets) == 0 {
		t.Fatalf("Expected discrepancy ticket to be created")
	}
	if tickets[0].QtyMissing != 10.0 {
		t.Errorf("Expected 10 units missing in ticket, got %v", tickets[0].QtyMissing)
	}

	// Verify Store stock increased by only 90
	stockS, _ := tc.NodeStockRepo.Get(ctx, storeNode.ID, "item_flour")
	if stockS.QtyOnHand != 90.0 {
		t.Errorf("Expected Store stock 90, got %v", stockS.QtyOnHand)
	}

	t.Log("Cross-site scenario successful.")

	// =========================================================================
	// Scenario 2: Same-Site 1-Click Transfer (Happy Path)
	// =========================================================================
	t.Log("Step 2.1: Kitchen requests 10 units of salt from Store (Same-site).")
	lines2 := []usecase.ITOLineInput{
		{
			ItemID:     "item_salt",
			QtyOrdered: 10.0,
			PkgUnit:    "kg",
			Conversion: 1.0,
		},
	}
	ito2, err := facade.ITO.CreateManualITO(ctx, orgID, kitchenNode.ID, storeNode.ID, "staff_chef", lines2)
	if err != nil {
		t.Fatalf("Failed to create manual ITO: %v", err)
	}
	if ito2.Status != models.ITOPendingApproval {
		t.Errorf("Expected ITO status PENDING_APPROVAL, got %s", ito2.Status)
	}

	err = facade.ITO.ApproveManualITO(ctx, ito2.ID)
	if err != nil {
		t.Fatalf("ApproveManualITO failed: %v", err)
	}
	approvedITO2, _ := tc.ITORepo.FindByID(ctx, ito2.ID)
	if approvedITO2.Status != models.ITOAutoApproved {
		t.Errorf("Expected ITO status AUTO_APPROVED, got %s", approvedITO2.Status)
	}

	t.Log("Step 2.2: Store dispatches 10 units. System auto-generates GR and completes ITO.")
	giInput2 := usecase.GoodsIssueInput{
		// Same site doesn't require driver info
		Lines: []usecase.GILineInput{
			{ItemID: "item_salt", QtyIssued: 10.0},
		},
	}
	gi2, err := facade.ITO.ConfirmGoodsIssue(ctx, ito2.ID, giInput2)
	if err != nil {
		t.Fatalf("ConfirmGoodsIssue (same-site) failed: %v", err)
	}
	if gi2.Status != models.GoodsIssueConfirmed {
		t.Errorf("Expected GI status CONFIRMED, got %s", gi2.Status)
	}

	t.Log("Step 2.3: Verify ITO immediately COMPLETED and stock updated instantly.")
	updatedITO2, _ := tc.ITORepo.FindByID(ctx, ito2.ID)
	if updatedITO2.Status != models.ITOCompleted {
		t.Errorf("Expected ITO status COMPLETED, got %s", updatedITO2.Status)
	}

	// Store salt stock should be 50 - 10 = 40
	stockS_salt, _ := tc.NodeStockRepo.Get(ctx, storeNode.ID, "item_salt")
	if stockS_salt.QtyOnHand != 40.0 {
		t.Errorf("Expected Store salt stock 40, got %v", stockS_salt.QtyOnHand)
	}

	// Kitchen salt stock should be 10
	stockK_salt, _ := tc.NodeStockRepo.Get(ctx, kitchenNode.ID, "item_salt")
	if stockK_salt.QtyOnHand != 10.0 {
		t.Errorf("Expected Kitchen salt stock 10, got %v", stockK_salt.QtyOnHand)
	}

	// =========================================================================
	// Scenario 3: Manual ITO Rejection
	// =========================================================================
	t.Log("Step 3.1: Store requests 50 units of flour, but Factory rejects it.")
	ito3, err := facade.ITO.CreateManualITO(ctx, orgID, storeNode.ID, factoryNode.ID, "staff_store_mgr", []usecase.ITOLineInput{
		{ItemID: "item_flour", QtyOrdered: 50.0, PkgUnit: "kg", Conversion: 1.0},
	})
	if err != nil {
		t.Fatalf("Failed to create manual ITO: %v", err)
	}

	t.Log("Step 3.2: Factory Manager rejects the ITO.")
	err = facade.ITO.RejectManualITO(ctx, ito3.ID, "Not enough stock for this large request")
	if err != nil {
		t.Fatalf("RejectManualITO failed: %v", err)
	}

	ito3DB, _ := tc.ITORepo.FindByID(ctx, ito3.ID)
	if ito3DB.Status != models.ITOCancelled {
		t.Errorf("Expected ITO status CANCELLED, got %s", ito3DB.Status)
	}
	t.Log("Manual rejection scenario successful.")

	// =========================================================================
	// Scenario 4: Auto-ITO triggered by ROP Engine
	// =========================================================================
	t.Log("Step 4.1: Configure Store for Internal Transfer of Sugar with ROP=20, SafetyStock=10.")
	err = tc.NodeItemConfigRepo.Upsert(ctx, &models.NodeItemConfig{
		ItemID:           "item_sugar",
		NodeID:           storeNode.ID,
		SourcingStrategy: models.SourcingInternalTransfer,
		ProviderNodeID:   &factoryNode.ID,
		ReorderPoint:     20.0,
		SafetyStock:      10.0,
	})
	if err != nil {
		t.Fatalf("Failed to create NodeItemConfig: %v", err)
	}

	// Init Store sugar stock to 25
	err = facade.Inventory.InitStock(ctx, storeNode.ID, "item_sugar", 25.0)
	if err != nil {
		t.Fatalf("InitStock failed: %v", err)
	}

	t.Log("Step 4.2: Store consumes 10 units of sugar, dropping stock to 15 (below ROP).")
	hqNodeID := "node_hq"
	err = facade.StockOutWithROP(ctx, orgID, hqNodeID, storeNode.ID, "item_sugar", 10.0)
	if err != nil {
		t.Fatalf("StockOutWithROP failed: %v", err)
	}

	t.Log("Step 4.3: Verify Auto ITO was created and is AUTO_APPROVED.")
	itos, err := facade.ITO.ListITOsByNode(ctx, storeNode.ID)
	if err != nil {
		t.Fatalf("ListITOsByNode failed: %v", err)
	}

	var autoITO *models.InternalTransferOrder
	for _, ito := range itos {
		if ito.Trigger == models.ITOTriggerROP && ito.ProviderNodeID == factoryNode.ID {
			autoITO = ito
			break
		}
	}

	if autoITO == nil {
		t.Fatalf("Expected an Auto ITO to be created, but none found.")
	}
	if autoITO.Status != models.ITOAutoApproved {
		t.Errorf("Expected Auto ITO status AUTO_APPROVED, got %s", autoITO.Status)
	}

	// Verify the quantity ordered was calculated correctly: (20 - 15) gap + 10 safety stock = 15
	_, lines3, err := facade.ITO.GetITO(ctx, autoITO.ID)
	if err != nil || len(lines3) != 1 {
		t.Fatalf("Expected 1 line in Auto ITO, got %v (err: %v)", len(lines3), err)
	}
	if lines3[0].QtyOrderedBU != 15.0 {
		t.Errorf("Expected Auto ITO to order 15 units (gap 5 + safety 10), got %v", lines3[0].QtyOrderedBU)
	}
	t.Log("Auto-ITO (ROP) scenario successful.")

	// =========================================================================
	// Scenario 5: Multiple ROP Breaches Generate Multiple ITOs
	// =========================================================================
	t.Log("Step 5.1: Store consumes another 2 units of sugar (stock goes 15 -> 13, below ROP again).")
	err = facade.StockOutWithROP(ctx, orgID, hqNodeID, storeNode.ID, "item_sugar", 2.0)
	if err != nil {
		t.Fatalf("StockOutWithROP failed: %v", err)
	}

	t.Log("Step 5.2: Verify NO SECOND Auto ITO was created because an active one already exists.")
	itos2, err := facade.ITO.ListITOsByNode(ctx, storeNode.ID)
	if err != nil {
		t.Fatalf("ListITOsByNode failed: %v", err)
	}

	autoITOCount := 0
	for _, ito := range itos2 {
		if ito.Trigger == models.ITOTriggerROP && ito.ProviderNodeID == factoryNode.ID {
			autoITOCount++
		}
	}

	if autoITOCount != 1 {
		t.Errorf("Expected exactly 1 Auto ITO (duplicate skipped), got %d", autoITOCount)
	}
	t.Log("Multiple ROP breaches scenario successful (System correctly skips duplicate ITO).")

	t.Log("--- ITO BUSINESS INTEGRATION TEST COMPLETED SUCCESSFULLY ---")
}
