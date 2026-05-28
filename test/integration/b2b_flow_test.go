package integration

import (
	"context"
	"testing"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/usecase"
)

func TestB2BBusinessFlow(t *testing.T) {
	ctx := context.Background()

	// Initialize mock or real MongoDB repositories and the facade
	tc := setupFacade()
	facade := tc.Facade

	t.Log("--- B2B SALES INTEGRATION TEST STARTED ---")
	orgID := "org_onesystem"
	hqNodeID := "node_hq"
	factoryNodeID := "node_factory_1"

	// Setup Nodes
	factoryNode := &models.Node{
		ID:    factoryNodeID,
		Name:  "Factory 1",
		OrgID: orgID,
		Type:  models.NodeFactory,
	}
	_ = tc.NodeRepo.Create(ctx, factoryNode)

	// Seed inventory at factory
	err := facade.Inventory.InitStock(ctx, factoryNode.ID, "item_premium_coffee", 500.0)
	if err != nil {
		t.Fatalf("InitStock failed: %v", err)
	}

	t.Log("Step 1: HQ creates B2B order assigned to Factory")
	lines := []usecase.B2BLineInput{
		{ItemID: "item_premium_coffee", QtyOrdered: 100.0, UnitPrice: 15.0},
	}
	b2bOrder, err := facade.B2B.CreateB2BOrder(ctx, orgID, hqNodeID, factoryNodeID, "staff_sales_rep", "External Cafe LLC", "123-456-7890", lines)
	if err != nil {
		t.Fatalf("CreateB2BOrder failed: %v", err)
	}

	if b2bOrder.Status != models.B2BSalesAssigned {
		t.Errorf("Expected status ASSIGNED, got %s", b2bOrder.Status)
	}

	t.Log("Step 2: Factory dispatches goods to the customer")
	giInput := usecase.GoodsIssueInput{
		DriverName:   "Alice External",
		VehiclePlate: "B2B-001",
		MediaURL:     "http://example.com/b2b_dispatch.jpg",
		Lines: []usecase.GILineInput{
			{ItemID: "item_premium_coffee", QtyIssued: 100.0},
		},
	}
	gi, _, err := facade.B2B.DispatchGoods(ctx, b2bOrder.ID, giInput)
	if err != nil {
		t.Fatalf("DispatchGoods failed: %v", err)
	}
	if gi.Status != models.GoodsIssueConfirmed {
		t.Errorf("Expected GI status CONFIRMED, got %s", gi.Status)
	}

	// Verify stock is decremented
	stock, _ := tc.NodeStockRepo.Get(ctx, factoryNodeID, "item_premium_coffee")
	if stock.QtyOnHand != 400.0 { // 500 - 100
		t.Errorf("Expected factory stock 400, got %v", stock.QtyOnHand)
	}

	t.Log("Step 3: HQ confirms delivery with Proof of Delivery")
	err = facade.B2B.ConfirmDelivery(ctx, b2bOrder.ID, "http://example.com/b2b_pod.pdf")
	if err != nil {
		t.Fatalf("ConfirmDelivery failed: %v", err)
	}

	updatedOrder, _, err := facade.B2B.GetB2BOrder(ctx, b2bOrder.ID)
	if err != nil {
		t.Fatalf("GetB2BOrder failed: %v", err)
	}
	if updatedOrder.Status != models.B2BSalesCompleted {
		t.Errorf("Expected B2B order status COMPLETED, got %s", updatedOrder.Status)
	}
	t.Log("B2B flow successful.")
	t.Log("--- B2B SALES INTEGRATION TEST COMPLETED SUCCESSFULLY ---")
}
