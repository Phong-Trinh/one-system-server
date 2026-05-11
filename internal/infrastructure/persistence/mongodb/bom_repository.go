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
	collBOMs     = "boms"
	collBOMLines = "bom_lines"
)

// ── BSON documents ────────────────────────────────────────────────────────────

type bomDoc struct {
	ID           string `bson:"_id"`
	OutputItemID string `bson:"output_item_id"`
	Version      int    `bson:"version"`
}

type bomLineDoc struct {
	ID     string  `bson:"_id"`
	BOMID  string  `bson:"bom_id"`
	ItemID string  `bson:"item_id"`
	Qty    float64 `bson:"qty"`
}

func bomToDoc(b *models.BOM) *bomDoc {
	return &bomDoc{
		ID:           b.ID,
		OutputItemID: b.OutputItemID,
		Version:      b.Version,
	}
}

func docToBOM(d *bomDoc) *models.BOM {
	return &models.BOM{
		ID:           d.ID,
		OutputItemID: d.OutputItemID,
		Version:      d.Version,
	}
}

func bomLineToDoc(l *models.BOMLine) *bomLineDoc {
	return &bomLineDoc{ID: l.ID, BOMID: l.BOMID, ItemID: l.ItemID, Qty: l.Qty}
}

func docToBOMLine(d *bomLineDoc) *models.BOMLine {
	return &models.BOMLine{ID: d.ID, BOMID: d.BOMID, ItemID: d.ItemID, Qty: d.Qty}
}

// ── Repository ────────────────────────────────────────────────────────────────

type bomRepository struct {
	boms  *mongo.Collection
	lines *mongo.Collection
}

// NewBOMRepository returns a services.BOMRepository backed by MongoDB.
func NewBOMRepository(client *Client, dbName string) services.BOMRepository {
	boms := client.DB(dbName).Collection(collBOMs)
	lines := client.DB(dbName).Collection(collBOMLines)

	// Index: look up all BOMs for a given output item
	_, _ = boms.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "output_item_id", Value: 1}},
	})
	// Compound unique index on bom_lines: one row per (bom_id, item_id)
	_, _ = lines.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "bom_id", Value: 1}, {Key: "item_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	return &bomRepository{boms: boms, lines: lines}
}

// ── BOM CRUD ──────────────────────────────────────────────────────────────────

func (r *bomRepository) Create(ctx context.Context, bom *models.BOM) error {
	_, err := r.boms.InsertOne(ctx, bomToDoc(bom))
	if err != nil {
		return fmt.Errorf("bomRepository.Create: %w", err)
	}
	return nil
}

func (r *bomRepository) FindByID(ctx context.Context, id string) (*models.BOM, error) {
	var doc bomDoc
	err := r.boms.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("bomRepository.FindByID: %w", err)
	}
	return docToBOM(&doc), nil
}

func (r *bomRepository) FindByOutputItem(ctx context.Context, outputItemID string) ([]*models.BOM, error) {
	cur, err := r.boms.Find(ctx, bson.M{"output_item_id": outputItemID})
	if err != nil {
		return nil, fmt.Errorf("bomRepository.FindByOutputItem: %w", err)
	}
	defer cur.Close(ctx)
	return decodeBOMs(ctx, cur)
}

func (r *bomRepository) FindAll(ctx context.Context) ([]*models.BOM, error) {
	cur, err := r.boms.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("bomRepository.FindAll: %w", err)
	}
	defer cur.Close(ctx)
	return decodeBOMs(ctx, cur)
}

func (r *bomRepository) Update(ctx context.Context, bom *models.BOM) error {
	_, err := r.boms.ReplaceOne(ctx, bson.M{"_id": bom.ID}, bomToDoc(bom))
	if err != nil {
		return fmt.Errorf("bomRepository.Update: %w", err)
	}
	return nil
}

func (r *bomRepository) Delete(ctx context.Context, id string) error {
	_, err := r.boms.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("bomRepository.Delete: %w", err)
	}
	return nil
}

// ── BOM Lines ─────────────────────────────────────────────────────────────────

func (r *bomRepository) AddLine(ctx context.Context, line *models.BOMLine) error {
	opts := options.Replace().SetUpsert(true)
	_, err := r.lines.ReplaceOne(ctx, bson.M{"_id": line.ID}, bomLineToDoc(line), opts)
	if err != nil {
		return fmt.Errorf("bomRepository.AddLine: %w", err)
	}
	return nil
}

func (r *bomRepository) ListLines(ctx context.Context, bomID string) ([]*models.BOMLine, error) {
	cur, err := r.lines.Find(ctx, bson.M{"bom_id": bomID})
	if err != nil {
		return nil, fmt.Errorf("bomRepository.ListLines: %w", err)
	}
	defer cur.Close(ctx)
	var result []*models.BOMLine
	for cur.Next(ctx) {
		var doc bomLineDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, docToBOMLine(&doc))
	}
	return result, cur.Err()
}

func (r *bomRepository) DeleteLine(ctx context.Context, id string) error {
	_, err := r.lines.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("bomRepository.DeleteLine: %w", err)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func decodeBOMs(ctx context.Context, cur *mongo.Cursor) ([]*models.BOM, error) {
	var result []*models.BOM
	for cur.Next(ctx) {
		var doc bomDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, docToBOM(&doc))
	}
	return result, cur.Err()
}
