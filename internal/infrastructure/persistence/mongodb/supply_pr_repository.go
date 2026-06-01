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

const collPR = "purchase_requisitions"
const collPRLines = "purchase_requisition_lines"

// ── PurchaseRequisition ───────────────────────────────────────────────────────

type prDoc struct {
	ID              string          `bson:"_id"`
	OrgID           string          `bson:"org_id"`
	RequesterNodeID string          `bson:"requester_node_id"`
	RequesterStaff  string          `bson:"requester_staff_id"`
	Status          models.PRStatus `bson:"status"`
	Justification   string          `bson:"justification"`
	ReviewedBy      *string         `bson:"reviewed_by,omitempty"`
	ReviewNote      *string         `bson:"review_note,omitempty"`
	ReviewedAt      *time.Time      `bson:"reviewed_at,omitempty"`
	CreatedAt       time.Time       `bson:"created_at"`
	UpdatedAt       time.Time       `bson:"updated_at"`
}

func prToDoc(pr *models.PurchaseRequisition) *prDoc {
	return &prDoc{
		ID:              pr.ID,
		OrgID:           pr.OrgID,
		RequesterNodeID: pr.RequesterNodeID,
		RequesterStaff:  pr.RequesterStaff,
		Status:          pr.Status,
		Justification:   pr.Justification,
		ReviewedBy:      pr.ReviewedBy,
		ReviewNote:      pr.ReviewNote,
		ReviewedAt:      pr.ReviewedAt,
		CreatedAt:       pr.CreatedAt,
		UpdatedAt:       pr.UpdatedAt,
	}
}

func docToPR(d *prDoc) *models.PurchaseRequisition {
	return &models.PurchaseRequisition{
		ID:              d.ID,
		OrgID:           d.OrgID,
		RequesterNodeID: d.RequesterNodeID,
		RequesterStaff:  d.RequesterStaff,
		Status:          d.Status,
		Justification:   d.Justification,
		ReviewedBy:      d.ReviewedBy,
		ReviewNote:      d.ReviewNote,
		ReviewedAt:      d.ReviewedAt,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

type prRepository struct {
	col *mongo.Collection
}

func NewPurchaseRequisitionRepository(client *Client, dbName string) services.PurchaseRequisitionRepository {
	col := client.DB(dbName).Collection(collPR)
	_, _ = col.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "requester_node_id", Value: 1}}},
		{Keys: bson.D{{Key: "org_id", Value: 1}, {Key: "status", Value: 1}}},
	})
	return &prRepository{col: col}
}

func (r *prRepository) Create(ctx context.Context, pr *models.PurchaseRequisition) error {
	_, err := r.col.InsertOne(ctx, prToDoc(pr))
	if err != nil {
		return fmt.Errorf("prRepository.Create: %w", err)
	}
	return nil
}

func (r *prRepository) FindByID(ctx context.Context, id string) (*models.PurchaseRequisition, error) {
	var doc prDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("prRepository.FindByID: %w", err)
	}
	return docToPR(&doc), nil
}

func (r *prRepository) FindByNode(ctx context.Context, nodeID string) ([]*models.PurchaseRequisition, error) {
	cur, err := r.col.Find(ctx, bson.M{"requester_node_id": nodeID})
	if err != nil {
		return nil, fmt.Errorf("prRepository.FindByNode: %w", err)
	}
	defer cur.Close(ctx)

	var prs []*models.PurchaseRequisition
	for cur.Next(ctx) {
		var doc prDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		prs = append(prs, docToPR(&doc))
	}
	return prs, cur.Err()
}

func (r *prRepository) FindPendingByOrg(ctx context.Context, orgID string) ([]*models.PurchaseRequisition, error) {
	cur, err := r.col.Find(ctx, bson.M{
		"org_id": orgID,
		"status": bson.M{"$in": []models.PRStatus{models.PRPendingHQApproval, models.PRApproved}},
	})
	if err != nil {
		return nil, fmt.Errorf("prRepository.FindPendingByOrg: %w", err)
	}
	defer cur.Close(ctx)

	var prs []*models.PurchaseRequisition
	for cur.Next(ctx) {
		var doc prDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		prs = append(prs, docToPR(&doc))
	}
	return prs, cur.Err()
}

func (r *prRepository) Update(ctx context.Context, pr *models.PurchaseRequisition) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": pr.ID}, prToDoc(pr))
	if err != nil {
		return fmt.Errorf("prRepository.Update: %w", err)
	}
	return nil
}

// ── PRLine ────────────────────────────────────────────────────────────────────

type prLineDoc struct {
	ID                    string   `bson:"_id"`
	PRID                  string   `bson:"pr_id"`
	ItemID                *string  `bson:"item_id,omitempty"`
	EquipmentTypeID       *string  `bson:"equipment_type_id,omitempty"`
	ProposedEquipmentName *string  `bson:"proposed_equipment_name,omitempty"`
	ProposedCapacityUnit  *string  `bson:"proposed_capacity_unit,omitempty"`
	ExpectedCapacity      *float64 `bson:"expected_capacity,omitempty"`
	Qty                   float64  `bson:"qty"`
	UnitOfMeasure         string   `bson:"unit_of_measure"`
	EstimatedUnitPrice    float64  `bson:"estimated_unit_price"`
	Justification         string   `bson:"justification"`
}

func prLineToDoc(line *models.PRLine) *prLineDoc {
	return &prLineDoc{
		ID:                    line.ID,
		PRID:                  line.PRID,
		ItemID:                line.ItemID,
		EquipmentTypeID:       line.EquipmentTypeID,
		ProposedEquipmentName: line.ProposedEquipmentName,
		ProposedCapacityUnit:  line.ProposedCapacityUnit,
		ExpectedCapacity:      line.ExpectedCapacity,
		Qty:                   line.Qty,
		UnitOfMeasure:         line.UnitOfMeasure,
		EstimatedUnitPrice:    line.EstimatedUnitPrice,
		Justification:         line.Justification,
	}
}

func docToPRLine(d *prLineDoc) *models.PRLine {
	return &models.PRLine{
		ID:                    d.ID,
		PRID:                  d.PRID,
		ItemID:                d.ItemID,
		EquipmentTypeID:       d.EquipmentTypeID,
		ProposedEquipmentName: d.ProposedEquipmentName,
		ProposedCapacityUnit:  d.ProposedCapacityUnit,
		ExpectedCapacity:      d.ExpectedCapacity,
		Qty:                   d.Qty,
		UnitOfMeasure:         d.UnitOfMeasure,
		EstimatedUnitPrice:    d.EstimatedUnitPrice,
		Justification:         d.Justification,
	}
}

type prLineRepository struct {
	col *mongo.Collection
}

func NewPRLineRepository(client *Client, dbName string) services.PRLineRepository {
	col := client.DB(dbName).Collection(collPRLines)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "pr_id", Value: 1}},
	})
	return &prLineRepository{col: col}
}

func (r *prLineRepository) AddLine(ctx context.Context, line *models.PRLine) error {
	_, err := r.col.InsertOne(ctx, prLineToDoc(line))
	if err != nil {
		return fmt.Errorf("prLineRepository.AddLine: %w", err)
	}
	return nil
}

func (r *prLineRepository) ListByPR(ctx context.Context, prID string) ([]*models.PRLine, error) {
	cur, err := r.col.Find(ctx, bson.M{"pr_id": prID})
	if err != nil {
		return nil, fmt.Errorf("prLineRepository.ListByPR: %w", err)
	}
	defer cur.Close(ctx)

	var lines []*models.PRLine
	for cur.Next(ctx) {
		var doc prLineDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		lines = append(lines, docToPRLine(&doc))
	}
	return lines, cur.Err()
}
