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

const collNodes = "nodes"

// ── BSON document ─────────────────────────────────────────────────────────────

type nodeDoc struct {
	ID        string          `bson:"_id"`
	OrgID     string          `bson:"org_id"`
	Type      models.NodeType `bson:"type"`
	Name      string          `bson:"name"`
	Address   string          `bson:"address"`
	CreatedAt time.Time       `bson:"created_at"`
	UpdatedAt time.Time       `bson:"updated_at"`
}

func nodeToDoc(n *models.Node) *nodeDoc {
	return &nodeDoc{
		ID:        n.ID,
		OrgID:     n.OrgID,
		Type:      n.Type,
		Name:      n.Name,
		Address:   n.Address,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}

func docToNode(d *nodeDoc) *models.Node {
	return &models.Node{
		ID:        d.ID,
		OrgID:     d.OrgID,
		Type:      d.Type,
		Name:      d.Name,
		Address:   d.Address,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// ── Repository ────────────────────────────────────────────────────────────────

type nodeRepository struct {
	col *mongo.Collection
}

// NewNodeRepository returns a services.NodeRepository backed by MongoDB.
func NewNodeRepository(client *Client, dbName string) services.NodeRepository {
	col := client.DB(dbName).Collection(collNodes)
	// Compound index: fast query by org
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "org_id", Value: 1}, {Key: "type", Value: 1}},
	})
	// Unique name within org
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "org_id", Value: 1}, {Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &nodeRepository{col: col}
}

func (r *nodeRepository) Create(ctx context.Context, node *models.Node) error {
	_, err := r.col.InsertOne(ctx, nodeToDoc(node))
	if err != nil {
		return fmt.Errorf("nodeRepository.Create: %w", err)
	}
	return nil
}

func (r *nodeRepository) FindByID(ctx context.Context, id string) (*models.Node, error) {
	var doc nodeDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("nodeRepository.FindByID: %w", err)
	}
	return docToNode(&doc), nil
}

func (r *nodeRepository) FindByOrgID(ctx context.Context, orgID string) ([]*models.Node, error) {
	cur, err := r.col.Find(ctx, bson.M{"org_id": orgID})
	if err != nil {
		return nil, fmt.Errorf("nodeRepository.FindByOrgID: %w", err)
	}
	defer cur.Close(ctx)

	var nodes []*models.Node
	for cur.Next(ctx) {
		var doc nodeDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		nodes = append(nodes, docToNode(&doc))
	}
	return nodes, cur.Err()
}

func (r *nodeRepository) FindAll(ctx context.Context) ([]*models.Node, error) {
	cur, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("nodeRepository.FindAll: %w", err)
	}
	defer cur.Close(ctx)

	var nodes []*models.Node
	for cur.Next(ctx) {
		var doc nodeDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		nodes = append(nodes, docToNode(&doc))
	}
	return nodes, cur.Err()
}

func (r *nodeRepository) Update(ctx context.Context, node *models.Node) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": node.ID}, nodeToDoc(node))
	if err != nil {
		return fmt.Errorf("nodeRepository.Update: %w", err)
	}
	return nil
}

func (r *nodeRepository) Delete(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("nodeRepository.Delete: %w", err)
	}
	return nil
}
