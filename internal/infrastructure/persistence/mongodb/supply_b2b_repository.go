package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

const collB2BSalesOrders = "b2b_sales_orders"
const collB2BSalesOrderLines = "b2b_sales_order_lines"

// ── B2BSalesOrder ─────────────────────────────────────────────────────────────

type b2bOrderDoc struct {
	ID              string                `bson:"_id"`
	OrgID           string                `bson:"org_id"`
	HQNodeID        string                `bson:"hq_node_id"`
	FactoryNodeID   string                `bson:"factory_node_id"`
	CustomerName    string                `bson:"customer_name"`
	CustomerContact string                `bson:"customer_contact"`
	Status          models.B2BSalesStatus `bson:"status"`
	ProofOfDelivery *string               `bson:"proof_of_delivery,omitempty"`
	CreatedBy       string                `bson:"created_by"`
	CreatedAt       time.Time             `bson:"created_at"`
	UpdatedAt       time.Time             `bson:"updated_at"`
}

func b2bOrderToDoc(o *models.B2BSalesOrder) *b2bOrderDoc {
	return &b2bOrderDoc{
		ID:              o.ID,
		OrgID:           o.OrgID,
		HQNodeID:        o.HQNodeID,
		FactoryNodeID:   o.FactoryNodeID,
		CustomerName:    o.CustomerName,
		CustomerContact: o.CustomerContact,
		Status:          o.Status,
		ProofOfDelivery: o.ProofOfDelivery,
		CreatedBy:       o.CreatedBy,
		CreatedAt:       o.CreatedAt,
		UpdatedAt:       o.UpdatedAt,
	}
}

func docToB2BOrder(d *b2bOrderDoc) *models.B2BSalesOrder {
	return &models.B2BSalesOrder{
		ID:              d.ID,
		OrgID:           d.OrgID,
		HQNodeID:        d.HQNodeID,
		FactoryNodeID:   d.FactoryNodeID,
		CustomerName:    d.CustomerName,
		CustomerContact: d.CustomerContact,
		Status:          d.Status,
		ProofOfDelivery: d.ProofOfDelivery,
		CreatedBy:       d.CreatedBy,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

type b2bOrderRepository struct {
	col *mongo.Collection
}

func NewB2BSalesOrderRepository(client *Client, dbName string) services.B2BSalesOrderRepository {
	col := client.DB(dbName).Collection(collB2BSalesOrders)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "factory_node_id", Value: 1}},
	})
	return &b2bOrderRepository{col: col}
}

func (r *b2bOrderRepository) Create(ctx context.Context, order *models.B2BSalesOrder) error {
	_, err := r.col.InsertOne(ctx, b2bOrderToDoc(order))
	if err != nil {
		return fmt.Errorf("b2bOrderRepository.Create: %w", err)
	}
	return nil
}

func (r *b2bOrderRepository) FindByID(ctx context.Context, id string) (*models.B2BSalesOrder, error) {
	var doc b2bOrderDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("b2bOrderRepository.FindByID: %w", err)
	}
	return docToB2BOrder(&doc), nil
}

func (r *b2bOrderRepository) FindByFactory(ctx context.Context, factoryNodeID string) ([]*models.B2BSalesOrder, error) {
	cur, err := r.col.Find(ctx, bson.M{"factory_node_id": factoryNodeID})
	if err != nil {
		return nil, fmt.Errorf("b2bOrderRepository.FindByFactory: %w", err)
	}
	defer cur.Close(ctx)

	var orders []*models.B2BSalesOrder
	for cur.Next(ctx) {
		var doc b2bOrderDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		orders = append(orders, docToB2BOrder(&doc))
	}
	return orders, cur.Err()
}

func (r *b2bOrderRepository) Update(ctx context.Context, order *models.B2BSalesOrder) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": order.ID}, b2bOrderToDoc(order))
	if err != nil {
		return fmt.Errorf("b2bOrderRepository.Update: %w", err)
	}
	return nil
}

// ── B2BSalesOrderLine ─────────────────────────────────────────────────────────

type b2bOrderLineDoc struct {
	ID         string  `bson:"_id"`
	OrderID    string  `bson:"order_id"`
	ItemID     string  `bson:"item_id"`
	QtyOrdered float64 `bson:"qty_ordered"`
	UnitPrice  float64 `bson:"unit_price"`
}

func b2bLineToDoc(line *models.B2BSalesOrderLine) *b2bOrderLineDoc {
	return &b2bOrderLineDoc{
		ID:         line.ID,
		OrderID:    line.OrderID,
		ItemID:     line.ItemID,
		QtyOrdered: line.QtyOrdered,
		UnitPrice:  line.UnitPrice,
	}
}

func docToB2BLine(d *b2bOrderLineDoc) *models.B2BSalesOrderLine {
	return &models.B2BSalesOrderLine{
		ID:         d.ID,
		OrderID:    d.OrderID,
		ItemID:     d.ItemID,
		QtyOrdered: d.QtyOrdered,
		UnitPrice:  d.UnitPrice,
	}
}

type b2bOrderLineRepository struct {
	col *mongo.Collection
}

func NewB2BSalesOrderLineRepository(client *Client, dbName string) services.B2BSalesOrderLineRepository {
	col := client.DB(dbName).Collection(collB2BSalesOrderLines)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "order_id", Value: 1}},
	})
	return &b2bOrderLineRepository{col: col}
}

func (r *b2bOrderLineRepository) AddLine(ctx context.Context, line *models.B2BSalesOrderLine) error {
	_, err := r.col.InsertOne(ctx, b2bLineToDoc(line))
	if err != nil {
		return fmt.Errorf("b2bOrderLineRepository.AddLine: %w", err)
	}
	return nil
}

func (r *b2bOrderLineRepository) ListByOrder(ctx context.Context, orderID string) ([]*models.B2BSalesOrderLine, error) {
	cur, err := r.col.Find(ctx, bson.M{"order_id": orderID})
	if err != nil {
		return nil, fmt.Errorf("b2bOrderLineRepository.ListByOrder: %w", err)
	}
	defer cur.Close(ctx)

	var lines []*models.B2BSalesOrderLine
	for cur.Next(ctx) {
		var doc b2bOrderLineDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		lines = append(lines, docToB2BLine(&doc))
	}
	return lines, cur.Err()
}
