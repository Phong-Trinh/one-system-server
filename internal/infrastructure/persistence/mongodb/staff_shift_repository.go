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

const collStaffShifts = "staff_shifts"

// ── BSON document ─────────────────────────────────────────────────────────────

type staffShiftDoc struct {
	ID         string              `bson:"_id"`
	StaffID    string              `bson:"staff_id"`
	NodeID     string              `bson:"node_id"`
	StationID  *string             `bson:"station_id,omitempty"` // FK → EquipmentType
	ShiftStart time.Time           `bson:"shift_start"`
	ShiftEnd   *time.Time          `bson:"shift_end,omitempty"`
	ActualEnd  *time.Time          `bson:"actual_end,omitempty"` // set khi EndShift sớm
	Status     models.ShiftStatus  `bson:"status"`
	CreatedAt  time.Time           `bson:"created_at"`
}

func shiftToDoc(s *models.StaffShift) *staffShiftDoc {
	return &staffShiftDoc{
		ID:         s.ID,
		StaffID:    s.StaffID,
		NodeID:     s.NodeID,
		StationID:  s.StationID,
		ShiftStart: s.ShiftStart,
		ShiftEnd:   s.ShiftEnd,
		ActualEnd:  s.ActualEnd,
		Status:     s.Status,
		CreatedAt:  s.CreatedAt,
	}
}

func docToShift(d *staffShiftDoc) *models.StaffShift {
	return &models.StaffShift{
		ID:         d.ID,
		StaffID:    d.StaffID,
		NodeID:     d.NodeID,
		StationID:  d.StationID,
		ShiftStart: d.ShiftStart,
		ShiftEnd:   d.ShiftEnd,
		ActualEnd:  d.ActualEnd,
		Status:     d.Status,
		CreatedAt:  d.CreatedAt,
	}
}

// ── Repository ────────────────────────────────────────────────────────────────

type staffShiftRepository struct {
	coll *mongo.Collection
}

// NewStaffShiftRepository returns a services.StaffShiftRepository backed by MongoDB.
func NewStaffShiftRepository(client *Client, dbName string) services.StaffShiftRepository {
	coll := client.DB(dbName).Collection(collStaffShifts)

	// staff_id: tìm ca của một nhân viên cụ thể
	_, _ = coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "staff_id", Value: 1}},
	})
	// node_id + status: SchedulingEngine query FindActiveByNode
	_, _ = coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "node_id", Value: 1}, {Key: "status", Value: 1}},
	})

	return &staffShiftRepository{coll: coll}
}

func (r *staffShiftRepository) Create(ctx context.Context, s *models.StaffShift) error {
	_, err := r.coll.InsertOne(ctx, shiftToDoc(s))
	if err != nil {
		return fmt.Errorf("staffShiftRepository.Create: %w", err)
	}
	return nil
}

func (r *staffShiftRepository) FindByID(ctx context.Context, id string) (*models.StaffShift, error) {
	var doc staffShiftDoc
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("staffShiftRepository.FindByID: %w", err)
	}
	return docToShift(&doc), nil
}

// FindActiveByNode trả về tất cả ca đang ACTIVE tại một node.
// SchedulingEngine gọi hàm này để biết ai đang available và đứng station nào.
func (r *staffShiftRepository) FindActiveByNode(ctx context.Context, nodeID string) ([]*models.StaffShift, error) {
	filter := bson.M{
		"node_id": nodeID,
		"status":  models.ShiftActive,
	}
	cur, err := r.coll.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("staffShiftRepository.FindActiveByNode: %w", err)
	}
	defer cur.Close(ctx)

	var result []*models.StaffShift
	for cur.Next(ctx) {
		var doc staffShiftDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, docToShift(&doc))
	}
	return result, cur.Err()
}

// FindByStaff trả về toàn bộ ca của một nhân viên (không filter status).
// Dùng để check xem staff có đang trong ca không trước khi assign task.
func (r *staffShiftRepository) FindByStaff(ctx context.Context, staffID string) ([]*models.StaffShift, error) {
	cur, err := r.coll.Find(ctx, bson.M{"staff_id": staffID})
	if err != nil {
		return nil, fmt.Errorf("staffShiftRepository.FindByStaff: %w", err)
	}
	defer cur.Close(ctx)

	var result []*models.StaffShift
	for cur.Next(ctx) {
		var doc staffShiftDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, docToShift(&doc))
	}
	return result, cur.Err()
}

// UpdateStatus cập nhật status của ca. actualEnd chỉ được set khi status = ShiftEnded.
func (r *staffShiftRepository) UpdateStatus(ctx context.Context, id string, status models.ShiftStatus, actualEnd *time.Time) error {
	update := bson.M{"$set": bson.M{"status": status}}
	if actualEnd != nil {
		update["$set"].(bson.M)["actual_end"] = actualEnd
	}
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("staffShiftRepository.UpdateStatus: %w", err)
	}
	return nil
}

// FindActiveShiftByStaff là helper nội bộ: tìm ca đang ACTIVE của staff, trả nil nếu không có.
// Dùng trong StaffShiftUseCase để validate trước khi StartShift hoặc EndShift.
func (r *staffShiftRepository) FindActiveShiftByStaff(ctx context.Context, staffID string) (*models.StaffShift, error) {
	filter := bson.M{
		"staff_id": staffID,
		"status":   models.ShiftActive,
	}
	var doc staffShiftDoc
	err := r.coll.FindOne(ctx, filter, options.FindOne().SetSort(bson.D{{Key: "shift_start", Value: -1}})).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("staffShiftRepository.FindActiveShiftByStaff: %w", err)
	}
	return docToShift(&doc), nil
}
