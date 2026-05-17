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

const (
	collPOs              = "production_orders"
	collBOMSnapshots     = "bom_snapshots"
	collStaffAssignments = "po_staff_assignments"
)

// ── BSON documents ────────────────────────────────────────────────────────────

type poDoc struct {
	ID             string          `bson:"_id"`
	ItemID         string          `bson:"item_id"`
	BOMID          string          `bson:"bom_id"`
	SOPID          string          `bson:"sop_id"`
	NodeID         string          `bson:"node_id"`
	TargetQty      float64         `bson:"target_qty"`
	YieldRate      float64         `bson:"yield_rate"`
	PlannedInput   float64         `bson:"planned_input"`
	ActualOutput   float64         `bson:"actual_output"`
	Status         models.POStatus `bson:"status"`
	DeadlineAt     *time.Time      `bson:"deadline_at,omitempty"`
	ScheduledStart time.Time       `bson:"scheduled_start"`
	ScheduledEnd   time.Time       `bson:"scheduled_end"`
	CreatedAt      time.Time       `bson:"created_at"`
	UpdatedAt      time.Time       `bson:"updated_at"`
}

type bomSnapshotDoc struct {
	POID             string `bson:"_id"` // POID as primary key (1:1)
	LockedBOMVersion int    `bson:"locked_bom_version"`
	SnapshotData     string `bson:"snapshot_data"`
}

type poStaffAssignmentDoc struct {
	POID    string  `bson:"po_id"`
	StaffID string  `bson:"staff_id"`
	Hours   float64 `bson:"hours"`
}

func poToDoc(po *models.ProductionOrder) *poDoc {
	return &poDoc{
		ID:             po.ID,
		ItemID:         po.ItemID,
		BOMID:          po.BOMID,
		SOPID:          po.SOPID,
		NodeID:         po.NodeID,
		TargetQty:      po.TargetQty,
		YieldRate:      po.YieldRate,
		PlannedInput:   po.PlannedInput,
		ActualOutput:   po.ActualOutput,
		Status:         po.Status,
		DeadlineAt:     po.DeadlineAt,
		ScheduledStart: po.ScheduledStart,
		ScheduledEnd:   po.ScheduledEnd,
		CreatedAt:      po.CreatedAt,
		UpdatedAt:      po.UpdatedAt,
	}
}

func docToPO(d *poDoc) *models.ProductionOrder {
	return &models.ProductionOrder{
		ID:             d.ID,
		ItemID:         d.ItemID,
		BOMID:          d.BOMID,
		SOPID:          d.SOPID,
		NodeID:         d.NodeID,
		TargetQty:      d.TargetQty,
		YieldRate:      d.YieldRate,
		PlannedInput:   d.PlannedInput,
		ActualOutput:   d.ActualOutput,
		Status:         d.Status,
		DeadlineAt:     d.DeadlineAt,
		ScheduledStart: d.ScheduledStart,
		ScheduledEnd:   d.ScheduledEnd,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

// ── Repository ────────────────────────────────────────────────────────────────

type productionOrderRepository struct {
	pos         *mongo.Collection
	snapshots   *mongo.Collection
	assignments *mongo.Collection
}

// NewProductionOrderRepository returns a services.ProductionOrderRepository backed by MongoDB.
func NewProductionOrderRepository(client *Client, dbName string) services.ProductionOrderRepository {
	pos := client.DB(dbName).Collection(collPOs)
	snapshots := client.DB(dbName).Collection(collBOMSnapshots)
	assignments := client.DB(dbName).Collection(collStaffAssignments)

	// Index: list POs by node or status
	_, _ = pos.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "node_id", Value: 1}}},
		{Keys: bson.D{{Key: "status", Value: 1}}},
	})

	// Index: assignments by PO
	_, _ = assignments.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "po_id", Value: 1}},
	})

	return &productionOrderRepository{
		pos:         pos,
		snapshots:   snapshots,
		assignments: assignments,
	}
}

func (r *productionOrderRepository) Create(ctx context.Context, po *models.ProductionOrder) error {
	_, err := r.pos.InsertOne(ctx, poToDoc(po))
	if err != nil {
		return fmt.Errorf("poRepository.Create: %w", err)
	}
	return nil
}

func (r *productionOrderRepository) FindByID(ctx context.Context, id string) (*models.ProductionOrder, error) {
	var doc poDoc
	err := r.pos.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("poRepository.FindByID: %w", err)
	}
	return docToPO(&doc), nil
}

func (r *productionOrderRepository) FindByNode(ctx context.Context, nodeID string) ([]*models.ProductionOrder, error) {
	cur, err := r.pos.Find(ctx, bson.M{"node_id": nodeID})
	if err != nil {
		return nil, fmt.Errorf("poRepository.FindByNode: %w", err)
	}
	defer cur.Close(ctx)
	return decodePOs(ctx, cur)
}

func (r *productionOrderRepository) FindByStatus(ctx context.Context, status models.POStatus) ([]*models.ProductionOrder, error) {
	cur, err := r.pos.Find(ctx, bson.M{"status": status})
	if err != nil {
		return nil, fmt.Errorf("poRepository.FindByStatus: %w", err)
	}
	defer cur.Close(ctx)
	return decodePOs(ctx, cur)
}

func (r *productionOrderRepository) FindAll(ctx context.Context) ([]*models.ProductionOrder, error) {
	cur, err := r.pos.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("poRepository.FindAll: %w", err)
	}
	defer cur.Close(ctx)
	return decodePOs(ctx, cur)
}

func (r *productionOrderRepository) UpdateStatus(ctx context.Context, id string, status models.POStatus, actualOutput *float64) error {
	update := bson.M{"$set": bson.M{
		"status":     status,
		"updated_at": time.Now(),
	}}
	if actualOutput != nil {
		update["$set"].(bson.M)["actual_output"] = *actualOutput
	}
	_, err := r.pos.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("poRepository.UpdateStatus: %w", err)
	}
	return nil
}

func (r *productionOrderRepository) Update(ctx context.Context, po *models.ProductionOrder) error {
	po.UpdatedAt = time.Now()
	_, err := r.pos.ReplaceOne(ctx, bson.M{"_id": po.ID}, poToDoc(po))
	if err != nil {
		return fmt.Errorf("poRepository.Update: %w", err)
	}
	return nil
}

func (r *productionOrderRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pos.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("poRepository.Delete: %w", err)
	}
	return nil
}

// ── BOM Snapshot ──────────────────────────────────────────────────────────────

func (r *productionOrderRepository) SaveSnapshot(ctx context.Context, snap *models.BOMSnapshot) error {
	doc := bson.M{
		"_id":                snap.POID,
		"locked_bom_version": snap.LockedBOMVersion,
		"snapshot_data":      snap.SnapshotData,
	}
	_, err := r.snapshots.ReplaceOne(ctx, bson.M{"_id": snap.POID}, doc, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("poRepository.SaveSnapshot: %w", err)
	}
	return nil
}

func (r *productionOrderRepository) GetSnapshot(ctx context.Context, poID string) (*models.BOMSnapshot, error) {
	var doc bomSnapshotDoc
	err := r.snapshots.FindOne(ctx, bson.M{"_id": poID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("poRepository.GetSnapshot: %w", err)
	}
	return &models.BOMSnapshot{
		POID:             doc.POID,
		LockedBOMVersion: doc.LockedBOMVersion,
		SnapshotData:     doc.SnapshotData,
	}, nil
}

// ── Staff Assignments ─────────────────────────────────────────────────────────

func (r *productionOrderRepository) AssignStaff(ctx context.Context, assignment *models.POStaffAssignment) error {
	doc := poStaffAssignmentDoc{
		POID:    assignment.POID,
		StaffID: assignment.StaffID,
		Hours:   assignment.Hours,
	}
	_, err := r.assignments.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("poRepository.AssignStaff: %w", err)
	}
	return nil
}

func (r *productionOrderRepository) ListStaffAssignments(ctx context.Context, poID string) ([]*models.POStaffAssignment, error) {
	cur, err := r.assignments.Find(ctx, bson.M{"po_id": poID})
	if err != nil {
		return nil, fmt.Errorf("poRepository.ListStaffAssignments: %w", err)
	}
	defer cur.Close(ctx)
	var result []*models.POStaffAssignment
	for cur.Next(ctx) {
		var doc poStaffAssignmentDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, &models.POStaffAssignment{
			POID:    doc.POID,
			StaffID: doc.StaffID,
			Hours:   doc.Hours,
		})
	}
	return result, cur.Err()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func decodePOs(ctx context.Context, cur *mongo.Cursor) ([]*models.ProductionOrder, error) {
	var result []*models.ProductionOrder
	for cur.Next(ctx) {
		var doc poDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, docToPO(&doc))
	}
	return result, cur.Err()
}
