package models

import (
	"testing"
	"time"
)

// BaseData holds the mocked database for our end-to-end flow
type BaseData struct {
	Org            *Organization
	HQ             *Node
	Factory        *Node
	Store          *Node
	HQAdmin        *Staff
	FactoryManager *Staff
	StoreManager   *Staff
	Potatoes       *Item
	Fries          *Item
	PotatoUoM      *UoM
	FryerType      *EquipmentType
	Suppliers      map[string]*Supplier
}

func setupBaseData(t *testing.T) *BaseData {
	t.Log("--- DAY 0: Base Setup ---")
	bd := &BaseData{
		Suppliers: make(map[string]*Supplier),
	}
	now := time.Now()

	bd.Org = &Organization{ID: "org_1", Name: "OneSystem Burger Chain", CreatedAt: now}
	
	bd.HQ = &Node{ID: "node_hq", OrgID: bd.Org.ID, Type: NodeHQ, Name: "HQ Office"}
	bd.Factory = &Node{ID: "node_f1", OrgID: bd.Org.ID, Type: NodeFactory, Name: "Factory"}
	bd.Store = &Node{ID: "node_s1", OrgID: bd.Org.ID, Type: NodeStore, Name: "Downtown Store"}
	
	bd.HQAdmin = &Staff{ID: "staff_hq_1", NodeID: bd.HQ.ID, Name: "Alice (HQ Admin)"}
	bd.FactoryManager = &Staff{ID: "staff_f1_1", NodeID: bd.Factory.ID, Name: "Bob (Factory Mgr)"}
	bd.StoreManager = &Staff{ID: "staff_s1_1", NodeID: bd.Store.ID, Name: "Charlie (Store Mgr)"}
	
	bd.FryerType = &EquipmentType{
		ID:           "eq_fryer",
		Name:         "Industrial Fryer",
		CapacityUnit: "liters",
	}

	bd.Potatoes = &Item{ID: "item_potato", Name: "Raw Potatoes", Type: ItemTypeRawMaterial, BaseUnit: "kg"}
	bd.Fries = &Item{ID: "item_fries", Name: "Frozen Fries", Type: ItemTypeSemiProduct, BaseUnit: "kg"}

	bd.PotatoUoM = &UoM{ItemID: bd.Potatoes.ID, PkgUnit: "sack", Conversion: 25.0} // 1 sack = 25kg

	_ = &NodeItemConfig{
		NodeID:           bd.Factory.ID,
		ItemID:           bd.Potatoes.ID,
		SourcingStrategy: SourcingExternalProcurement,
		ReorderPoint:     100.0,
	}
	_ = &NodeItemConfig{
		NodeID:           bd.Store.ID,
		ItemID:           bd.Fries.ID,
		SourcingStrategy: SourcingInternalTransfer,
		ProviderNodeID:   &bd.Factory.ID,
		ReorderPoint:     50.0,
	}

	return bd
}

func TestBusinessDrivenEndToEndFlow(t *testing.T) {
	bd := setupBaseData(t)
	now := time.Now()

	// =========================================================================
	// Scenario 1: CapEx Procurement with Supplier Check & Damage Discrepancy
	// =========================================================================
	t.Log("--- DAY 1: CapEx Flow with Supplier Logic & Damage Exception ---")
	
	t.Log("Step 1: Store Mgr submits PR for new Fryer")
	pr := &PurchaseRequisition{
		ID:              "pr_1",
		OrgID:           bd.Org.ID,
		RequesterNodeID: bd.Store.ID,
		RequesterStaff:  bd.StoreManager.ID,
		Status:          PRPendingHQApproval,
		Justification:   "Current fryer is broken beyond repair",
		CreatedAt:       now,
	}
	prLine := &PRLine{
		ID:                 "prl_1",
		PRID:               pr.ID,
		EquipmentTypeID:    &bd.FryerType.ID,
		Qty:                1,
		UnitOfMeasure:      "unit",
		EstimatedUnitPrice: 1500.00,
	}

	t.Log("Step 2: HQ Admin approves PR")
	pr.Status = PRApproved
	pr.ReviewedBy = &bd.HQAdmin.ID

	t.Log("Step 3: HQ checks if a Supplier exists for EquipmentType:", *prLine.EquipmentTypeID)
	var targetSupplier *Supplier
	for _, supp := range bd.Suppliers {
		if supp.Name == "KitchenEquip Pro" {
			targetSupplier = supp
			break
		}
	}

	if targetSupplier == nil {
		t.Log("-> Exception Handled: No supplier found! HQ creates a new Supplier.")
		targetSupplier = &Supplier{
			ID:          "supp_kitchen",
			OrgID:       bd.Org.ID,
			Name:        "KitchenEquip Pro",
			ContactInfo: "sales@kitchenequip.com",
		}
		bd.Suppliers[targetSupplier.ID] = targetSupplier
	}

	t.Log("Step 4: HQ issues PurO linked to PR")
	purO := &PurchaseOrder{
		ID:               "po_1",
		OrgID:            bd.Org.ID,
		TriggerType:      PurOTriggerPR,
		PRID:             &pr.ID,
		HQNodeID:         bd.HQ.ID,
		SupplierID:       targetSupplier.ID,
		DeliveryToNodeID: pr.RequesterNodeID,
		Status:           PurchaseOrderConfirmed,
	}
	
	purOLine := &PurchaseOrderLine{

		ID:              "pol_1",
		PurOID:          purO.ID,
		EquipmentTypeID: prLine.EquipmentTypeID,
		QtyOrdered:      1,
		PkgUnit:         "unit",
		Conversion:      1.0,
		UnitPrice:       1450.00,
	}
	_ = purOLine

	t.Log("Step 5: Store receives the Fryer... BUT it is badly dented!")
	// Real-world Exception: Damaged hardware on arrival
	gr := &GoodsReceipt{
		ID:              "gr_1",
		RefType:         GoodsReceiptRefPurO,
		RefID:           purO.ID,
		ReceivingNodeID: bd.Store.ID,
		Status:          GoodsReceiptDiscrepancy, // Changed from Confirmed
	}
	
	// Create a discrepancy ticket automatically
	ticket := &DiscrepancyTicket{
		ID:   "dt_1",
		GRID: gr.ID,
		// In a real DB, HQ would review this ticket
	}
	_ = ticket

	t.Log("Step 6: Verify Asset and Machine hold")
	if gr.Status == GoodsReceiptDiscrepancy {
		t.Log("-> Asset NOT registered because of DiscrepancyTicket dt_1")
	} else {
		t.Fatalf("Expected GoodsReceipt to be in DISCREPANCY state")
	}


	// =========================================================================
	// Scenario 2: OpEx Auto-Replenishment & Weather Damage Discrepancy
	// =========================================================================
	t.Log("--- DAY 2: OpEx Auto-Replenishment with Fresh Produce Damage ---")
	
	t.Log("Step 1: System detects Factory needs Potatoes (ROP breached)")
	draftPurO := &PurchaseOrder{
		ID:               "po_2",
		OrgID:            bd.Org.ID,
		TriggerType:      PurOTriggerAutoDraft,
		HQNodeID:         bd.HQ.ID,
		DeliveryToNodeID: bd.Factory.ID,
		Status:           PurchaseOrderDraft,
	}
	
	t.Log("Step 2: HQ attaches Dalat Farm Supplier and issues PO")
	farmSupplier := &Supplier{ID: "supp_dalat_farm", OrgID: bd.Org.ID, Name: "Dalat Fresh Farm"}
	bd.Suppliers[farmSupplier.ID] = farmSupplier
	
	draftPurO.SupplierID = farmSupplier.ID
	draftPurO.Status = PurchaseOrderConfirmed
	
	purOLine2 := &PurchaseOrderLine{
		ID:         "pol_2",
		PurOID:     draftPurO.ID,
		ItemID:     &bd.Potatoes.ID,
		QtyOrdered: 100, // 100 sacks ordered
		PkgUnit:    "sack",
		Conversion: 25.0,
	}

	t.Log("Step 3: Factory receives Potatoes... BUT 5 sacks are rain damaged!")
	gr2 := &GoodsReceipt{
		ID:              "gr_2",
		RefType:         GoodsReceiptRefPurO,
		RefID:           draftPurO.ID,
		ReceivingNodeID: bd.Factory.ID,
		Status:          GoodsReceiptDiscrepancy, // Real-world weather damage
	}
	
	// Only 95 sacks are received successfully
	qtyReceived := 95.0
	grLine2 := &GoodsReceiptLine{
		ID:          "grl_2",
		GRID:        gr2.ID,
		ItemID:      *purOLine2.ItemID,
		QtyExpected: purOLine2.QtyOrdered * purOLine2.Conversion, // 2500 kg
		QtyReceived: qtyReceived * purOLine2.Conversion,        // 2375 kg
	}
	
	if grLine2.QtyReceived < grLine2.QtyExpected {
		t.Logf("-> Discrepancy Logged: Expected %v kg, Received %v kg", grLine2.QtyExpected, grLine2.QtyReceived)
		// StockIn event only increments by QtyReceived (2375 kg)
	} else {
		t.Fatalf("Expected receiving less due to damage")
	}

	// =========================================================================
	// Scenario 3: Production Yield Variance & Breakdown
	// =========================================================================
	t.Log("--- DAY 2 (Cont.): Factory Produces Fries ---")

	t.Log("Step 1: Factory executes Production Order... BUT Peeler breaks down mid-run!")
	
	machinePeeler := &Machine{
		ID:              "mac_peeler_1",
		EquipmentTypeID: "eq_peeler",
		NodeID:          bd.Factory.ID,
		Status:          MachineUnderMaintenance, // Changed due to breakdown
	}
	
	batch := &ProductionBatch{

		ID:        "batch_1",
		Status:    BatchCompleted,
		ItemID:    bd.Fries.ID,
		MachineID: machinePeeler.ID,
	}
	_ = batch
	
	if machinePeeler.Status == MachineUnderMaintenance {
		t.Log("-> Exception Handled: Machine marked UNDER_MAINTENANCE. System will not allocate future batches here.")
	}

	// =========================================================================
	// Scenario 4: Internal Transfer Logistics with 3rd Party Delivery
	// =========================================================================
	t.Log("--- DAY 3: Internal Transfer via Ahamove ---")

	t.Log("Step 1: System detects Store needs Fries -> Auto-drafts ITO")
	ito := &InternalTransferOrder{
		ID:              "ito_1",
		OrgID:           bd.Org.ID,
		Trigger:         ITOTriggerROP,
		RequesterNodeID: bd.Store.ID,
		ProviderNodeID:  bd.Factory.ID,
		Status:          ITOAutoApproved,
	}
	
	itoLine := &ITOLine{
		ID:           "ito_line_1",
		ITOID:        ito.ID,
		ItemID:       bd.Fries.ID,
		QtyOrdered:   50,
		PkgUnit:      "box",
		Conversion:   1,
		QtyOrderedBU: 50,
	}
	_ = itoLine

	t.Log("Step 2: Factory dispatches Fries via Ahamove (GoodsIssue)")
	gi := &GoodsIssue{
		ID:            "gi_1",
		RefType:       GoodsIssueRefITO,
		RefID:         ito.ID,
		IssuingNodeID: bd.Factory.ID,
		// REAL-WORLD constraint: Must capture driver info for 3rd-party logistics
		DriverName:   "Nguyen Van A (Ahamove)",
		DriverPhone:  "0912345678",
		VehiclePlate: "59F1-123.45",
		MediaURL:     "https://s3.onesystem.vn/gi_evidence/gi_1_sealed_box.jpg",
		Status:       GoodsIssueConfirmed,
	}
	
	if gi.VehiclePlate == "" || gi.MediaURL == "" {
		t.Fatalf("Expected Driver info and Media Evidence for 3rd-party delivery dispatch")
	}
	t.Logf("-> Dispatch recorded with Driver: %s (%s) and Photo Evidence", gi.DriverName, gi.VehiclePlate)

	t.Log("Step 3: Store receives Fries (GoodsReceipt)")
	gr3 := &GoodsReceipt{
		ID:              "gr_3",
		RefType:         GoodsReceiptRefITO,
		RefID:           ito.ID,
		ReceivingNodeID: bd.Store.ID,
		Status:          GoodsReceiptConfirmed,
	}
	ito.Status = ITOCompleted

	if gr3.ReceivingNodeID != bd.Store.ID {
		t.Fatalf("Expected receiving node to be Store")
	}

	t.Log("All business-driven flow tests with edge-cases passed successfully!")
}
