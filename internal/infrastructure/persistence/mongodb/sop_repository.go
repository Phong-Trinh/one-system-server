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

const (
	collSOPs     = "sops"
	collSOPSteps = "sop_steps"
)

// ── BSON documents ────────────────────────────────────────────────────────────

type sopDoc struct {
	ID      string `bson:"_id"`
	BOMID   string `bson:"bom_id"`
	Version int    `bson:"version"`
}

type sopStepDoc struct {
	ID                   string   `bson:"_id"`
	SOPID                string   `bson:"sop_id"`
	SeqNo                int      `bson:"seq_no"`
	DependsOn            []string `bson:"depends_on"`
	StationTypeID        string   `bson:"station_type_id,omitempty"`
	SlotConsumption      float64  `bson:"slot_consumption"`
	AllowMix             bool     `bson:"allow_mix"`
	IngredientBOMLineIDs []string `bson:"ingredient_bom_line_ids"`
	Duration             int      `bson:"duration"`
	Description          string   `bson:"description"`
}

func sopToDoc(s *models.SOP) *sopDoc {
	return &sopDoc{ID: s.ID, BOMID: s.BOMID, Version: s.Version}
}

func docToSOP(d *sopDoc) *models.SOP {
	return &models.SOP{ID: d.ID, BOMID: d.BOMID, Version: d.Version}
}

func sopStepToDoc(s *models.SOPStep) *sopStepDoc {
	return &sopStepDoc{
		ID:                   s.ID,
		SOPID:                s.SOPID,
		SeqNo:                s.SeqNo,
		DependsOn:            s.DependsOn,
		StationTypeID:        s.StationTypeID,
		SlotConsumption:      s.SlotConsumption,
		AllowMix:             s.AllowMix,
		IngredientBOMLineIDs: s.IngredientBOMLineIDs,
		Duration:             s.Duration,
		Description:          s.Description,
	}
}

func docToSOPStep(d *sopStepDoc) *models.SOPStep {
	return &models.SOPStep{
		ID:                   d.ID,
		SOPID:                d.SOPID,
		SeqNo:                d.SeqNo,
		DependsOn:            d.DependsOn,
		StationTypeID:        d.StationTypeID,
		SlotConsumption:      d.SlotConsumption,
		AllowMix:             d.AllowMix,
		IngredientBOMLineIDs: d.IngredientBOMLineIDs,
		Duration:             d.Duration,
		Description:          d.Description,
	}
}

// ── Repository ────────────────────────────────────────────────────────────────

type sopRepository struct {
	sops  *mongo.Collection
	steps *mongo.Collection
}

// NewSOPRepository returns a services.SOPRepository backed by MongoDB.
func NewSOPRepository(client *Client, dbName string) services.SOPRepository {
	sops := client.DB(dbName).Collection(collSOPs)
	steps := client.DB(dbName).Collection(collSOPSteps)

	// Index: find SOP by its parent BOM
	_, _ = sops.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "bom_id", Value: 1}},
	})
	// Index: find SOP Steps by SOPID
	_, _ = steps.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "sop_id", Value: 1}},
	})

	// Index: unique seq_no per SOP
	_, _ = steps.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "sop_id", Value: 1}, {Key: "seq_no", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	return &sopRepository{sops: sops, steps: steps}
}

// ── SOP CRUD ──────────────────────────────────────────────────────────────────

func (r *sopRepository) Create(ctx context.Context, sop *models.SOP) error {
	_, err := r.sops.InsertOne(ctx, sopToDoc(sop))
	if err != nil {
		return fmt.Errorf("sopRepository.Create: %w", err)
	}
	return nil
}

func (r *sopRepository) FindByID(ctx context.Context, id string) (*models.SOP, error) {
	var doc sopDoc
	err := r.sops.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sopRepository.FindByID: %w", err)
	}
	return docToSOP(&doc), nil
}

func (r *sopRepository) FindByBOMID(ctx context.Context, bomID string) (*models.SOP, error) {
	var doc sopDoc
	err := r.sops.FindOne(ctx, bson.M{"bom_id": bomID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sopRepository.FindByBOMID: %w", err)
	}
	return docToSOP(&doc), nil
}

func (r *sopRepository) Update(ctx context.Context, sop *models.SOP) error {
	_, err := r.sops.ReplaceOne(ctx, bson.M{"_id": sop.ID}, sopToDoc(sop))
	if err != nil {
		return fmt.Errorf("sopRepository.Update: %w", err)
	}
	return nil
}

func (r *sopRepository) Delete(ctx context.Context, id string) error {
	_, err := r.sops.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("sopRepository.Delete: %w", err)
	}
	return nil
}

// ── SOP Steps ─────────────────────────────────────────────────────────────────

func (r *sopRepository) AddStep(ctx context.Context, step *models.SOPStep) error {
	opts := options.Replace().SetUpsert(true)
	_, err := r.steps.ReplaceOne(ctx, bson.M{"_id": step.ID}, sopStepToDoc(step), opts)
	if err != nil {
		return fmt.Errorf("sopRepository.AddStep: %w", err)
	}
	return nil
}

func (r *sopRepository) FindStepByID(ctx context.Context, id string) (*models.SOPStep, error) {
	var doc sopStepDoc
	err := r.steps.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sopRepository.FindStepByID: %w", err)
	}
	return docToSOPStep(&doc), nil
}

// ListSteps returns steps for a SOP.
func (r *sopRepository) ListSteps(ctx context.Context, sopID string) ([]*models.SOPStep, error) {
	cur, err := r.steps.Find(ctx, bson.M{"sop_id": sopID})
	if err != nil {
		return nil, fmt.Errorf("sopRepository.ListSteps: %w", err)
	}
	defer cur.Close(ctx)
	var result []*models.SOPStep
	for cur.Next(ctx) {
		var doc sopStepDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, docToSOPStep(&doc))
	}
	return result, cur.Err()
}

func (r *sopRepository) DeleteStep(ctx context.Context, sopID string, stepID string) error {
	_, err := r.steps.DeleteOne(ctx, bson.M{"sop_id": sopID, "_id": stepID})
	if err != nil {
		return fmt.Errorf("sopRepository.DeleteStep: %w", err)
	}
	return nil
}
