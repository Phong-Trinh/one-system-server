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

const collAssets = "assets"

type assetDoc struct {
	ID               string                    `bson:"_id"`
	OrgID            string                    `bson:"org_id"`
	EquipmentTypeID  string                    `bson:"equipment_type_id"`
	NodeID           string                    `bson:"node_id"`
	LinkedPRID       string                    `bson:"linked_pr_id"`
	LinkedPurOID     string                    `bson:"linked_puro_id"`
	LinkedGRID       string                    `bson:"linked_gr_id"`
	LinkedMachineID  *string                   `bson:"linked_machine_id,omitempty"`
	AcquisitionCost  float64                   `bson:"acquisition_cost"`
	AcquisitionDate  time.Time                 `bson:"acquisition_date"`
	Status           models.AssetStatus        `bson:"status"`
	Depreciation     models.DepreciationMethod `bson:"depreciation_method"`
	UsefulLifeYears  int                       `bson:"useful_life_years"`
	CurrentBookValue float64                   `bson:"current_book_value"`
	CreatedAt        time.Time                 `bson:"created_at"`
	UpdatedAt        time.Time                 `bson:"updated_at"`
}

func assetToDoc(a *models.Asset) *assetDoc {
	return &assetDoc{
		ID:               a.ID,
		OrgID:            a.OrgID,
		EquipmentTypeID:  a.EquipmentTypeID,
		NodeID:           a.NodeID,
		LinkedPRID:       a.LinkedPRID,
		LinkedPurOID:     a.LinkedPurOID,
		LinkedGRID:       a.LinkedGRID,
		LinkedMachineID:  a.LinkedMachineID,
		AcquisitionCost:  a.AcquisitionCost,
		AcquisitionDate:  a.AcquisitionDate,
		Status:           a.Status,
		Depreciation:     a.Depreciation,
		UsefulLifeYears:  a.UsefulLifeYears,
		CurrentBookValue: a.CurrentBookValue,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}

func docToAsset(d *assetDoc) *models.Asset {
	return &models.Asset{
		ID:               d.ID,
		OrgID:            d.OrgID,
		EquipmentTypeID:  d.EquipmentTypeID,
		NodeID:           d.NodeID,
		LinkedPRID:       d.LinkedPRID,
		LinkedPurOID:     d.LinkedPurOID,
		LinkedGRID:       d.LinkedGRID,
		LinkedMachineID:  d.LinkedMachineID,
		AcquisitionCost:  d.AcquisitionCost,
		AcquisitionDate:  d.AcquisitionDate,
		Status:           d.Status,
		Depreciation:     d.Depreciation,
		UsefulLifeYears:  d.UsefulLifeYears,
		CurrentBookValue: d.CurrentBookValue,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
}

type assetRepository struct {
	col *mongo.Collection
}

func NewAssetRepository(client *Client, dbName string) services.AssetRepository {
	col := client.DB(dbName).Collection(collAssets)
	_, _ = col.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "node_id", Value: 1}}},
		{Keys: bson.D{{Key: "linked_puro_id", Value: 1}}},
	})
	return &assetRepository{col: col}
}

func (r *assetRepository) Create(ctx context.Context, asset *models.Asset) error {
	_, err := r.col.InsertOne(ctx, assetToDoc(asset))
	if err != nil {
		return fmt.Errorf("assetRepository.Create: %w", err)
	}
	return nil
}

func (r *assetRepository) FindByID(ctx context.Context, id string) (*models.Asset, error) {
	var doc assetDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("assetRepository.FindByID: %w", err)
	}
	return docToAsset(&doc), nil
}

func (r *assetRepository) FindByNode(ctx context.Context, nodeID string) ([]*models.Asset, error) {
	cur, err := r.col.Find(ctx, bson.M{"node_id": nodeID})
	if err != nil {
		return nil, fmt.Errorf("assetRepository.FindByNode: %w", err)
	}
	defer cur.Close(ctx)

	var assets []*models.Asset
	for cur.Next(ctx) {
		var doc assetDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		assets = append(assets, docToAsset(&doc))
	}
	return assets, cur.Err()
}

func (r *assetRepository) FindByPurO(ctx context.Context, purOID string) (*models.Asset, error) {
	var doc assetDoc
	err := r.col.FindOne(ctx, bson.M{"linked_puro_id": purOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("assetRepository.FindByPurO: %w", err)
	}
	return docToAsset(&doc), nil
}

func (r *assetRepository) Update(ctx context.Context, asset *models.Asset) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": asset.ID}, assetToDoc(asset))
	if err != nil {
		return fmt.Errorf("assetRepository.Update: %w", err)
	}
	return nil
}
