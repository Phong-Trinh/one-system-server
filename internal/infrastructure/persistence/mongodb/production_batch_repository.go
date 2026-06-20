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

const collBatches = "production_batches"

type batchDoc struct {
	ID                  string             `bson:"_id"`
	POID                string             `bson:"po_id"`
	SOPStepID           string             `bson:"sop_step_id"`
	NodeID              string             `bson:"node_id"`
	MachineID           string             `bson:"machine_id"`
	ReferenceOrderID    string             `bson:"reference_order_id,omitempty"`
	ItemID              string             `bson:"item_id"`
	Qty                 float64            `bson:"qty"`
	SlotsUsed           float64            `bson:"slots_used"`
	Status              models.BatchStatus `bson:"status"`
	AllocatedAt         *time.Time         `bson:"allocated_at,omitempty"`
	StartedAt           *time.Time         `bson:"started_at,omitempty"`
	EstimatedCompletion *time.Time         `bson:"estimated_completion,omitempty"`
	ActualEnd           *time.Time         `bson:"actual_end,omitempty"`
}

func batchToDoc(b *models.ProductionBatch) *batchDoc {
	return &batchDoc{
		ID:                  b.ID,
		POID:                b.POID,
		SOPStepID:           b.SOPStepID,
		NodeID:              b.NodeID,
		MachineID:           b.MachineID,
		ReferenceOrderID:    b.ReferenceOrderID,
		ItemID:              b.ItemID,
		Qty:                 b.Qty,
		SlotsUsed:           b.SlotsUsed,
		Status:              b.Status,
		AllocatedAt:         b.AllocatedAt,
		StartedAt:           b.StartedAt,
		EstimatedCompletion: b.EstimatedCompletion,
		ActualEnd:           b.ActualEnd,
	}
}

func docToBatch(d *batchDoc) *models.ProductionBatch {
	return &models.ProductionBatch{
		ID:                  d.ID,
		POID:                d.POID,
		SOPStepID:           d.SOPStepID,
		NodeID:              d.NodeID,
		MachineID:           d.MachineID,
		ReferenceOrderID:    d.ReferenceOrderID,
		ItemID:              d.ItemID,
		Qty:                 d.Qty,
		SlotsUsed:           d.SlotsUsed,
		Status:              d.Status,
		AllocatedAt:         d.AllocatedAt,
		StartedAt:           d.StartedAt,
		EstimatedCompletion: d.EstimatedCompletion,
		ActualEnd:           d.ActualEnd,
	}
}

type batchRepository struct {
	col *mongo.Collection
}

func NewProductionBatchRepository(client *Client, dbName string) services.ProductionBatchRepository {
	col := client.DB(dbName).Collection(collBatches)
	_, _ = col.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "node_id", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "machine_id", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "po_id", Value: 1}}},
	})
	return &batchRepository{col: col}
}

func (r *batchRepository) Create(ctx context.Context, b *models.ProductionBatch) error {
	_, err := r.col.InsertOne(ctx, batchToDoc(b))
	if err != nil {
		return fmt.Errorf("batchRepository.Create: %w", err)
	}
	return nil
}

func (r *batchRepository) FindByID(ctx context.Context, id string) (*models.ProductionBatch, error) {
	var doc batchDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("batchRepository.FindByID: %w", err)
	}
	return docToBatch(&doc), nil
}

func (r *batchRepository) FindByNode(ctx context.Context, nodeID string, statuses []models.BatchStatus) ([]*models.ProductionBatch, error) {
	filter := bson.M{"node_id": nodeID}
	if len(statuses) > 0 {
		filter["status"] = bson.M{"$in": statuses}
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("batchRepository.FindByNode: %w", err)
	}
	defer cur.Close(ctx)
	return decodeBatches(ctx, cur)
}

func (r *batchRepository) FindByMachine(ctx context.Context, machineID string, statuses []models.BatchStatus) ([]*models.ProductionBatch, error) {
	filter := bson.M{"machine_id": machineID}
	if len(statuses) > 0 {
		filter["status"] = bson.M{"$in": statuses}
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("batchRepository.FindByMachine: %w", err)
	}
	defer cur.Close(ctx)
	return decodeBatches(ctx, cur)
}

func (r *batchRepository) UpdateStatus(ctx context.Context, id string, status models.BatchStatus) error {
	update := bson.M{"$set": bson.M{"status": status}}
	now := time.Now()
	if status == models.BatchInProgress {
		update["$set"].(bson.M)["started_at"] = now
	} else if status == models.BatchCompleted || status == models.BatchFailed {
		update["$set"].(bson.M)["actual_end"] = now
	}

	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("batchRepository.UpdateStatus: %w", err)
	}
	return nil
}

func (r *batchRepository) Update(ctx context.Context, b *models.ProductionBatch) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": b.ID}, batchToDoc(b))
	if err != nil {
		return fmt.Errorf("batchRepository.Update: %w", err)
	}
	return nil
}

func (r *batchRepository) Delete(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("batchRepository.Delete: %w", err)
	}
	return nil
}

func decodeBatches(ctx context.Context, cur *mongo.Cursor) ([]*models.ProductionBatch, error) {
	var result []*models.ProductionBatch
	for cur.Next(ctx) {
		var doc batchDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, docToBatch(&doc))
	}
	return result, cur.Err()
}
