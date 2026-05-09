package mongodb

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

const collStaff = "staff"

// ── BSON document ─────────────────────────────────────────────────────────────

type staffDoc struct {
	ID       string  `bson:"_id"`
	NodeID   string  `bson:"node_id"`
	Name     string  `bson:"name"`
	WageRate float64 `bson:"wage_rate"`
}

func staffToDoc(s *models.Staff) *staffDoc {
	return &staffDoc{ID: s.ID, NodeID: s.NodeID, Name: s.Name, WageRate: s.WageRate}
}

func docToStaff(d *staffDoc) *models.Staff {
	return &models.Staff{ID: d.ID, NodeID: d.NodeID, Name: d.Name, WageRate: d.WageRate}
}

// ── Repository ────────────────────────────────────────────────────────────────

type staffRepository struct {
	col *mongo.Collection
}

// NewStaffRepository returns a services.StaffRepository backed by MongoDB.
func NewStaffRepository(client *Client, dbName string) services.StaffRepository {
	col := client.DB(dbName).Collection(collStaff)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "node_id", Value: 1}},
	})
	return &staffRepository{col: col}
}

func (r *staffRepository) Create(ctx context.Context, s *models.Staff) error {
	_, err := r.col.InsertOne(ctx, staffToDoc(s))
	if err != nil {
		return fmt.Errorf("staffRepository.Create: %w", err)
	}
	return nil
}

func (r *staffRepository) FindByID(ctx context.Context, id string) (*models.Staff, error) {
	var doc staffDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("staffRepository.FindByID: %w", err)
	}
	return docToStaff(&doc), nil
}

func (r *staffRepository) FindByNodeID(ctx context.Context, nodeID string) ([]*models.Staff, error) {
	cur, err := r.col.Find(ctx, bson.M{"node_id": nodeID})
	if err != nil {
		return nil, fmt.Errorf("staffRepository.FindByNodeID: %w", err)
	}
	defer cur.Close(ctx)

	var result []*models.Staff
	for cur.Next(ctx) {
		var doc staffDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, docToStaff(&doc))
	}
	return result, cur.Err()
}

func (r *staffRepository) Update(ctx context.Context, s *models.Staff) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": s.ID}, staffToDoc(s))
	if err != nil {
		return fmt.Errorf("staffRepository.Update: %w", err)
	}
	return nil
}

func (r *staffRepository) Delete(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("staffRepository.Delete: %w", err)
	}
	return nil
}
