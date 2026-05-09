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

const collStationTypes = "station_types"

// ── BSON document ─────────────────────────────────────────────────────────────

type stationTypeDoc struct {
	ID           string `bson:"_id"`
	Name         string `bson:"name"`
	CapacityUnit string `bson:"capacity_unit"`
}

func stationTypeToDoc(s *models.StationType) *stationTypeDoc {
	return &stationTypeDoc{ID: s.ID, Name: s.Name, CapacityUnit: s.CapacityUnit}
}

func docToStationType(d *stationTypeDoc) *models.StationType {
	return &models.StationType{ID: d.ID, Name: d.Name, CapacityUnit: d.CapacityUnit}
}

// ── Repository ────────────────────────────────────────────────────────────────

type stationTypeRepository struct {
	col *mongo.Collection
}

// NewStationTypeRepository returns a services.StationTypeRepository backed by MongoDB.
func NewStationTypeRepository(client *Client, dbName string) services.StationTypeRepository {
	col := client.DB(dbName).Collection(collStationTypes)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &stationTypeRepository{col: col}
}

func (r *stationTypeRepository) Create(ctx context.Context, st *models.StationType) error {
	_, err := r.col.InsertOne(ctx, stationTypeToDoc(st))
	if err != nil {
		return fmt.Errorf("stationTypeRepository.Create: %w", err)
	}
	return nil
}

func (r *stationTypeRepository) FindByID(ctx context.Context, id string) (*models.StationType, error) {
	var doc stationTypeDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stationTypeRepository.FindByID: %w", err)
	}
	return docToStationType(&doc), nil
}

func (r *stationTypeRepository) FindAll(ctx context.Context) ([]*models.StationType, error) {
	cur, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("stationTypeRepository.FindAll: %w", err)
	}
	defer cur.Close(ctx)

	var result []*models.StationType
	for cur.Next(ctx) {
		var doc stationTypeDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, docToStationType(&doc))
	}
	return result, cur.Err()
}

func (r *stationTypeRepository) Update(ctx context.Context, st *models.StationType) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": st.ID}, stationTypeToDoc(st))
	if err != nil {
		return fmt.Errorf("stationTypeRepository.Update: %w", err)
	}
	return nil
}

func (r *stationTypeRepository) Delete(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("stationTypeRepository.Delete: %w", err)
	}
	return nil
}
