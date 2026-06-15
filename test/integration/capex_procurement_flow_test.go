package integration

import (
	"context"
	"testing"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
	"one-system-server/internal/usecase"
)

// ─── INTEGRATION TEST CASE ───────────────────────────────────────────────────

func TestCapExProcurementBusinessFlow(t *testing.T) {
	ctx := context.Background()

	// Initialize mock or real MongoDB repositories and the facade
	tc := setupFacade()
	facade := tc.Facade

	t.Log("--- BUSINESS INTEGRATION TEST STARTED ---")

	// =========================================================================
	// Scenario 1: Store requests a new kind of machine (not yet in system)
	// =========================================================================
	t.Log("Step 1.1: Store Manager requests a new category of equipment (Industrial Pizza Oven) by submitting a PR.")
	t.Log("UX Optimization: The Store Manager does not type technical IDs. They type the name 'Industrial Pizza Oven', and the UI auto-generates 'eq_pizza_oven' for the API call.")
	orgID := "org_onesystem"
	storeNodeID := "node_downtown_store"
	hqNodeID := "node_hq"
	staffID := "staff_store_manager"

	pizzaOvenTypeID := "eq_pizza_oven"
	proposedName := "Industrial Pizza Oven"
	proposedCapUnit := "tray"
	expectedCap := 2.0
	prLineInput := services.PRLineInput{
		EquipmentTypeID:       &pizzaOvenTypeID,
		ProposedEquipmentName: &proposedName,
		ProposedCapacityUnit:  &proposedCapUnit,
		ExpectedCapacity:      &expectedCap,
		Qty:                   1,
		UnitOfMeasure:         "unit",
		EstimatedUnitPrice:    3000.0,
		Justification:         "Requesting brand new equipment type: Industrial Pizza Oven",
	}

	// PR is submitted directly with the proposed new equipment type ID and proposed specifications
	pr, err := facade.PR.SubmitPR(ctx, services.SubmitPRRequest{
		OrgID:           orgID,
		RequesterNodeID: storeNodeID,
		StaffID:         staffID,
		Justification:   "Adding premium pizza menu to store",
		Lines:           []services.PRLineInput{prLineInput},
	})
	if err != nil {
		t.Fatalf("Failed to submit PR: %v", err)
	}
	if pr.Status != models.PRPendingHQApproval {
		t.Errorf("Expected PR status to be PENDING_HQ_APPROVAL, got %s", pr.Status)
	}
	t.Logf("PR created successfully with ID: %s", pr.ID)

	// Verify that the EquipmentType was automatically created in DRAFT status upon PR submission
	draftEqType, err := tc.EqTypeRepo.FindByID(ctx, pizzaOvenTypeID)
	if err != nil || draftEqType == nil {
		t.Fatalf("Expected EquipmentType %s to be registered as DRAFT, got nil or error: %v", pizzaOvenTypeID, err)
	}
	if draftEqType.Status != models.EquipmentTypeDraft {
		t.Errorf("Expected draft EquipmentType status to be DRAFT, got %s", draftEqType.Status)
	}
	t.Log("EquipmentType was automatically created as DRAFT in catalog upon PR submission")

	t.Log("Step 1.2: HQ reviews the PR and approves it. The system automatically activates the EquipmentType 'eq_pizza_oven' in the catalog.")
	note := "Approved for Pizza Expansion project"
	// Get the PR lines to approve
	prLines, err := tc.PRLineRepo.ListByPR(ctx, pr.ID)
	if err != nil || len(prLines) == 0 {
		t.Fatalf("Failed to list PR lines: %v", err)
	}
	corrections := []services.PRLineCorrection{
		{
			LineID:          prLines[0].ID,
			EquipmentTypeID: "eq_pizza_oven",
			ExpectedCapacity: &expectedCap,
			Qty:             1,
			UnitOfMeasure:   "unit",
			EstimatedPrice:  3000.0,
		},
	}
	err = facade.PR.ApprovePR(ctx, pr.ID, "staff_hq_admin", &note, corrections)
	if err != nil {
		t.Fatalf("Failed to approve PR: %v", err)
	}

	// Verify that the draft EquipmentType has been activated to ACTIVE
	registeredEqType, err := tc.EqTypeRepo.FindByID(ctx, pizzaOvenTypeID)
	if err != nil || registeredEqType == nil {
		t.Fatalf("Expected EquipmentType %s to exist, got nil or error: %v", pizzaOvenTypeID, err)
	}
	if registeredEqType.Status != models.EquipmentTypeActive {
		t.Errorf("Expected EquipmentType status to be ACTIVE, got %s", registeredEqType.Status)
	}
	if registeredEqType.Name != "Industrial Pizza Oven" || registeredEqType.CapacityUnit != "tray" {
		t.Errorf("Registered EquipmentType fields do not match, got: %+v", registeredEqType)
	}
	t.Log("EquipmentType was automatically activated to ACTIVE in catalog during PR approval!")

	// Verify PR status is approved
	updatedPR, _, err := facade.PR.GetPR(ctx, pr.ID)
	if err != nil || updatedPR.Status != models.PRApproved {
		t.Fatalf("Expected PR to be APPROVED, got error: %v, status: %s", err, updatedPR.Status)
	}
	t.Log("PR approved successfully by HQ")

	// =========================================================================
	// Scenario 2: Pivot to different Supplier due to bad behavior (damaged delivery)
	// =========================================================================
	t.Log("Step 2.1: HQ reviews approved PR and prepares to order from Supplier A")
	// Setup Supplier A in database
	suppA := &models.Supplier{
		ID:          "supp_a",
		OrgID:       orgID,
		Name:        "Supplier A (Lousy Logistics)",
		ContactInfo: "support@suppliera.com",
	}
	_ = tc.SupplierRepo.Create(ctx, suppA)

	t.Log("Step 2.2: Convert approved PR to PurO with Supplier A")
	puroLines := []usecase.PurOLineInput{
		{
			EquipmentTypeID: &pizzaOvenTypeID,
			QtyOrdered:      1,
			PkgUnit:         "unit",
			Conversion:      1.0,
			UnitPrice:       2900.0, // Negotiated price is $2900
		},
	}
	purO, err := facade.PurO.CreatePRTriggeredPurO(ctx, pr.ID, suppA.ID, hqNodeID, "staff_hq_admin", puroLines)
	if err != nil {
		t.Fatalf("Failed to create PR triggered PO: %v", err)
	}
	if purO.Status != models.PurchaseOrderConfirmed {
		t.Errorf("Expected PurO status to be CONFIRMED, got %s", purO.Status)
	}
	t.Logf("PurO #1 issued to Supplier A with ID: %s", purO.ID)

	t.Log("Step 2.3: Supplier A ships the machine, but it arrives completely damaged (QtyReceived = 0)")
	err = facade.PurO.MarkOnWayDelivery(ctx, purO.ID)
	if err != nil {
		t.Fatalf("Failed to mark shipped: %v", err)
	}

	grInputLines := []usecase.GRLineInput{
		{
			ItemID:      "", // CapEx line has no ItemID
			QtyExpected: 1.0,
			QtyReceived: 0.0, // Damaged, rejected at receiving bay!
		},
	}
	gr, err := facade.GR.ConfirmPurOGoodsReceipt(ctx, purO.ID, storeNodeID, staffID, grInputLines)
	if err != nil {
		t.Fatalf("Failed to record GR: %v", err)
	}
	if gr.Status != models.GoodsReceiptDiscrepancy {
		t.Errorf("Expected GR status to be DISCREPANCY, got %s", gr.Status)
	}

	t.Log("Step 2.4: Assert a DiscrepancyTicket is automatically created, blocking asset creation")
	tickets, err := tc.DTRepo.FindByGR(ctx, gr.ID)
	if err != nil || len(tickets) == 0 {
		t.Fatalf("Expected discrepancy ticket to be created, got error: %v, count: %d", err, len(tickets))
	}
	if tickets[0].Status != models.DiscrepancyOpen {
		t.Errorf("Expected ticket status to be OPEN, got %s", tickets[0].Status)
	}

	t.Log("Step 2.5: HQ decides to cancel Supplier A's order due to their bad behavior, and pivots to Supplier B")
	// Set PurO to Cancelled (HQ action due to breach of service quality)
	purO.Status = models.PurchaseOrderCancelled
	_ = tc.PurORepo.Update(ctx, purO)
	t.Logf("PurO #1 (Supplier A) cancelled. Status: %s", purO.Status)

	// HQ reverts PR status back to APPROVED so it can be re-converted
	pr.Status = models.PRApproved
	_ = tc.PRRepo.Update(ctx, pr)
	t.Log("PR status reset back to APPROVED to allow pivot to Supplier B")

	// Setup Supplier B (Premium Logistics)
	suppB := &models.Supplier{
		ID:          "supp_b",
		OrgID:       orgID,
		Name:        "Supplier B (Premium Logistics)",
		ContactInfo: "sales@supplierb.com",
	}
	_ = tc.SupplierRepo.Create(ctx, suppB)

	t.Log("Step 2.6: HQ issues a new PurO directly to Supplier B for the same oven")
	purO2, err := facade.PurO.CreatePRTriggeredPurO(ctx, pr.ID, suppB.ID, hqNodeID, "staff_hq_admin", puroLines)
	if err != nil {
		t.Fatalf("Failed to create new PO for Supplier B: %v", err)
	}
	t.Logf("PurO #2 issued to Supplier B with ID: %s", purO2.ID)

	t.Log("Step 2.7: Supplier B delivers successfully. Store Manager receives it intact")
	err = facade.PurO.MarkOnWayDelivery(ctx, purO2.ID)
	if err != nil {
		t.Fatalf("Failed to mark shipped: %v", err)
	}

	grInputLines2 := []usecase.GRLineInput{
		{
			ItemID:      "",
			QtyExpected: 1.0,
			QtyReceived: 1.0, // Perfect delivery!
		},
	}
	gr2, err := facade.GR.ConfirmPurOGoodsReceipt(ctx, purO2.ID, storeNodeID, staffID, grInputLines2)
	if err != nil {
		t.Fatalf("Failed to record GR for Supplier B: %v", err)
	}
	if gr2.Status != models.GoodsReceiptConfirmed {
		t.Errorf("Expected GR status to be CONFIRMED, got %s", gr2.Status)
	}

	// =========================================================================
	// Scenario 3: Three-Way Financial Matching and Ledger Recording
	// =========================================================================
	t.Log("Step 3.1: Supplier B issues Invoice for $2900. HQ performs Three-Way Match")
	invLines := []usecase.InvoiceLineInput{
		{
			ItemID:    nil,
			Qty:       1.0,
			UnitPrice: 2900.0,
		},
	}
	invoice, err := facade.Invoice.RecordInvoice(ctx, orgID, purO2.ID, suppB.ID, "INV-SUPPB-99", 2900.0, 0.0, "https://doc.supplierb.com/inv99.pdf", invLines)
	if err != nil {
		t.Fatalf("Failed to record supplier invoice: %v", err)
	}

	// Match: PurO2 ($2900) + Supplier Invoice ($2900) + Goods Receipt gr2 (1 unit)
	asset, err := facade.Invoice.PerformThreeWayMatch(ctx, invoice.ID, gr2.ID, "staff_hq_admin")
	if err != nil {
		t.Fatalf("Three-Way Matching failed: %v", err)
	}
	if asset == nil {
		t.Fatal("Expected Asset record to be returned on payment settlement, got nil")
	}

	t.Log("Step 3.2: Verify Invoice is MATCHED and Purchase Order is COMPLETED")
	matchedInv, _, _ := facade.Invoice.GetInvoice(ctx, invoice.ID)
	if matchedInv.Status != models.SupplierInvoiceMatched {
		t.Errorf("Expected invoice status to be MATCHED, got %s", matchedInv.Status)
	}
	completedPO, err := tc.PurORepo.FindByID(ctx, purO2.ID)
	if err != nil || completedPO.Status != models.PurchaseOrderCompleted {
		t.Errorf("Expected PurO status to be COMPLETED, got status: %s", completedPO.Status)
	}

	t.Log("Step 3.3: Verify Expense transaction is registered in the General Ledger")
	txs, err := tc.TxRepo.ListByNode(ctx, storeNodeID, nil)
	if err != nil || len(txs) != 1 {
		t.Fatalf("Expected exactly 1 expense ledger entry at node, got error: %v, count: %d", err, len(txs))
	}
	if txs[0].Amount != 2900.0 || txs[0].Type != models.TxExpense {
		t.Errorf("Expected transaction to be an EXPENSE of $2900, got Type: %s, Amount: %f", txs[0].Type, txs[0].Amount)
	}
	t.Logf("Ledger Expense written successfully: ID: %s, Description: %s", txs[0].ID, txs[0].Description)

	// =========================================================================
	// Scenario 4: Physical Onboarding (Machine Registration)
	// =========================================================================
	t.Log("Step 4.1: Store Manager registers the oven in the kitchen as Machine ID: M_PIZZA_OVEN_01")
	if asset.Status != models.AssetPendingRegistration {
		t.Errorf("Expected asset status to be PENDING_REGISTRATION, got %s", asset.Status)
	}

	machine, err := facade.Asset.RegisterAsMachine(ctx, asset.ID, usecase.MachineRegistrationInput{
		MachineID:       "M_PIZZA_OVEN_01",
		EquipmentTypeID: "eq_pizza_oven",
		MaxCapacity:     2.0, // 2 trays max capacity
	})
	if err != nil {
		t.Fatalf("Failed to onboard machine: %v", err)
	}

	t.Log("Step 4.2: Verify Machine is created IDLE and Asset status is IN_USE")
	if machine.Status != models.MachineIdle {
		t.Errorf("Expected machine status to be IDLE, got %s", machine.Status)
	}
	updatedAsset, err := facade.Asset.GetAsset(ctx, asset.ID)
	if err != nil || updatedAsset.Status != models.AssetInUse {
		t.Errorf("Expected asset status to be IN_USE, got %s", updatedAsset.Status)
	}
	t.Log("Machine onboarded and ready for scheduling")

	// =========================================================================
	// Scenario 5: Operational Breakdown and Maintenance State Synchronization
	// =========================================================================
	t.Log("Step 5.1: Heating element fails. Manager reports breakdown, triggers maintenance status")
	err = facade.Asset.SyncAssetStatus(ctx, asset.ID, models.AssetUnderMaintenance)
	if err != nil {
		t.Fatalf("Failed to sync status to maintenance: %v", err)
	}

	// Verify both Asset and Machine statuses sync to UNDER_MAINTENANCE
	mch, _ := tc.MachineRepo.FindByID(ctx, machine.ID)
	ast, _ := tc.AssetRepo.FindByID(ctx, asset.ID)
	if mch.Status != models.MachineUnderMaintenance || ast.Status != models.AssetUnderMaintenance {
		t.Errorf("Expected status sync to fail: Machine: %s, Asset: %s", mch.Status, ast.Status)
	}
	t.Log("Kitchen scheduling is blocked because Machine is marked UNDER_MAINTENANCE")

	t.Log("Step 5.2: Repair completed. Manager returns fryer to service")
	err = facade.Asset.SyncAssetStatus(ctx, asset.ID, models.AssetInUse)
	if err != nil {
		t.Fatalf("Failed to sync status back to use: %v", err)
	}

	mch, _ = tc.MachineRepo.FindByID(ctx, machine.ID)
	ast, _ = tc.AssetRepo.FindByID(ctx, asset.ID)
	if mch.Status != models.MachineIdle || ast.Status != models.AssetInUse {
		t.Errorf("Expected status sync back to use to fail: Machine: %s, Asset: %s", mch.Status, ast.Status)
	}
	t.Log("Kitchen scheduling resumes as Machine status returned to IDLE")

	// =========================================================================
	// Scenario 6: Supplier requires Prepayment (Two-Way Matching) before Shipping
	// =========================================================================
	t.Log("Step 6.1: Store Manager requests an Industrial Mixer")
	mixerTypeID := "eq_industrial_mixer"
	mixerName := "Industrial Mixer"
	mixerCapUnit := "liters"
	mixerCap := 50.0

	prMixerLine := services.PRLineInput{
		EquipmentTypeID:       &mixerTypeID,
		ProposedEquipmentName: &mixerName,
		ProposedCapacityUnit:  &mixerCapUnit,
		ExpectedCapacity:      &mixerCap,
		Qty:                   1,
		UnitOfMeasure:         "unit",
		EstimatedUnitPrice:    5000.0,
		Justification:         "Need mixer for new bakery section",
	}

	prMixer, err := facade.PR.SubmitPR(ctx, services.SubmitPRRequest{
		OrgID:           orgID,
		RequesterNodeID: storeNodeID,
		StaffID:         staffID,
		Justification:   "Bakery Expansion",
		Lines:           []services.PRLineInput{prMixerLine},
	})
	if err != nil {
		t.Fatalf("Failed to submit PR for Mixer: %v", err)
	}

	noteMixer := "Approved for Bakery project"
	prMixerLines, err := tc.PRLineRepo.ListByPR(ctx, prMixer.ID)
	if err != nil || len(prMixerLines) == 0 {
		t.Fatalf("Failed to list PR lines for Mixer: %v", err)
	}
	expectedMixerCap := 50.0
	mixerCorrections := []services.PRLineCorrection{
		{
			LineID:          prMixerLines[0].ID,
			EquipmentTypeID: "eq_industrial_mixer",
			ExpectedCapacity: &expectedMixerCap,
			Qty:             1,
			UnitOfMeasure:   "unit",
			EstimatedPrice:  5000.0,
		},
	}
	err = facade.PR.ApprovePR(ctx, prMixer.ID, "staff_hq_admin", &noteMixer, mixerCorrections)
	if err != nil {
		t.Fatalf("Failed to approve PR for Mixer: %v", err)
	}

	t.Log("Step 6.2: HQ creates PurO for Supplier C (Requires Prepayment)")
	suppC := &models.Supplier{
		ID:          "supp_c",
		OrgID:       orgID,
		Name:        "Supplier C (Prepayment Required)",
		ContactInfo: "sales@supplierc.com",
	}
	_ = tc.SupplierRepo.Create(ctx, suppC)

	puroLinesMixer := []usecase.PurOLineInput{
		{
			EquipmentTypeID: &mixerTypeID,
			QtyOrdered:      1,
			PkgUnit:         "unit",
			Conversion:      1.0,
			UnitPrice:       5000.0,
		},
	}
	purOMixer, err := facade.PurO.CreatePRTriggeredPurO(ctx, prMixer.ID, suppC.ID, hqNodeID, "staff_hq_admin", puroLinesMixer)
	if err != nil {
		t.Fatalf("Failed to create PurO for Mixer: %v", err)
	}

	t.Log("Step 6.3: Supplier C issues Invoice for $5000, requesting prepayment")
	invLinesMixer := []usecase.InvoiceLineInput{
		{
			ItemID:    nil,
			Qty:       1.0,
			UnitPrice: 5000.0,
		},
	}
	invMixer, err := facade.Invoice.RecordInvoice(ctx, orgID, purOMixer.ID, suppC.ID, "INV-SUPPC-01", 5000.0, 0.0, "https://doc.supplierc.com/inv01.pdf", invLinesMixer)
	if err != nil {
		t.Fatalf("Failed to record prepayment invoice: %v", err)
	}

	t.Log("Step 6.4: HQ performs Prepayment Match (Two-Way) to authorize payment")
	err = facade.Invoice.PerformPrepaymentMatch(ctx, invMixer.ID, "staff_hq_admin")
	if err != nil {
		t.Fatalf("Prepayment match failed: %v", err)
	}

	t.Log("Step 6.5: HQ pays the invoice")
	err = facade.Invoice.MarkPaid(ctx, invMixer.ID)
	if err != nil {
		t.Fatalf("MarkPaid failed: %v", err)
	}

	t.Log("Step 6.6: Supplier C receives payment and ships the mixer")
	err = facade.PurO.MarkOnWayDelivery(ctx, purOMixer.ID)
	if err != nil {
		t.Fatalf("MarkShipped failed: %v", err)
	}

	t.Log("Step 6.7: Store Manager receives the mixer")
	grInputLinesMixer := []usecase.GRLineInput{
		{
			ItemID:      "",
			QtyExpected: 1.0,
			QtyReceived: 1.0,
		},
	}
	grMixer, err := facade.GR.ConfirmPurOGoodsReceipt(ctx, purOMixer.ID, storeNodeID, staffID, grInputLinesMixer)
	if err != nil {
		t.Fatalf("Failed to record GR for Mixer: %v", err)
	}

	t.Log("Step 6.8: HQ links Goods Receipt to the Prepaid Invoice, completing PurO and creating Asset")
	assetMixer, err := facade.Invoice.LinkGoodsReceiptToPrepaidInvoice(ctx, invMixer.ID, grMixer.ID, "staff_hq_admin")
	if err != nil {
		t.Fatalf("LinkGoodsReceiptToPrepaidInvoice failed: %v", err)
	}
	if assetMixer == nil {
		t.Fatal("Expected Asset to be returned after linking GR to prepaid invoice")
	}

	t.Log("Step 6.9: Verify PurO is COMPLETED and Asset is PENDING_REGISTRATION")
	completedPurOMixer, _ := tc.PurORepo.FindByID(ctx, purOMixer.ID)
	if completedPurOMixer.Status != models.PurchaseOrderCompleted {
		t.Errorf("Expected PurO status to be COMPLETED, got %s", completedPurOMixer.Status)
	}
	if assetMixer.Status != models.AssetPendingRegistration {
		t.Errorf("Expected asset status to be PENDING_REGISTRATION, got %s", assetMixer.Status)
	}

	t.Log("--- BUSINESS INTEGRATION TEST COMPLETED SUCCESSFULLY ---")
}
