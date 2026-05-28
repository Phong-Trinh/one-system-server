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

const collPurchaseOrders = "purchase_orders"
const collPurchaseOrderLines = "purchase_order_lines"

// ── PurchaseOrder ─────────────────────────────────────────────────────────────

type puroDoc struct {
	ID               string                     `bson:"_id"`
	OrgID            string                     `bson:"org_id"`
	TriggerType      models.PurOTriggerType     `bson:"trigger_type"`
	PRID             *string                    `bson:"pr_id,omitempty"`
	HQNodeID         string                     `bson:"hq_node_id"`
	SupplierID       string                     `bson:"supplier_id"`
	DeliveryToNodeID string                     `bson:"delivery_to_node_id"`
	Status           models.PurchaseOrderStatus `bson:"status"`
	ConfirmedBy      *string                    `bson:"confirmed_by,omitempty"`
	ConfirmedAt      *time.Time                 `bson:"confirmed_at,omitempty"`
	CreatedAt        time.Time                  `bson:"created_at"`
	UpdatedAt        time.Time                  `bson:"updated_at"`
}

func puroToDoc(po *models.PurchaseOrder) *puroDoc {
	return &puroDoc{
		ID:               po.ID,
		OrgID:            po.OrgID,
		TriggerType:      po.TriggerType,
		PRID:             po.PRID,
		HQNodeID:         po.HQNodeID,
		SupplierID:       po.SupplierID,
		DeliveryToNodeID: po.DeliveryToNodeID,
		Status:           po.Status,
		ConfirmedBy:      po.ConfirmedBy,
		ConfirmedAt:      po.ConfirmedAt,
		CreatedAt:        po.CreatedAt,
		UpdatedAt:        po.UpdatedAt,
	}
}

func docToPurO(d *puroDoc) *models.PurchaseOrder {
	return &models.PurchaseOrder{
		ID:               d.ID,
		OrgID:            d.OrgID,
		TriggerType:      d.TriggerType,
		PRID:             d.PRID,
		HQNodeID:         d.HQNodeID,
		SupplierID:       d.SupplierID,
		DeliveryToNodeID: d.DeliveryToNodeID,
		Status:           d.Status,
		ConfirmedBy:      d.ConfirmedBy,
		ConfirmedAt:      d.ConfirmedAt,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
}

type puroRepository struct {
	col *mongo.Collection
}

func NewPurchaseOrderRepository(client *Client, dbName string) services.PurchaseOrderRepository {
	col := client.DB(dbName).Collection(collPurchaseOrders)
	_, _ = col.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "org_id", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "delivery_to_node_id", Value: 1}}},
	})
	return &puroRepository{col: col}
}

func (r *puroRepository) Create(ctx context.Context, po *models.PurchaseOrder) error {
	_, err := r.col.InsertOne(ctx, puroToDoc(po))
	if err != nil {
		return fmt.Errorf("puroRepository.Create: %w", err)
	}
	return nil
}

func (r *puroRepository) FindByID(ctx context.Context, id string) (*models.PurchaseOrder, error) {
	var doc puroDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("puroRepository.FindByID: %w", err)
	}
	return docToPurO(&doc), nil
}

func (r *puroRepository) FindByStatus(ctx context.Context, orgID string, status models.PurchaseOrderStatus) ([]*models.PurchaseOrder, error) {
	cur, err := r.col.Find(ctx, bson.M{"org_id": orgID, "status": status})
	if err != nil {
		return nil, fmt.Errorf("puroRepository.FindByStatus: %w", err)
	}
	defer cur.Close(ctx)

	var puros []*models.PurchaseOrder
	for cur.Next(ctx) {
		var doc puroDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		puros = append(puros, docToPurO(&doc))
	}
	return puros, cur.Err()
}

func (r *puroRepository) FindByDeliveryNode(ctx context.Context, nodeID string) ([]*models.PurchaseOrder, error) {
	cur, err := r.col.Find(ctx, bson.M{"delivery_to_node_id": nodeID})
	if err != nil {
		return nil, fmt.Errorf("puroRepository.FindByDeliveryNode: %w", err)
	}
	defer cur.Close(ctx)

	var puros []*models.PurchaseOrder
	for cur.Next(ctx) {
		var doc puroDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		puros = append(puros, docToPurO(&doc))
	}
	return puros, cur.Err()
}

func (r *puroRepository) Update(ctx context.Context, po *models.PurchaseOrder) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": po.ID}, puroToDoc(po))
	if err != nil {
		return fmt.Errorf("puroRepository.Update: %w", err)
	}
	return nil
}

// ── PurchaseOrderLine ─────────────────────────────────────────────────────────

type puroLineDoc struct {
	ID              string  `bson:"_id"`
	PurOID          string  `bson:"puro_id"`
	ItemID          *string `bson:"item_id,omitempty"`
	EquipmentTypeID *string `bson:"equipment_type_id,omitempty"`
	QtyOrdered      float64 `bson:"qty_ordered"`
	PkgUnit         string  `bson:"pkg_unit"`
	Conversion      float64 `bson:"conversion"`
	UnitPrice       float64 `bson:"unit_price"`
}

func puroLineToDoc(line *models.PurchaseOrderLine) *puroLineDoc {
	return &puroLineDoc{
		ID:              line.ID,
		PurOID:          line.PurOID,
		ItemID:          line.ItemID,
		EquipmentTypeID: line.EquipmentTypeID,
		QtyOrdered:      line.QtyOrdered,
		PkgUnit:         line.PkgUnit,
		Conversion:      line.Conversion,
		UnitPrice:       line.UnitPrice,
	}
}

func docToPurOLine(d *puroLineDoc) *models.PurchaseOrderLine {
	return &models.PurchaseOrderLine{
		ID:              d.ID,
		PurOID:          d.PurOID,
		ItemID:          d.ItemID,
		EquipmentTypeID: d.EquipmentTypeID,
		QtyOrdered:      d.QtyOrdered,
		PkgUnit:         d.PkgUnit,
		Conversion:      d.Conversion,
		UnitPrice:       d.UnitPrice,
	}
}

type puroLineRepository struct {
	col *mongo.Collection
}

func NewPurchaseOrderLineRepository(client *Client, dbName string) services.PurchaseOrderLineRepository {
	col := client.DB(dbName).Collection(collPurchaseOrderLines)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "puro_id", Value: 1}},
	})
	return &puroLineRepository{col: col}
}

func (r *puroLineRepository) AddLine(ctx context.Context, line *models.PurchaseOrderLine) error {
	_, err := r.col.InsertOne(ctx, puroLineToDoc(line))
	if err != nil {
		return fmt.Errorf("puroLineRepository.AddLine: %w", err)
	}
	return nil
}

func (r *puroLineRepository) ListByPurO(ctx context.Context, purOID string) ([]*models.PurchaseOrderLine, error) {
	cur, err := r.col.Find(ctx, bson.M{"puro_id": purOID})
	if err != nil {
		return nil, fmt.Errorf("puroLineRepository.ListByPurO: %w", err)
	}
	defer cur.Close(ctx)

	var lines []*models.PurchaseOrderLine
	for cur.Next(ctx) {
		var doc puroLineDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		lines = append(lines, docToPurOLine(&doc))
	}
	return lines, cur.Err()
}

func (r *puroLineRepository) DeleteLine(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("puroLineRepository.DeleteLine: %w", err)
	}
	return nil
}
