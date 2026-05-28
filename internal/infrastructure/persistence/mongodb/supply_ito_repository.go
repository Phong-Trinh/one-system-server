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

const collITO = "internal_transfer_orders"
const collITOLines = "internal_transfer_order_lines"

// ── ITO ───────────────────────────────────────────────────────────────────────

type itoDoc struct {
	ID              string            `bson:"_id"`
	OrgID           string            `bson:"org_id"`
	RequesterNodeID string            `bson:"requester_node_id"`
	ProviderNodeID  string            `bson:"provider_node_id"`
	Trigger         models.ITOTrigger `bson:"trigger"`
	Status          models.ITOStatus  `bson:"status"`
	IsSameSite      bool              `bson:"is_same_site"`
	RequestedBy     *string           `bson:"requested_by,omitempty"`
	CreatedAt       time.Time         `bson:"created_at"`
	UpdatedAt       time.Time         `bson:"updated_at"`
}

func itoToDoc(ito *models.InternalTransferOrder) *itoDoc {
	return &itoDoc{
		ID:              ito.ID,
		OrgID:           ito.OrgID,
		RequesterNodeID: ito.RequesterNodeID,
		ProviderNodeID:  ito.ProviderNodeID,
		Trigger:         ito.Trigger,
		Status:          ito.Status,
		IsSameSite:      ito.IsSameSite,
		RequestedBy:     ito.RequestedBy,
		CreatedAt:       ito.CreatedAt,
		UpdatedAt:       ito.UpdatedAt,
	}
}

func docToITO(d *itoDoc) *models.InternalTransferOrder {
	return &models.InternalTransferOrder{
		ID:              d.ID,
		OrgID:           d.OrgID,
		RequesterNodeID: d.RequesterNodeID,
		ProviderNodeID:  d.ProviderNodeID,
		Trigger:         d.Trigger,
		Status:          d.Status,
		IsSameSite:      d.IsSameSite,
		RequestedBy:     d.RequestedBy,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

type itoRepository struct {
	col *mongo.Collection
}

func NewInternalTransferOrderRepository(client *Client, dbName string) services.InternalTransferOrderRepository {
	col := client.DB(dbName).Collection(collITO)
	_, _ = col.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "requester_node_id", Value: 1}}},
		{Keys: bson.D{{Key: "provider_node_id", Value: 1}}},
	})
	return &itoRepository{col: col}
}

func (r *itoRepository) Create(ctx context.Context, ito *models.InternalTransferOrder) error {
	_, err := r.col.InsertOne(ctx, itoToDoc(ito))
	if err != nil {
		return fmt.Errorf("itoRepository.Create: %w", err)
	}
	return nil
}

func (r *itoRepository) FindByID(ctx context.Context, id string) (*models.InternalTransferOrder, error) {
	var doc itoDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("itoRepository.FindByID: %w", err)
	}
	return docToITO(&doc), nil
}

func (r *itoRepository) FindByNode(ctx context.Context, nodeID string) ([]*models.InternalTransferOrder, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"requester_node_id": nodeID},
			{"provider_node_id": nodeID},
		},
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("itoRepository.FindByNode: %w", err)
	}
	defer cur.Close(ctx)

	var itos []*models.InternalTransferOrder
	for cur.Next(ctx) {
		var doc itoDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		itos = append(itos, docToITO(&doc))
	}
	return itos, cur.Err()
}

func (r *itoRepository) UpdateStatus(ctx context.Context, id string, status models.ITOStatus) error {
	update := bson.M{"$set": bson.M{"status": status, "updated_at": time.Now()}}
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("itoRepository.UpdateStatus: %w", err)
	}
	fmt.Printf("[DEBUG] itoRepository.UpdateStatus id=%s status=%s matched=%d modified=%d\n", id, status, res.MatchedCount, res.ModifiedCount)
	return nil
}

// ── ITOLine ───────────────────────────────────────────────────────────────────

type itoLineDoc struct {
	ID            string   `bson:"_id"`
	ITOID         string   `bson:"ito_id"`
	ItemID        string   `bson:"item_id"`
	QtyOrdered    float64  `bson:"qty_ordered"`
	PkgUnit       string   `bson:"pkg_unit"`
	Conversion    float64  `bson:"conversion"`
	QtyOrderedBU  float64  `bson:"qty_ordered_bu"`
	QtyReceived   *float64 `bson:"qty_received,omitempty"`
	QtyReceivedBU *float64 `bson:"qty_received_bu,omitempty"`
}

func lineToDoc(line *models.ITOLine) *itoLineDoc {
	return &itoLineDoc{
		ID:            line.ID,
		ITOID:         line.ITOID,
		ItemID:        line.ItemID,
		QtyOrdered:    line.QtyOrdered,
		PkgUnit:       line.PkgUnit,
		Conversion:    line.Conversion,
		QtyOrderedBU:  line.QtyOrderedBU,
		QtyReceived:   line.QtyReceived,
		QtyReceivedBU: line.QtyReceivedBU,
	}
}

func docToLine(d *itoLineDoc) *models.ITOLine {
	return &models.ITOLine{
		ID:            d.ID,
		ITOID:         d.ITOID,
		ItemID:        d.ItemID,
		QtyOrdered:    d.QtyOrdered,
		PkgUnit:       d.PkgUnit,
		Conversion:    d.Conversion,
		QtyOrderedBU:  d.QtyOrderedBU,
		QtyReceived:   d.QtyReceived,
		QtyReceivedBU: d.QtyReceivedBU,
	}
}

type itoLineRepository struct {
	col *mongo.Collection
}

func NewITOLineRepository(client *Client, dbName string) services.ITOLineRepository {
	col := client.DB(dbName).Collection(collITOLines)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "ito_id", Value: 1}},
	})
	return &itoLineRepository{col: col}
}

func (r *itoLineRepository) AddLine(ctx context.Context, line *models.ITOLine) error {
	_, err := r.col.InsertOne(ctx, lineToDoc(line))
	if err != nil {
		return fmt.Errorf("itoLineRepository.AddLine: %w", err)
	}
	return nil
}

func (r *itoLineRepository) ListByITO(ctx context.Context, itoID string) ([]*models.ITOLine, error) {
	cur, err := r.col.Find(ctx, bson.M{"ito_id": itoID})
	if err != nil {
		return nil, fmt.Errorf("itoLineRepository.ListByITO: %w", err)
	}
	defer cur.Close(ctx)

	var lines []*models.ITOLine
	for cur.Next(ctx) {
		var doc itoLineDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		lines = append(lines, docToLine(&doc))
	}
	return lines, cur.Err()
}
