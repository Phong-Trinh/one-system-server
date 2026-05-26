package mongodb

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

const collEquipmentTypes = "equipment_types"

// ── BSON document ─────────────────────────────────────────────────────────────

type equipmentTypeDoc struct {
	ID           string `bson:"_id"`
	Name         string `bson:"name"`
	CapacityUnit string `bson:"capacity_unit"`
}

func equipmentTypeToDoc(et *models.EquipmentType) *equipmentTypeDoc {
	return &equipmentTypeDoc{
		ID:           et.ID,
		Name:         et.Name,
		CapacityUnit: et.CapacityUnit,
	}
}

func docToEquipmentType(d *equipmentTypeDoc) *models.EquipmentType {
	return &models.EquipmentType{
		ID:           d.ID,
		Name:         d.Name,
		CapacityUnit: d.CapacityUnit,
	}
}

// ── Repository ────────────────────────────────────────────────────────────────

type equipmentTypeRepository struct {
	col *mongo.Collection
}

// NewEquipmentTypeRepository returns a services.EquipmentTypeRepository backed by MongoDB.
// Previously named NewStationTypeRepository — renamed to match the canonical EquipmentType model.
func NewEquipmentTypeRepository(client *Client, dbName string) services.EquipmentTypeRepository {
	col := client.DB(dbName).Collection(collEquipmentTypes)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &equipmentTypeRepository{col: col}
}

// NewStationTypeRepository is a compat alias for NewEquipmentTypeRepository.
func NewStationTypeRepository(client *Client, dbName string) services.EquipmentTypeRepository {
	return NewEquipmentTypeRepository(client, dbName)
}

func (r *equipmentTypeRepository) Create(ctx context.Context, et *models.EquipmentType) error {
	_, err := r.col.InsertOne(ctx, equipmentTypeToDoc(et))
	if err != nil {
		return fmt.Errorf("equipmentTypeRepository.Create: %w", err)
	}
	return nil
}

func (r *equipmentTypeRepository) FindByID(ctx context.Context, id string) (*models.EquipmentType, error) {
	var doc equipmentTypeDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("equipmentTypeRepository.FindByID: %w", err)
	}
	return docToEquipmentType(&doc), nil
}

func (r *equipmentTypeRepository) FindAll(ctx context.Context) ([]*models.EquipmentType, error) {
	cur, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("equipmentTypeRepository.FindAll: %w", err)
	}
	defer cur.Close(ctx)

	var result []*models.EquipmentType
	for cur.Next(ctx) {
		var doc equipmentTypeDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, docToEquipmentType(&doc))
	}
	return result, cur.Err()
}

func (r *equipmentTypeRepository) Update(ctx context.Context, et *models.EquipmentType) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": et.ID}, equipmentTypeToDoc(et))
	if err != nil {
		return fmt.Errorf("equipmentTypeRepository.Update: %w", err)
	}
	return nil
}

func (r *equipmentTypeRepository) Delete(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("equipmentTypeRepository.Delete: %w", err)
	}
	return nil
}
