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

const collSuppliers = "suppliers"

// ── BSON document ─────────────────────────────────────────────────────────────

type supplierDoc struct {
	ID          string `bson:"_id"`
	OrgID       string `bson:"org_id"`
	Name        string `bson:"name"`
	ContactInfo string `bson:"contact_info"`
	Address     string `bson:"address"`
}

func supplierToDoc(s *models.Supplier) *supplierDoc {
	return &supplierDoc{
		ID:          s.ID,
		OrgID:       s.OrgID,
		Name:        s.Name,
		ContactInfo: s.ContactInfo,
		Address:     s.Address,
	}
}

func docToSupplier(d *supplierDoc) *models.Supplier {
	return &models.Supplier{
		ID:          d.ID,
		OrgID:       d.OrgID,
		Name:        d.Name,
		ContactInfo: d.ContactInfo,
		Address:     d.Address,
	}
}

// ── Repository ────────────────────────────────────────────────────────────────

type supplierRepository struct {
	col *mongo.Collection
}

func NewSupplierRepository(client *Client, dbName string) services.SupplierRepository {
	col := client.DB(dbName).Collection(collSuppliers)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "org_id", Value: 1}},
	})
	return &supplierRepository{col: col}
}

func (r *supplierRepository) Create(ctx context.Context, supplier *models.Supplier) error {
	_, err := r.col.InsertOne(ctx, supplierToDoc(supplier))
	if err != nil {
		return fmt.Errorf("supplierRepository.Create: %w", err)
	}
	return nil
}

func (r *supplierRepository) FindByID(ctx context.Context, id string) (*models.Supplier, error) {
	var doc supplierDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("supplierRepository.FindByID: %w", err)
	}
	return docToSupplier(&doc), nil
}

func (r *supplierRepository) FindByOrg(ctx context.Context, orgID string) ([]*models.Supplier, error) {
	cur, err := r.col.Find(ctx, bson.M{"org_id": orgID})
	if err != nil {
		return nil, fmt.Errorf("supplierRepository.FindByOrg: %w", err)
	}
	defer cur.Close(ctx)

	var suppliers []*models.Supplier
	for cur.Next(ctx) {
		var doc supplierDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		suppliers = append(suppliers, docToSupplier(&doc))
	}
	return suppliers, cur.Err()
}
