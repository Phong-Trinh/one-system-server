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

const collMachines = "machines"

// ── BSON document ─────────────────────────────────────────────────────────────

type machineDoc struct {
	ID             string               `bson:"_id"`
	StationTypeID  string               `bson:"station_type_id"`
	NodeID         string               `bson:"node_id"`
	MaxSlots       int                  `bson:"max_slots"`
	Status         models.MachineStatus `bson:"status"`
	CurrentBatchID *string              `bson:"current_batch_id,omitempty"`
}

func machineToDoc(m *models.Machine) *machineDoc {
	return &machineDoc{
		ID:             m.ID,
		StationTypeID:  m.StationTypeID,
		NodeID:         m.NodeID,
		MaxSlots:       m.MaxSlots,
		Status:         m.Status,
		CurrentBatchID: m.CurrentBatchID,
	}
}

func docToMachine(d *machineDoc) *models.Machine {
	return &models.Machine{
		ID:             d.ID,
		StationTypeID:  d.StationTypeID,
		NodeID:         d.NodeID,
		MaxSlots:       d.MaxSlots,
		Status:         d.Status,
		CurrentBatchID: d.CurrentBatchID,
	}
}

// ── Repository ────────────────────────────────────────────────────────────────

type machineRepository struct {
	col *mongo.Collection
}

// NewMachineRepository returns a services.MachineRepository backed by MongoDB.
func NewMachineRepository(client *Client, dbName string) services.MachineRepository {
	col := client.DB(dbName).Collection(collMachines)
	// Compound index: used by bin-packing allocation engine
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{
			{Key: "node_id", Value: 1},
			{Key: "station_type_id", Value: 1},
			{Key: "status", Value: 1},
		},
	})
	return &machineRepository{col: col}
}

func (r *machineRepository) Create(ctx context.Context, m *models.Machine) error {
	_, err := r.col.InsertOne(ctx, machineToDoc(m))
	if err != nil {
		return fmt.Errorf("machineRepository.Create: %w", err)
	}
	return nil
}

func (r *machineRepository) FindByID(ctx context.Context, id string) (*models.Machine, error) {
	var doc machineDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("machineRepository.FindByID: %w", err)
	}
	return docToMachine(&doc), nil
}

func (r *machineRepository) FindByNodeID(ctx context.Context, nodeID string) ([]*models.Machine, error) {
	cur, err := r.col.Find(ctx, bson.M{"node_id": nodeID})
	if err != nil {
		return nil, fmt.Errorf("machineRepository.FindByNodeID: %w", err)
	}
	defer cur.Close(ctx)
	return decodeMachines(ctx, cur)
}

// FindIdleByStationType supports the bin-packing allocation engine (Layer 5).
func (r *machineRepository) FindIdleByStationType(ctx context.Context, nodeID, stationTypeID string) ([]*models.Machine, error) {
	filter := bson.M{
		"node_id":         nodeID,
		"station_type_id": stationTypeID,
		"status":          models.MachineIdle,
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("machineRepository.FindIdleByStationType: %w", err)
	}
	defer cur.Close(ctx)
	return decodeMachines(ctx, cur)
}

func (r *machineRepository) UpdateStatus(ctx context.Context, id string, status models.MachineStatus, batchID *string) error {
	update := bson.M{
		"$set": bson.M{
			"status":           status,
			"current_batch_id": batchID,
		},
	}
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("machineRepository.UpdateStatus: %w", err)
	}
	return nil
}

func (r *machineRepository) Update(ctx context.Context, m *models.Machine) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": m.ID}, machineToDoc(m))
	if err != nil {
		return fmt.Errorf("machineRepository.Update: %w", err)
	}
	return nil
}

func (r *machineRepository) Delete(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("machineRepository.Delete: %w", err)
	}
	return nil
}

func decodeMachines(ctx context.Context, cur *mongo.Cursor) ([]*models.Machine, error) {
	var machines []*models.Machine
	for cur.Next(ctx) {
		var doc machineDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		machines = append(machines, docToMachine(&doc))
	}
	return machines, cur.Err()
}

func (r *machineRepository) FindAll(ctx context.Context) ([]*models.Machine, error) {
	cur, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("machineRepository.FindAll: %w", err)
	}
	defer cur.Close(ctx)

	var machines []*models.Machine
	for cur.Next(ctx) {
		var doc machineDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		machines = append(machines, docToMachine(&doc))
	}
	return machines, cur.Err()
}
