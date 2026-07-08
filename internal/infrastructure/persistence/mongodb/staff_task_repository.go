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

const collStaffTasks = "staff_tasks"

// ── BSON document ─────────────────────────────────────────────────────────────

type staffTaskDoc struct {
	ID              string             `bson:"_id"`
	POID            string             `bson:"po_id"`
	OrderItemID     *string            `bson:"order_item_id,omitempty"` // FK → OrderItem (nil = Phase 1)
	SOPStepID       string             `bson:"sop_step_id"`
	TaskKind        models.TaskKind    `bson:"task_kind"`
	AssignedTo      string             `bson:"assigned_to"`  // "" = unassigned
	MachineID       string             `bson:"machine_id"`   // "" = manual step
	NodeID          string             `bson:"node_id"`
	Status          models.TaskStatus  `bson:"status"`
	Priority        int                `bson:"priority"`
	IsInterruptible bool               `bson:"is_interruptible"`
	ParentTaskID    *string            `bson:"parent_task_id,omitempty"`
	ScheduledStart  time.Time          `bson:"scheduled_start"`
	ScheduledEnd    time.Time          `bson:"scheduled_end"`
	StartedAt       *time.Time         `bson:"started_at,omitempty"`
	CompletedAt     *time.Time         `bson:"completed_at,omitempty"`
	CreatedAt       time.Time          `bson:"created_at"`
}

func taskToDoc(t *models.StaffTask) *staffTaskDoc {
	return &staffTaskDoc{
		ID:              t.ID,
		POID:            t.POID,
		OrderItemID:     t.OrderItemID,
		SOPStepID:       t.SOPStepID,
		TaskKind:        t.TaskKind,
		AssignedTo:      t.AssignedTo,
		MachineID:       t.MachineID,
		NodeID:          t.NodeID,
		Status:          t.Status,
		Priority:        t.Priority,
		IsInterruptible: t.IsInterruptible,
		ParentTaskID:    t.ParentTaskID,
		ScheduledStart:  t.ScheduledStart,
		ScheduledEnd:    t.ScheduledEnd,
		StartedAt:       t.StartedAt,
		CompletedAt:     t.CompletedAt,
		CreatedAt:       t.CreatedAt,
	}
}

func docToTask(d *staffTaskDoc) *models.StaffTask {
	return &models.StaffTask{
		ID:              d.ID,
		POID:            d.POID,
		OrderItemID:     d.OrderItemID,
		SOPStepID:       d.SOPStepID,
		TaskKind:        d.TaskKind,
		AssignedTo:      d.AssignedTo,
		MachineID:       d.MachineID,
		NodeID:          d.NodeID,
		Status:          d.Status,
		Priority:        d.Priority,
		IsInterruptible: d.IsInterruptible,
		ParentTaskID:    d.ParentTaskID,
		ScheduledStart:  d.ScheduledStart,
		ScheduledEnd:    d.ScheduledEnd,
		StartedAt:       d.StartedAt,
		CompletedAt:     d.CompletedAt,
		CreatedAt:       d.CreatedAt,
	}
}

// ── Repository ────────────────────────────────────────────────────────────────

type staffTaskRepository struct {
	coll *mongo.Collection
}

// NewStaffTaskRepository returns a services.StaffTaskRepository backed by MongoDB.
func NewStaffTaskRepository(client *Client, dbName string) services.StaffTaskRepository {
	coll := client.DB(dbName).Collection(collStaffTasks)

	// po_id: FindByPO — check PO completion
	_, _ = coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "po_id", Value: 1}},
	})
	// assigned_to + status: FindByStaff (scheduler tính free_at), FindActiveByStaff (KDS)
	_, _ = coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "assigned_to", Value: 1}, {Key: "status", Value: 1}},
	})
	// node_id + status: FindByNode (Manager view, fill-in lookup), FindQueued (Dispatcher)
	_, _ = coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "node_id", Value: 1}, {Key: "status", Value: 1}},
	})
	// parent_task_id: FindWaitingByStaff — tìm fill-in tasks của một parent WAITING task
	_, _ = coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "parent_task_id", Value: 1}},
	})
	// created_at: FIFO ordering trong Dispatcher.Dispatch()
	_, _ = coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "created_at", Value: 1}},
	})

	return &staffTaskRepository{coll: coll}
}

func (r *staffTaskRepository) Create(ctx context.Context, t *models.StaffTask) error {
	_, err := r.coll.InsertOne(ctx, taskToDoc(t))
	if err != nil {
		return fmt.Errorf("staffTaskRepository.Create: %w", err)
	}
	return nil
}

func (r *staffTaskRepository) FindByID(ctx context.Context, id string) (*models.StaffTask, error) {
	var doc staffTaskDoc
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("staffTaskRepository.FindByID: %w", err)
	}
	return docToTask(&doc), nil
}

// FindByPO trả về tất cả tasks liên kết với một PO.
// Dùng để check liệu tất cả SOPStep của PO đã có task DONE chưa.
func (r *staffTaskRepository) FindByPO(ctx context.Context, poID string) ([]*models.StaffTask, error) {
	return r.findMany(ctx, bson.M{"po_id": poID}, "staffTaskRepository.FindByPO")
}

// FindByStaff trả về tasks của staff, optionally filter theo status.
// SchedulingEngine dùng để tính staff_free_at = ScheduledEnd của task cuối trong danh sách.
// nil statuses = tất cả tasks (kể cả DONE, CANCELLED).
func (r *staffTaskRepository) FindByStaff(ctx context.Context, staffID string, statuses []models.TaskStatus) ([]*models.StaffTask, error) {
	filter := bson.M{"assigned_to": staffID}
	if len(statuses) > 0 {
		filter["status"] = bson.M{"$in": statuses}
	}
	return r.findMany(ctx, filter, "staffTaskRepository.FindByStaff")
}

// FindByNode trả về tasks tại một node, optionally filter theo status.
// Dùng cho Manager overview và để SchedulingEngine tìm PENDING tasks chưa assign
// (candidates cho fill-in task lookup).
func (r *staffTaskRepository) FindByNode(ctx context.Context, nodeID string, statuses []models.TaskStatus) ([]*models.StaffTask, error) {
	filter := bson.M{"node_id": nodeID}
	if len(statuses) > 0 {
		filter["status"] = bson.M{"$in": statuses}
	}
	return r.findMany(ctx, filter, "staffTaskRepository.FindByNode")
}

// FindActiveByStaff trả về task đang ACTIVE hoặc WAITING của staff.
// Staff KDS polling dùng endpoint này mỗi 3 giây để refresh màn hình.
// Ưu tiên ACTIVE trước WAITING (nhân viên đang làm việc gì đó cụ thể).
func (r *staffTaskRepository) FindActiveByStaff(ctx context.Context, staffID string) (*models.StaffTask, error) {
	filter := bson.M{
		"assigned_to": staffID,
		"status":      bson.M{"$in": []models.TaskStatus{models.TaskActive, models.TaskWaiting}},
	}
	var doc staffTaskDoc
	// Sắp xếp: ACTIVE trước WAITING, rồi theo priority tăng dần
	opts := options.FindOne().SetSort(bson.D{
		{Key: "status", Value: 1}, // "ACTIVE" < "WAITING" lexicographically
		{Key: "priority", Value: 1},
	})
	err := r.coll.FindOne(ctx, filter, opts).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("staffTaskRepository.FindActiveByStaff: %w", err)
	}
	return docToTask(&doc), nil
}

// FindWaitingByStaff trả về các tasks đang WAITING của staff (idle step đang chạy).
// SchedulingEngine dùng khi tìm idle windows để chèn fill-in tasks vào.
func (r *staffTaskRepository) FindWaitingByStaff(ctx context.Context, staffID string) ([]*models.StaffTask, error) {
	filter := bson.M{
		"assigned_to": staffID,
		"status":      models.TaskWaiting,
	}
	return r.findMany(ctx, filter, "staffTaskRepository.FindWaitingByStaff")
}

// FindQueued trả về tất cả QUEUED tasks tại một node, sắp xếp theo CreatedAt tăng dần (FIFO).
// Dispatcher dùng để lấy danh sách tasks cần được assign.
func (r *staffTaskRepository) FindQueued(ctx context.Context, nodeID string) ([]*models.StaffTask, error) {
	filter := bson.M{
		"node_id": nodeID,
		"status":  models.TaskQueued,
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("staffTaskRepository.FindQueued: %w", err)
	}
	defer cur.Close(ctx)

	var result []*models.StaffTask
	for cur.Next(ctx) {
		var doc staffTaskDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, docToTask(&doc))
	}
	return result, cur.Err()
}

// Update ghi lại toàn bộ document của task.
// Dùng khi cập nhật Status + timestamps (StartedAt, CompletedAt).
func (r *staffTaskRepository) Update(ctx context.Context, t *models.StaffTask) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": t.ID}, taskToDoc(t))
	if err != nil {
		return fmt.Errorf("staffTaskRepository.Update: %w", err)
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// findMany là helper chung cho các Find queries trả về slice.
func (r *staffTaskRepository) findMany(ctx context.Context, filter bson.M, caller string) ([]*models.StaffTask, error) {
	cur, err := r.coll.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", caller, err)
	}
	defer cur.Close(ctx)

	var result []*models.StaffTask
	for cur.Next(ctx) {
		var doc staffTaskDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, docToTask(&doc))
	}
	return result, cur.Err()
}
