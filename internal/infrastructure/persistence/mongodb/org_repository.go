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

const collOrgs = "organizations"

// ── BSON document ─────────────────────────────────────────────────────────────

type orgDoc struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func orgToDoc(o *models.Organization) *orgDoc {
	return &orgDoc{
		ID:        o.ID,
		Name:      o.Name,
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}
}

func docToOrg(d *orgDoc) *models.Organization {
	return &models.Organization{
		ID:        d.ID,
		Name:      d.Name,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// ── Repository ────────────────────────────────────────────────────────────────

type orgRepository struct {
	col *mongo.Collection
}

// NewOrgRepository returns a services.OrgRepository backed by MongoDB.
func NewOrgRepository(client *Client, dbName string) services.OrgRepository {
	col := client.DB(dbName).Collection(collOrgs)
	// Ensure unique index on name
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &orgRepository{col: col}
}

func (r *orgRepository) Create(ctx context.Context, org *models.Organization) error {
	_, err := r.col.InsertOne(ctx, orgToDoc(org))
	if err != nil {
		return fmt.Errorf("orgRepository.Create: %w", err)
	}
	return nil
}

func (r *orgRepository) FindByID(ctx context.Context, id string) (*models.Organization, error) {
	var doc orgDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("orgRepository.FindByID: %w", err)
	}
	return docToOrg(&doc), nil
}

func (r *orgRepository) FindAll(ctx context.Context) ([]*models.Organization, error) {
	cur, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("orgRepository.FindAll: %w", err)
	}
	defer cur.Close(ctx)

	var orgs []*models.Organization
	for cur.Next(ctx) {
		var doc orgDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		orgs = append(orgs, docToOrg(&doc))
	}
	return orgs, cur.Err()
}

func (r *orgRepository) Update(ctx context.Context, org *models.Organization) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": org.ID}, orgToDoc(org))
	if err != nil {
		return fmt.Errorf("orgRepository.Update: %w", err)
	}
	return nil
}

func (r *orgRepository) Delete(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("orgRepository.Delete: %w", err)
	}
	return nil
}
