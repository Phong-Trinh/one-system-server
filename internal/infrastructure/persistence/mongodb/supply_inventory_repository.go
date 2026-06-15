package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

const collNodeStock = "node_stocks"
const collNodeItemConfig = "node_item_configs"

// ── NodeStock ─────────────────────────────────────────────────────────────────

type nodeStockDoc struct {
	ItemID        string    `bson:"item_id"`
	NodeID        string    `bson:"node_id"`
	QtyOnHand     float64   `bson:"qty_on_hand"`
	LastUpdatedAt time.Time `bson:"last_updated_at"`
}

func stockToDoc(s *models.NodeStock) *nodeStockDoc {
	return &nodeStockDoc{
		ItemID:        s.ItemID,
		NodeID:        s.NodeID,
		QtyOnHand:     s.QtyOnHand,
		LastUpdatedAt: s.LastUpdatedAt,
	}
}

func docToStock(d *nodeStockDoc) *models.NodeStock {
	return &models.NodeStock{
		ItemID:        d.ItemID,
		NodeID:        d.NodeID,
		QtyOnHand:     d.QtyOnHand,
		LastUpdatedAt: d.LastUpdatedAt,
	}
}

type nodeStockRepository struct {
	col *mongo.Collection
}

func NewNodeStockRepository(client *Client, dbName string) services.NodeStockRepository {
	col := client.DB(dbName).Collection(collNodeStock)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "node_id", Value: 1}, {Key: "item_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &nodeStockRepository{col: col}
}

func (r *nodeStockRepository) Get(ctx context.Context, nodeID, itemID string) (*models.NodeStock, error) {
	var doc nodeStockDoc
	err := r.col.FindOne(ctx, bson.M{"node_id": nodeID, "item_id": itemID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("nodeStockRepository.Get: %w", err)
	}
	return docToStock(&doc), nil
}

func (r *nodeStockRepository) Upsert(ctx context.Context, stock *models.NodeStock) error {
	filter := bson.M{"node_id": stock.NodeID, "item_id": stock.ItemID}
	update := bson.M{"$set": stockToDoc(stock)}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("nodeStockRepository.Upsert: %w", err)
	}
	return nil
}

func (r *nodeStockRepository) ListByNode(ctx context.Context, nodeID string) ([]*models.NodeStock, error) {
	cur, err := r.col.Find(ctx, bson.M{"node_id": nodeID})
	if err != nil {
		return nil, fmt.Errorf("nodeStockRepository.ListByNode: %w", err)
	}
	defer cur.Close(ctx)
	var results []*models.NodeStock
	for cur.Next(ctx) {
		var doc nodeStockDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		results = append(results, docToStock(&doc))
	}
	return results, cur.Err()
}

// ── NodeItemConfig ────────────────────────────────────────────────────────────

type nodeItemConfigDoc struct {
	ItemID                string                  `bson:"item_id"`
	NodeID                string                  `bson:"node_id"`
	SourcingStrategy      models.SourcingStrategy `bson:"sourcing_strategy"`
	ProviderNodeID        *string                 `bson:"provider_node_id,omitempty"`
	SupplierID            *string                 `bson:"supplier_id,omitempty"`
	ReorderPoint          float64                 `bson:"reorder_point"`
	SafetyStock           float64                 `bson:"safety_stock"`
	SupplierLeadTimeDays  int                     `bson:"supplier_lead_time_days"`
	CProd                 *float64                `bson:"c_prod,omitempty"`
	CTransfer             *float64                `bson:"c_transfer,omitempty"`
	ConsumptionWindowDays int                     `bson:"consumption_window_days"`
	UpdatedAt             time.Time               `bson:"updated_at"`
}

func configToDoc(c *models.NodeItemConfig) *nodeItemConfigDoc {
	return &nodeItemConfigDoc{
		ItemID:                c.ItemID,
		NodeID:                c.NodeID,
		SourcingStrategy:      c.SourcingStrategy,
		ProviderNodeID:        c.ProviderNodeID,
		SupplierID:            c.SupplierID,
		ReorderPoint:          c.ReorderPoint,
		SafetyStock:           c.SafetyStock,
		SupplierLeadTimeDays:  c.SupplierLeadTimeDays,
		CProd:                 c.CProd,
		CTransfer:             c.CTransfer,
		ConsumptionWindowDays: c.ConsumptionWindowDays,
		UpdatedAt:             c.UpdatedAt,
	}
}

func docToConfig(d *nodeItemConfigDoc) *models.NodeItemConfig {
	return &models.NodeItemConfig{
		ItemID:                d.ItemID,
		NodeID:                d.NodeID,
		SourcingStrategy:      d.SourcingStrategy,
		ProviderNodeID:        d.ProviderNodeID,
		SupplierID:            d.SupplierID,
		ReorderPoint:          d.ReorderPoint,
		SafetyStock:           d.SafetyStock,
		SupplierLeadTimeDays:  d.SupplierLeadTimeDays,
		CProd:                 d.CProd,
		CTransfer:             d.CTransfer,
		ConsumptionWindowDays: d.ConsumptionWindowDays,
		UpdatedAt:             d.UpdatedAt,
	}
}

type nodeItemConfigRepository struct {
	col *mongo.Collection
}

func NewNodeItemConfigRepository(client *Client, dbName string) services.NodeItemConfigRepository {
	col := client.DB(dbName).Collection(collNodeItemConfig)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "node_id", Value: 1}, {Key: "item_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &nodeItemConfigRepository{col: col}
}

func (r *nodeItemConfigRepository) Get(ctx context.Context, nodeID, itemID string) (*models.NodeItemConfig, error) {
	var doc nodeItemConfigDoc
	err := r.col.FindOne(ctx, bson.M{"node_id": nodeID, "item_id": itemID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("nodeItemConfigRepository.Get: %w", err)
	}
	return docToConfig(&doc), nil
}

func (r *nodeItemConfigRepository) Upsert(ctx context.Context, cfg *models.NodeItemConfig) error {
	filter := bson.M{"node_id": cfg.NodeID, "item_id": cfg.ItemID}
	update := bson.M{"$set": configToDoc(cfg)}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("nodeItemConfigRepository.Upsert: %w", err)
	}
	return nil
}

func (r *nodeItemConfigRepository) ListByNode(ctx context.Context, nodeID string) ([]*models.NodeItemConfig, error) {
	cur, err := r.col.Find(ctx, bson.M{"node_id": nodeID})
	if err != nil {
		return nil, fmt.Errorf("nodeItemConfigRepository.ListByNode: %w", err)
	}
	defer cur.Close(ctx)
	var results []*models.NodeItemConfig
	for cur.Next(ctx) {
		var doc nodeItemConfigDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		results = append(results, docToConfig(&doc))
	}
	return results, cur.Err()
}
