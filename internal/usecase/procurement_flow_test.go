package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ─── MOCK REPOSITORIES ────────────────────────────────────────────────────────

type mockPRRepo struct {
	prs map[string]*models.PurchaseRequisition
}

func (m *mockPRRepo) Create(ctx context.Context, pr *models.PurchaseRequisition) error {
	m.prs[pr.ID] = pr
	return nil
}
func (m *mockPRRepo) FindByID(ctx context.Context, id string) (*models.PurchaseRequisition, error) {
	return m.prs[id], nil
}
func (m *mockPRRepo) FindByNode(ctx context.Context, nodeID string) ([]*models.PurchaseRequisition, error) {
	var res []*models.PurchaseRequisition
	for _, pr := range m.prs {
		if pr.RequesterNodeID == nodeID {
			res = append(res, pr)
		}
	}
	return res, nil
}
func (m *mockPRRepo) FindPendingByOrg(ctx context.Context, orgID string) ([]*models.PurchaseRequisition, error) {
	var res []*models.PurchaseRequisition
	for _, pr := range m.prs {
		if pr.OrgID == orgID && pr.Status == models.PRPendingHQApproval {
			res = append(res, pr)
		}
	}
	return res, nil
}
func (m *mockPRRepo) Update(ctx context.Context, pr *models.PurchaseRequisition) error {
	m.prs[pr.ID] = pr
	return nil
}
func (m *mockPRRepo) Delete(ctx context.Context, id string) error {
	delete(m.prs, id)
	return nil
}

type mockPRLineRepo struct {
	lines map[string][]*models.PRLine
}

func (m *mockPRLineRepo) AddLine(ctx context.Context, line *models.PRLine) error {
	m.lines[line.PRID] = append(m.lines[line.PRID], line)
	return nil
}
func (m *mockPRLineRepo) ListByPR(ctx context.Context, prID string) ([]*models.PRLine, error) {
	return m.lines[prID], nil
}
func (m *mockPRLineRepo) DeleteLine(ctx context.Context, id string) error {
	for prID, lines := range m.lines {
		for i, l := range lines {
			if l.ID == id {
				m.lines[prID] = append(lines[:i], lines[i+1:]...)
				return nil
			}
		}
	}
	return nil
}

type mockPurORepo struct {
	puros map[string]*models.PurchaseOrder
}

func (m *mockPurORepo) Create(ctx context.Context, po *models.PurchaseOrder) error {
	m.puros[po.ID] = po
	return nil
}
func (m *mockPurORepo) FindByID(ctx context.Context, id string) (*models.PurchaseOrder, error) {
	return m.puros[id], nil
}
func (m *mockPurORepo) FindByStatus(ctx context.Context, orgID string, status models.PurchaseOrderStatus) ([]*models.PurchaseOrder, error) {
	var res []*models.PurchaseOrder
	for _, po := range m.puros {
		if po.OrgID == orgID && po.Status == status {
			res = append(res, po)
		}
	}
	return res, nil
}
func (m *mockPurORepo) FindByDeliveryNode(ctx context.Context, nodeID string) ([]*models.PurchaseOrder, error) {
	var res []*models.PurchaseOrder
	for _, po := range m.puros {
		if po.DeliveryToNodeID == nodeID {
			res = append(res, po)
		}
	}
	return res, nil
}
func (m *mockPurORepo) Update(ctx context.Context, po *models.PurchaseOrder) error {
	m.puros[po.ID] = po
	return nil
}
func (m *mockPurORepo) Delete(ctx context.Context, id string) error {
	delete(m.puros, id)
	return nil
}

type mockPurOLineRepo struct {
	lines map[string][]*models.PurchaseOrderLine
}

func (m *mockPurOLineRepo) AddLine(ctx context.Context, line *models.PurchaseOrderLine) error {
	m.lines[line.PurOID] = append(m.lines[line.PurOID], line)
	return nil
}
func (m *mockPurOLineRepo) ListByPurO(ctx context.Context, purOID string) ([]*models.PurchaseOrderLine, error) {
	return m.lines[purOID], nil
}
func (m *mockPurOLineRepo) DeleteLine(ctx context.Context, id string) error {
	for purID, lines := range m.lines {
		for i, l := range lines {
			if l.ID == id {
				m.lines[purID] = append(lines[:i], lines[i+1:]...)
				return nil
			}
		}
	}
	return nil
}

type mockSupplierRepo struct {
	suppliers map[string]*models.Supplier
}

func (m *mockSupplierRepo) Create(ctx context.Context, s *models.Supplier) error {
	m.suppliers[s.ID] = s
	return nil
}
func (m *mockSupplierRepo) FindByID(ctx context.Context, id string) (*models.Supplier, error) {
	return m.suppliers[id], nil
}
func (m *mockSupplierRepo) FindByOrg(ctx context.Context, orgID string) ([]*models.Supplier, error) {
	var res []*models.Supplier
	for _, s := range m.suppliers {
		if s.OrgID == orgID {
			res = append(res, s)
		}
	}
	return res, nil
}
func (m *mockSupplierRepo) FindByName(ctx context.Context, orgID, name string) (*models.Supplier, error) {
	for _, s := range m.suppliers {
		if s.OrgID == orgID && s.Name == name {
			return s, nil
		}
	}
	return nil, nil
}
func (m *mockSupplierRepo) Update(ctx context.Context, s *models.Supplier) error {
	m.suppliers[s.ID] = s
	return nil
}
func (m *mockSupplierRepo) Delete(ctx context.Context, id string) error {
	delete(m.suppliers, id)
	return nil
}

type mockGRRepo struct {
	grs map[string]*models.GoodsReceipt
}

func (m *mockGRRepo) Create(ctx context.Context, gr *models.GoodsReceipt) error {
	m.grs[gr.ID] = gr
	return nil
}
func (m *mockGRRepo) FindByID(ctx context.Context, id string) (*models.GoodsReceipt, error) {
	return m.grs[id], nil
}
func (m *mockGRRepo) FindByRef(ctx context.Context, refType models.GoodsReceiptRefType, refID string) ([]*models.GoodsReceipt, error) {
	var res []*models.GoodsReceipt
	for _, gr := range m.grs {
		if gr.RefType == refType && gr.RefID == refID {
			res = append(res, gr)
		}
	}
	return res, nil
}
func (m *mockGRRepo) UpdateStatus(ctx context.Context, id string, status models.GoodsReceiptStatus) error {
	if gr, ok := m.grs[id]; ok {
		gr.Status = status
		return nil
	}
	return fmt.Errorf("gr not found")
}
func (m *mockGRRepo) Update(ctx context.Context, gr *models.GoodsReceipt) error {
	m.grs[gr.ID] = gr
	return nil
}

type mockGRLineRepo struct {
	lines map[string][]*models.GoodsReceiptLine
}

func (m *mockGRLineRepo) AddLine(ctx context.Context, line *models.GoodsReceiptLine) error {
	m.lines[line.GRID] = append(m.lines[line.GRID], line)
	return nil
}
func (m *mockGRLineRepo) ListByGR(ctx context.Context, grID string) ([]*models.GoodsReceiptLine, error) {
	return m.lines[grID], nil
}

type mockDiscrepancyRepo struct {
	tickets map[string]*models.DiscrepancyTicket
}

func (m *mockDiscrepancyRepo) Create(ctx context.Context, dt *models.DiscrepancyTicket) error {
	m.tickets[dt.ID] = dt
	return nil
}
func (m *mockDiscrepancyRepo) FindByID(ctx context.Context, id string) (*models.DiscrepancyTicket, error) {
	return m.tickets[id], nil
}
func (m *mockDiscrepancyRepo) FindByGR(ctx context.Context, grID string) ([]*models.DiscrepancyTicket, error) {
	var res []*models.DiscrepancyTicket
	for _, dt := range m.tickets {
		if dt.GRID == grID {
			res = append(res, dt)
		}
	}
	return res, nil
}
func (m *mockDiscrepancyRepo) UpdateStatus(ctx context.Context, id string, status models.DiscrepancyTicketStatus, resolution *string, resolvedBy *string) error {
	if dt, ok := m.tickets[id]; ok {
		dt.Status = status
		dt.Resolution = resolution
		dt.ResolvedBy = resolvedBy
		now := time.Now()
		dt.ResolvedAt = &now
		return nil
	}
	return fmt.Errorf("ticket not found")
}
func (m *mockDiscrepancyRepo) Update(ctx context.Context, dt *models.DiscrepancyTicket) error {
	m.tickets[dt.ID] = dt
	return nil
}

type mockInvoiceRepo struct {
	invoices map[string]*models.SupplierInvoice
}

func (m *mockInvoiceRepo) Create(ctx context.Context, inv *models.SupplierInvoice) error {
	m.invoices[inv.ID] = inv
	return nil
}
func (m *mockInvoiceRepo) FindByID(ctx context.Context, id string) (*models.SupplierInvoice, error) {
	return m.invoices[id], nil
}
func (m *mockInvoiceRepo) FindByPurO(ctx context.Context, purOID string) ([]*models.SupplierInvoice, error) {
	var res []*models.SupplierInvoice
	for _, inv := range m.invoices {
		if inv.PurchaseOrderID == purOID {
			res = append(res, inv)
		}
	}
	return res, nil
}
func (m *mockInvoiceRepo) Update(ctx context.Context, inv *models.SupplierInvoice) error {
	m.invoices[inv.ID] = inv
	return nil
}

type mockInvoiceLineRepo struct {
	lines map[string][]*models.SupplierInvoiceLine
}

func (m *mockInvoiceLineRepo) AddLine(ctx context.Context, line *models.SupplierInvoiceLine) error {
	m.lines[line.InvoiceID] = append(m.lines[line.InvoiceID], line)
	return nil
}
func (m *mockInvoiceLineRepo) ListByInvoice(ctx context.Context, invoiceID string) ([]*models.SupplierInvoiceLine, error) {
	return m.lines[invoiceID], nil
}

type mockTxRepo struct {
	txs map[string]*models.Transaction
}

func (m *mockTxRepo) Create(ctx context.Context, tx *models.Transaction) error {
	m.txs[tx.ID] = tx
	return nil
}
func (m *mockTxRepo) FindByID(ctx context.Context, id string) (*models.Transaction, error) {
	return m.txs[id], nil
}
func (m *mockTxRepo) ListByNode(ctx context.Context, nodeID string, txType *models.TransactionType) ([]*models.Transaction, error) {
	var res []*models.Transaction
	for _, tx := range m.txs {
		if tx.NodeID == nodeID && (txType == nil || tx.Type == *txType) {
			res = append(res, tx)
		}
	}
	return res, nil
}
func (m *mockTxRepo) ListByRef(ctx context.Context, refType models.TransactionRefType, refID string) ([]*models.Transaction, error) {
	var res []*models.Transaction
	for _, tx := range m.txs {
		if tx.RefType == refType && tx.ReferenceID == refID {
			res = append(res, tx)
		}
	}
	return res, nil
}

type mockAssetRepo struct {
	assets map[string]*models.Asset
}

func (m *mockAssetRepo) Create(ctx context.Context, asset *models.Asset) error {
	m.assets[asset.ID] = asset
	return nil
}
func (m *mockAssetRepo) FindByID(ctx context.Context, id string) (*models.Asset, error) {
	return m.assets[id], nil
}
func (m *mockAssetRepo) FindByNode(ctx context.Context, nodeID string) ([]*models.Asset, error) {
	var res []*models.Asset
	for _, a := range m.assets {
		if a.NodeID == nodeID {
			res = append(res, a)
		}
	}
	return res, nil
}
func (m *mockAssetRepo) FindByPurO(ctx context.Context, purOID string) (*models.Asset, error) {
	for _, a := range m.assets {
		if a.LinkedPurOID == purOID {
			return a, nil
		}
	}
	return nil, nil
}
func (m *mockAssetRepo) Update(ctx context.Context, asset *models.Asset) error {
	m.assets[asset.ID] = asset
	return nil
}

type mockMachineRepo struct {
	machines map[string]*models.Machine
}

func (m *mockMachineRepo) Create(ctx context.Context, mch *models.Machine) error {
	m.machines[mch.ID] = mch
	return nil
}
func (m *mockMachineRepo) FindByID(ctx context.Context, id string) (*models.Machine, error) {
	return m.machines[id], nil
}
func (m *mockMachineRepo) FindByNodeID(ctx context.Context, nodeID string) ([]*models.Machine, error) {
	var res []*models.Machine
	for _, mch := range m.machines {
		if mch.NodeID == nodeID {
			res = append(res, mch)
		}
	}
	return res, nil
}
func (m *mockMachineRepo) FindAll(ctx context.Context) ([]*models.Machine, error) {
	var res []*models.Machine
	for _, mch := range m.machines {
		res = append(res, mch)
	}
	return res, nil
}
func (m *mockMachineRepo) FindIdleByStationType(ctx context.Context, nodeID, stationTypeID string) ([]*models.Machine, error) {
	var res []*models.Machine
	for _, mch := range m.machines {
		if mch.NodeID == nodeID && mch.EquipmentTypeID == stationTypeID && mch.Status == models.MachineIdle {
			res = append(res, mch)
		}
	}
	return res, nil
}
func (m *mockMachineRepo) UpdateStatus(ctx context.Context, id string, status models.MachineStatus, batchID *string) error {
	if mch, ok := m.machines[id]; ok {
		mch.Status = status
		mch.CurrentBatchID = batchID
		return nil
	}
	return fmt.Errorf("machine not found")
}
func (m *mockMachineRepo) Update(ctx context.Context, mch *models.Machine) error {
	m.machines[mch.ID] = mch
	return nil
}
func (m *mockMachineRepo) Delete(ctx context.Context, id string) error {
	delete(m.machines, id)
	return nil
}

type mockEquipmentTypeRepo struct {
	types map[string]*models.EquipmentType
}

func (m *mockEquipmentTypeRepo) Create(ctx context.Context, st *models.EquipmentType) error {
	m.types[st.ID] = st
	return nil
}
func (m *mockEquipmentTypeRepo) FindByID(ctx context.Context, id string) (*models.EquipmentType, error) {
	return m.types[id], nil
}
func (m *mockEquipmentTypeRepo) FindAll(ctx context.Context) ([]*models.EquipmentType, error) {
	var res []*models.EquipmentType
	for _, t := range m.types {
		res = append(res, t)
	}
	return res, nil
}
func (m *mockEquipmentTypeRepo) Update(ctx context.Context, st *models.EquipmentType) error {
	m.types[st.ID] = st
	return nil
}
func (m *mockEquipmentTypeRepo) Delete(ctx context.Context, id string) error {
	delete(m.types, id)
	return nil
}

type mockInventoryService struct{}

func (m *mockInventoryService) GetStock(ctx context.Context, nodeID, itemID string) (float64, error) {
	return 0, nil
}
func (m *mockInventoryService) InitStock(ctx context.Context, nodeID, itemID string, qtyBU float64) error {
	return nil
}
func (m *mockInventoryService) StockIn(ctx context.Context, nodeID, itemID string, qtyBU float64) error {
	return nil
}
func (m *mockInventoryService) StockOut(ctx context.Context, nodeID, itemID string, qtyBU float64) (*services.ROPCheckResult, error) {
	return &services.ROPCheckResult{Breached: false}, nil
}
func (m *mockInventoryService) CheckROP(ctx context.Context, nodeID, itemID string) (*services.ROPCheckResult, error) {
	return &services.ROPCheckResult{Breached: false}, nil
}

// ─── INTEGRATION TEST CASE ───────────────────────────────────────────────────

func TestCapExProcurementBusinessFlow(t *testing.T) {
	ctx := context.Background()

	// Initialize mock repositories
	prRepo := &mockPRRepo{prs: make(map[string]*models.PurchaseRequisition)}
	prLineRepo := &mockPRLineRepo{lines: make(map[string][]*models.PRLine)}
	purORepo := &mockPurORepo{puros: make(map[string]*models.PurchaseOrder)}
	purOLineRepo := &mockPurOLineRepo{lines: make(map[string][]*models.PurchaseOrderLine)}
	supplierRepo := &mockSupplierRepo{suppliers: make(map[string]*models.Supplier)}
	grRepo := &mockGRRepo{grs: make(map[string]*models.GoodsReceipt)}
	grLineRepo := &mockGRLineRepo{lines: make(map[string][]*models.GoodsReceiptLine)}
	dtRepo := &mockDiscrepancyRepo{tickets: make(map[string]*models.DiscrepancyTicket)}
	invoiceRepo := &mockInvoiceRepo{invoices: make(map[string]*models.SupplierInvoice)}
	invoiceLineRepo := &mockInvoiceLineRepo{lines: make(map[string][]*models.SupplierInvoiceLine)}
	txRepo := &mockTxRepo{txs: make(map[string]*models.Transaction)}
	assetRepo := &mockAssetRepo{assets: make(map[string]*models.Asset)}
	machineRepo := &mockMachineRepo{machines: make(map[string]*models.Machine)}
	eqTypeRepo := &mockEquipmentTypeRepo{types: make(map[string]*models.EquipmentType)}

	// Setup Usecases
	prUC := newPRUseCase(prRepo, prLineRepo, eqTypeRepo)
	purOUC := newPurOUseCase(purORepo, purOLineRepo, prRepo, prLineRepo, supplierRepo)
	grUC := newGRUseCase(grRepo, grLineRepo, dtRepo, purORepo, &mockInventoryService{})
	invoiceUC := newInvoiceUseCase(invoiceRepo, invoiceLineRepo, txRepo, purORepo, grRepo)
	assetUC := newAssetUseCase(assetRepo, machineRepo, purORepo, grRepo, prRepo, prLineRepo)

	// Bind late-bound circular dependencies
	purOUC.setAssetUseCase(assetUC)
	invoiceUC.setPurOUseCase(purOUC)

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
	pr, err := prUC.SubmitPR(ctx, services.SubmitPRRequest{
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
	draftEqType, err := eqTypeRepo.FindByID(ctx, pizzaOvenTypeID)
	if err != nil || draftEqType == nil {
		t.Fatalf("Expected EquipmentType %s to be registered as DRAFT, got nil or error: %v", pizzaOvenTypeID, err)
	}
	if draftEqType.Status != models.EquipmentTypeDraft {
		t.Errorf("Expected draft EquipmentType status to be DRAFT, got %s", draftEqType.Status)
	}
	t.Log("EquipmentType was automatically created as DRAFT in catalog upon PR submission")

	t.Log("Step 1.2: HQ reviews the PR and approves it. The system automatically activates the EquipmentType 'eq_pizza_oven' in the catalog.")
	note := "Approved for Pizza Expansion project"
	err = prUC.ApprovePR(ctx, pr.ID, "staff_hq_admin", &note)
	if err != nil {
		t.Fatalf("Failed to approve PR: %v", err)
	}

	// Verify that the draft EquipmentType has been activated to ACTIVE
	registeredEqType, err := eqTypeRepo.FindByID(ctx, pizzaOvenTypeID)
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
	updatedPR, _, err := prUC.GetPR(ctx, pr.ID)
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
	_ = supplierRepo.Create(ctx, suppA)

	t.Log("Step 2.2: Convert approved PR to PurO with Supplier A")
	puroLines := []PurOLineInput{
		{
			EquipmentTypeID: &pizzaOvenTypeID,
			QtyOrdered:      1,
			PkgUnit:         "unit",
			Conversion:      1.0,
			UnitPrice:       2900.0, // Negotiated price is $2900
		},
	}
	purO, err := purOUC.CreatePRTriggeredPurO(ctx, pr.ID, suppA.ID, hqNodeID, "staff_hq_admin", puroLines)
	if err != nil {
		t.Fatalf("Failed to create PR triggered PO: %v", err)
	}
	if purO.Status != models.PurchaseOrderConfirmed {
		t.Errorf("Expected PurO status to be CONFIRMED, got %s", purO.Status)
	}
	t.Logf("PurO #1 issued to Supplier A with ID: %s", purO.ID)

	t.Log("Step 2.3: Supplier A ships the machine, but it arrives completely damaged (QtyReceived = 0)")
	err = purOUC.MarkShipped(ctx, purO.ID)
	if err != nil {
		t.Fatalf("Failed to mark shipped: %v", err)
	}

	grInputLines := []GRLineInput{
		{
			ItemID:      "", // CapEx line has no ItemID
			QtyExpected: 1.0,
			QtyReceived: 0.0, // Damaged, rejected at receiving bay!
		},
	}
	gr, err := grUC.ConfirmPurOGoodsReceipt(ctx, purO.ID, storeNodeID, staffID, grInputLines)
	if err != nil {
		t.Fatalf("Failed to record GR: %v", err)
	}
	if gr.Status != models.GoodsReceiptDiscrepancy {
		t.Errorf("Expected GR status to be DISCREPANCY, got %s", gr.Status)
	}

	t.Log("Step 2.4: Assert a DiscrepancyTicket is automatically created, blocking asset creation")
	tickets, err := dtRepo.FindByGR(ctx, gr.ID)
	if err != nil || len(tickets) == 0 {
		t.Fatalf("Expected discrepancy ticket to be created, got error: %v, count: %d", err, len(tickets))
	}
	if tickets[0].Status != models.DiscrepancyOpen {
		t.Errorf("Expected ticket status to be OPEN, got %s", tickets[0].Status)
	}

	t.Log("Step 2.5: HQ decides to cancel Supplier A's order due to their bad behavior, and pivots to Supplier B")
	// Set PurO to Cancelled (HQ action due to breach of service quality)
	purO.Status = models.PurchaseOrderCancelled
	_ = purORepo.Update(ctx, purO)
	t.Logf("PurO #1 (Supplier A) cancelled. Status: %s", purO.Status)

	// HQ reverts PR status back to APPROVED so it can be re-converted
	pr.Status = models.PRApproved
	_ = prRepo.Update(ctx, pr)
	t.Log("PR status reset back to APPROVED to allow pivot to Supplier B")

	// Setup Supplier B (Premium Logistics)
	suppB := &models.Supplier{
		ID:          "supp_b",
		OrgID:       orgID,
		Name:        "Supplier B (Premium Logistics)",
		ContactInfo: "sales@supplierb.com",
	}
	_ = supplierRepo.Create(ctx, suppB)

	t.Log("Step 2.6: HQ issues a new PurO directly to Supplier B for the same oven")
	purO2, err := purOUC.CreatePRTriggeredPurO(ctx, pr.ID, suppB.ID, hqNodeID, "staff_hq_admin", puroLines)
	if err != nil {
		t.Fatalf("Failed to create new PO for Supplier B: %v", err)
	}
	t.Logf("PurO #2 issued to Supplier B with ID: %s", purO2.ID)

	t.Log("Step 2.7: Supplier B delivers successfully. Store Manager receives it intact")
	err = purOUC.MarkShipped(ctx, purO2.ID)
	if err != nil {
		t.Fatalf("Failed to mark shipped: %v", err)
	}

	grInputLines2 := []GRLineInput{
		{
			ItemID:      "",
			QtyExpected: 1.0,
			QtyReceived: 1.0, // Perfect delivery!
		},
	}
	gr2, err := grUC.ConfirmPurOGoodsReceipt(ctx, purO2.ID, storeNodeID, staffID, grInputLines2)
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
	invLines := []InvoiceLineInput{
		{
			ItemID:    nil,
			Qty:       1.0,
			UnitPrice: 2900.0,
		},
	}
	invoice, err := invoiceUC.RecordInvoice(ctx, orgID, purO2.ID, suppB.ID, "INV-SUPPB-99", 2900.0, 0.0, "https://doc.supplierb.com/inv99.pdf", invLines)
	if err != nil {
		t.Fatalf("Failed to record supplier invoice: %v", err)
	}

	// Match: PurO2 ($2900) + Supplier Invoice ($2900) + Goods Receipt gr2 (1 unit)
	asset, err := invoiceUC.PerformThreeWayMatch(ctx, invoice.ID, gr2.ID, "staff_hq_admin")
	if err != nil {
		t.Fatalf("Three-Way Matching failed: %v", err)
	}
	if asset == nil {
		t.Fatal("Expected Asset record to be returned on payment settlement, got nil")
	}

	t.Log("Step 3.2: Verify Invoice is MATCHED and Purchase Order is COMPLETED")
	matchedInv, _, _ := invoiceUC.GetInvoice(ctx, invoice.ID)
	if matchedInv.Status != models.SupplierInvoiceMatched {
		t.Errorf("Expected invoice status to be MATCHED, got %s", matchedInv.Status)
	}
	completedPO, err := purORepo.FindByID(ctx, purO2.ID)
	if err != nil || completedPO.Status != models.PurchaseOrderCompleted {
		t.Errorf("Expected PurO status to be COMPLETED, got status: %s", completedPO.Status)
	}

	t.Log("Step 3.3: Verify Expense transaction is registered in the General Ledger")
	txs, err := txRepo.ListByNode(ctx, storeNodeID, nil)
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

	machine, err := assetUC.RegisterAsMachine(ctx, asset.ID, MachineRegistrationInput{
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
	updatedAsset, err := assetUC.GetAsset(ctx, asset.ID)
	if err != nil || updatedAsset.Status != models.AssetInUse {
		t.Errorf("Expected asset status to be IN_USE, got %s", updatedAsset.Status)
	}
	t.Log("Machine onboarded and ready for scheduling")

	// =========================================================================
	// Scenario 5: Operational Breakdown and Maintenance State Synchronization
	// =========================================================================
	t.Log("Step 5.1: Heating element fails. Manager reports breakdown, triggers maintenance status")
	err = assetUC.SyncAssetStatus(ctx, asset.ID, models.AssetUnderMaintenance)
	if err != nil {
		t.Fatalf("Failed to sync status to maintenance: %v", err)
	}

	// Verify both Asset and Machine statuses sync to UNDER_MAINTENANCE
	mch, _ := machineRepo.FindByID(ctx, machine.ID)
	ast, _ := assetRepo.FindByID(ctx, asset.ID)
	if mch.Status != models.MachineUnderMaintenance || ast.Status != models.AssetUnderMaintenance {
		t.Errorf("Expected status sync to fail: Machine: %s, Asset: %s", mch.Status, ast.Status)
	}
	t.Log("Kitchen scheduling is blocked because Machine is marked UNDER_MAINTENANCE")

	t.Log("Step 5.2: Repair completed. Manager returns fryer to service")
	err = assetUC.SyncAssetStatus(ctx, asset.ID, models.AssetInUse)
	if err != nil {
		t.Fatalf("Failed to sync status back to use: %v", err)
	}

	mch, _ = machineRepo.FindByID(ctx, machine.ID)
	ast, _ = assetRepo.FindByID(ctx, asset.ID)
	if mch.Status != models.MachineIdle || ast.Status != models.AssetInUse {
		t.Errorf("Expected status sync back to use to fail: Machine: %s, Asset: %s", mch.Status, ast.Status)
	}
	t.Log("Kitchen scheduling resumes as Machine status returned to IDLE")

	t.Log("--- BUSINESS INTEGRATION TEST COMPLETED SUCCESSFULLY ---")
}
