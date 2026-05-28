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

const collGoodsIssues = "goods_issues"
const collGoodsIssueLines = "goods_issue_lines"
const collGoodsReceipts = "goods_receipts"
const collGoodsReceiptLines = "goods_receipt_lines"
const collDiscrepancyTickets = "discrepancy_tickets"

// ── GoodsIssue ────────────────────────────────────────────────────────────────

type goodsIssueDoc struct {
	ID            string                  `bson:"_id"`
	RefType       models.GoodsIssueRefType `bson:"ref_type"`
	RefID         string                  `bson:"ref_id"`
	IssuingNodeID string                  `bson:"issuing_node_id"`
	DriverName    string                  `bson:"driver_name,omitempty"`
	DriverPhone   string                  `bson:"driver_phone,omitempty"`
	VehiclePlate  string                  `bson:"vehicle_plate,omitempty"`
	MediaURL      string                  `bson:"media_url,omitempty"`
	ShippingFee   float64                 `bson:"shipping_fee"`
	Status        models.GoodsIssueStatus `bson:"status"`
	IssuedAt      *time.Time              `bson:"issued_at,omitempty"`
	CreatedAt     time.Time               `bson:"created_at"`
	UpdatedAt     time.Time               `bson:"updated_at"`
}

func giToDoc(gi *models.GoodsIssue) *goodsIssueDoc {
	return &goodsIssueDoc{
		ID:            gi.ID,
		RefType:       gi.RefType,
		RefID:         gi.RefID,
		IssuingNodeID: gi.IssuingNodeID,
		DriverName:    gi.DriverName,
		DriverPhone:   gi.DriverPhone,
		VehiclePlate:  gi.VehiclePlate,
		MediaURL:      gi.MediaURL,
		ShippingFee:   gi.ShippingFee,
		Status:        gi.Status,
		IssuedAt:      gi.IssuedAt,
		CreatedAt:     gi.CreatedAt,
		UpdatedAt:     gi.UpdatedAt,
	}
}

func docToGI(d *goodsIssueDoc) *models.GoodsIssue {
	return &models.GoodsIssue{
		ID:            d.ID,
		RefType:       d.RefType,
		RefID:         d.RefID,
		IssuingNodeID: d.IssuingNodeID,
		DriverName:    d.DriverName,
		DriverPhone:   d.DriverPhone,
		VehiclePlate:  d.VehiclePlate,
		MediaURL:      d.MediaURL,
		ShippingFee:   d.ShippingFee,
		Status:        d.Status,
		IssuedAt:      d.IssuedAt,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

type giRepository struct {
	col *mongo.Collection
}

func NewGoodsIssueRepository(client *Client, dbName string) services.GoodsIssueRepository {
	col := client.DB(dbName).Collection(collGoodsIssues)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "ref_type", Value: 1}, {Key: "ref_id", Value: 1}},
	})
	return &giRepository{col: col}
}

func (r *giRepository) Create(ctx context.Context, gi *models.GoodsIssue) error {
	_, err := r.col.InsertOne(ctx, giToDoc(gi))
	if err != nil {
		return fmt.Errorf("giRepository.Create: %w", err)
	}
	return nil
}

func (r *giRepository) FindByID(ctx context.Context, id string) (*models.GoodsIssue, error) {
	var doc goodsIssueDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("giRepository.FindByID: %w", err)
	}
	return docToGI(&doc), nil
}

func (r *giRepository) UpdateStatus(ctx context.Context, id string, status models.GoodsIssueStatus) error {
	update := bson.M{"$set": bson.M{"status": status, "updated_at": time.Now()}}
	if status == models.GoodsIssueConfirmed {
		now := time.Now()
		update["$set"].(bson.M)["issued_at"] = &now
	}
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("giRepository.UpdateStatus: %w", err)
	}
	return nil
}

// ── GoodsIssueLine ────────────────────────────────────────────────────────────

type giLineDoc struct {
	ID        string  `bson:"_id"`
	GIID      string  `bson:"gi_id"`
	ItemID    string  `bson:"item_id"`
	QtyIssued float64 `bson:"qty_issued"`
}

func giLineToDoc(line *models.GoodsIssueLine) *giLineDoc {
	return &giLineDoc{
		ID:        line.ID,
		GIID:      line.GIID,
		ItemID:    line.ItemID,
		QtyIssued: line.QtyIssued,
	}
}

func docToGILine(d *giLineDoc) *models.GoodsIssueLine {
	return &models.GoodsIssueLine{
		ID:        d.ID,
		GIID:      d.GIID,
		ItemID:    d.ItemID,
		QtyIssued: d.QtyIssued,
	}
}

type giLineRepository struct {
	col *mongo.Collection
}

func NewGoodsIssueLineRepository(client *Client, dbName string) services.GoodsIssueLineRepository {
	col := client.DB(dbName).Collection(collGoodsIssueLines)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "gi_id", Value: 1}},
	})
	return &giLineRepository{col: col}
}

func (r *giLineRepository) AddLine(ctx context.Context, line *models.GoodsIssueLine) error {
	_, err := r.col.InsertOne(ctx, giLineToDoc(line))
	if err != nil {
		return fmt.Errorf("giLineRepository.AddLine: %w", err)
	}
	return nil
}

func (r *giLineRepository) ListByGI(ctx context.Context, giID string) ([]*models.GoodsIssueLine, error) {
	cur, err := r.col.Find(ctx, bson.M{"gi_id": giID})
	if err != nil {
		return nil, fmt.Errorf("giLineRepository.ListByGI: %w", err)
	}
	defer cur.Close(ctx)

	var lines []*models.GoodsIssueLine
	for cur.Next(ctx) {
		var doc giLineDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		lines = append(lines, docToGILine(&doc))
	}
	return lines, cur.Err()
}

// ── GoodsReceipt ──────────────────────────────────────────────────────────────

type goodsReceiptDoc struct {
	ID              string                    `bson:"_id"`
	RefType         models.GoodsReceiptRefType `bson:"ref_type"`
	RefID           string                    `bson:"ref_id"`
	ReceivingNodeID string                    `bson:"receiving_node_id"`
	Status          models.GoodsReceiptStatus `bson:"status"`
	ReceivedBy      string                    `bson:"received_by"`
	ReceivedAt      *time.Time                `bson:"received_at,omitempty"`
	CreatedAt       time.Time                 `bson:"created_at"`
	UpdatedAt       time.Time                 `bson:"updated_at"`
}

func grToDoc(gr *models.GoodsReceipt) *goodsReceiptDoc {
	return &goodsReceiptDoc{
		ID:              gr.ID,
		RefType:         gr.RefType,
		RefID:           gr.RefID,
		ReceivingNodeID: gr.ReceivingNodeID,
		Status:          gr.Status,
		ReceivedBy:      gr.ReceivedBy,
		ReceivedAt:      gr.ReceivedAt,
		CreatedAt:       gr.CreatedAt,
		UpdatedAt:       gr.UpdatedAt,
	}
}

func docToGR(d *goodsReceiptDoc) *models.GoodsReceipt {
	return &models.GoodsReceipt{
		ID:              d.ID,
		RefType:         d.RefType,
		RefID:           d.RefID,
		ReceivingNodeID: d.ReceivingNodeID,
		Status:          d.Status,
		ReceivedBy:      d.ReceivedBy,
		ReceivedAt:      d.ReceivedAt,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

type grRepository struct {
	col *mongo.Collection
}

func NewGoodsReceiptRepository(client *Client, dbName string) services.GoodsReceiptRepository {
	col := client.DB(dbName).Collection(collGoodsReceipts)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "ref_type", Value: 1}, {Key: "ref_id", Value: 1}},
	})
	return &grRepository{col: col}
}

func (r *grRepository) Create(ctx context.Context, gr *models.GoodsReceipt) error {
	_, err := r.col.InsertOne(ctx, grToDoc(gr))
	if err != nil {
		return fmt.Errorf("grRepository.Create: %w", err)
	}
	return nil
}

func (r *grRepository) FindByID(ctx context.Context, id string) (*models.GoodsReceipt, error) {
	var doc goodsReceiptDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("grRepository.FindByID: %w", err)
	}
	return docToGR(&doc), nil
}

func (r *grRepository) UpdateStatus(ctx context.Context, id string, status models.GoodsReceiptStatus) error {
	update := bson.M{"$set": bson.M{"status": status, "updated_at": time.Now()}}
	if status == models.GoodsReceiptConfirmed || status == models.GoodsReceiptDiscrepancy {
		now := time.Now()
		update["$set"].(bson.M)["received_at"] = &now
	}
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("grRepository.UpdateStatus: %w", err)
	}
	return nil
}

// ── GoodsReceiptLine ──────────────────────────────────────────────────────────

type grLineDoc struct {
	ID          string  `bson:"_id"`
	GRID        string  `bson:"gr_id"`
	ItemID      string  `bson:"item_id"`
	QtyExpected float64 `bson:"qty_expected"`
	QtyReceived float64 `bson:"qty_received"`
}

func grLineToDoc(line *models.GoodsReceiptLine) *grLineDoc {
	return &grLineDoc{
		ID:          line.ID,
		GRID:        line.GRID,
		ItemID:      line.ItemID,
		QtyExpected: line.QtyExpected,
		QtyReceived: line.QtyReceived,
	}
}

func docToGRLine(d *grLineDoc) *models.GoodsReceiptLine {
	return &models.GoodsReceiptLine{
		ID:          d.ID,
		GRID:        d.GRID,
		ItemID:      d.ItemID,
		QtyExpected: d.QtyExpected,
		QtyReceived: d.QtyReceived,
	}
}

type grLineRepository struct {
	col *mongo.Collection
}

func NewGoodsReceiptLineRepository(client *Client, dbName string) services.GoodsReceiptLineRepository {
	col := client.DB(dbName).Collection(collGoodsReceiptLines)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "gr_id", Value: 1}},
	})
	return &grLineRepository{col: col}
}

func (r *grLineRepository) AddLine(ctx context.Context, line *models.GoodsReceiptLine) error {
	_, err := r.col.InsertOne(ctx, grLineToDoc(line))
	if err != nil {
		return fmt.Errorf("grLineRepository.AddLine: %w", err)
	}
	return nil
}

func (r *grLineRepository) ListByGR(ctx context.Context, grID string) ([]*models.GoodsReceiptLine, error) {
	cur, err := r.col.Find(ctx, bson.M{"gr_id": grID})
	if err != nil {
		return nil, fmt.Errorf("grLineRepository.ListByGR: %w", err)
	}
	defer cur.Close(ctx)

	var lines []*models.GoodsReceiptLine
	for cur.Next(ctx) {
		var doc grLineDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		lines = append(lines, docToGRLine(&doc))
	}
	return lines, cur.Err()
}

// ── DiscrepancyTicket ─────────────────────────────────────────────────────────

type discrepancyTicketDoc struct {
	ID         string                         `bson:"_id"`
	GRID       string                         `bson:"gr_id"`
	ItemID     string                         `bson:"item_id"`
	QtyMissing float64                        `bson:"qty_missing"`
	QtyDamaged float64                        `bson:"qty_damaged"`
	Status     models.DiscrepancyTicketStatus `bson:"status"`
	Resolution *string                        `bson:"resolution,omitempty"`
	ResolvedBy *string                        `bson:"resolved_by,omitempty"`
	ResolvedAt *time.Time                     `bson:"resolved_at,omitempty"`
	CreatedAt  time.Time                      `bson:"created_at"`
	UpdatedAt  time.Time                      `bson:"updated_at"`
}

func ticketToDoc(dt *models.DiscrepancyTicket) *discrepancyTicketDoc {
	return &discrepancyTicketDoc{
		ID:         dt.ID,
		GRID:       dt.GRID,
		ItemID:     dt.ItemID,
		QtyMissing: dt.QtyMissing,
		QtyDamaged: dt.QtyDamaged,
		Status:     dt.Status,
		Resolution: dt.Resolution,
		ResolvedBy: dt.ResolvedBy,
		ResolvedAt: dt.ResolvedAt,
		CreatedAt:  dt.CreatedAt,
		UpdatedAt:  dt.UpdatedAt,
	}
}

func docToTicket(d *discrepancyTicketDoc) *models.DiscrepancyTicket {
	return &models.DiscrepancyTicket{
		ID:         d.ID,
		GRID:       d.GRID,
		ItemID:     d.ItemID,
		QtyMissing: d.QtyMissing,
		QtyDamaged: d.QtyDamaged,
		Status:     d.Status,
		Resolution: d.Resolution,
		ResolvedBy: d.ResolvedBy,
		ResolvedAt: d.ResolvedAt,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

type discrepancyTicketRepository struct {
	col *mongo.Collection
}

func NewDiscrepancyTicketRepository(client *Client, dbName string) services.DiscrepancyTicketRepository {
	col := client.DB(dbName).Collection(collDiscrepancyTickets)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "gr_id", Value: 1}},
	})
	return &discrepancyTicketRepository{col: col}
}

func (r *discrepancyTicketRepository) Create(ctx context.Context, dt *models.DiscrepancyTicket) error {
	_, err := r.col.InsertOne(ctx, ticketToDoc(dt))
	if err != nil {
		return fmt.Errorf("discrepancyTicketRepository.Create: %w", err)
	}
	return nil
}

func (r *discrepancyTicketRepository) FindByID(ctx context.Context, id string) (*models.DiscrepancyTicket, error) {
	var doc discrepancyTicketDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("discrepancyTicketRepository.FindByID: %w", err)
	}
	return docToTicket(&doc), nil
}

func (r *discrepancyTicketRepository) FindByGR(ctx context.Context, grID string) ([]*models.DiscrepancyTicket, error) {
	cur, err := r.col.Find(ctx, bson.M{"gr_id": grID})
	if err != nil {
		return nil, fmt.Errorf("discrepancyTicketRepository.FindByGR: %w", err)
	}
	defer cur.Close(ctx)

	var tickets []*models.DiscrepancyTicket
	for cur.Next(ctx) {
		var doc discrepancyTicketDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		tickets = append(tickets, docToTicket(&doc))
	}
	return tickets, cur.Err()
}

func (r *discrepancyTicketRepository) UpdateStatus(ctx context.Context, id string, status models.DiscrepancyTicketStatus, resolution *string, resolvedBy *string) error {
	update := bson.M{
		"$set": bson.M{
			"status":      status,
			"resolution":  resolution,
			"resolved_by": resolvedBy,
			"updated_at":  time.Now(),
		},
	}
	if status == models.DiscrepancyResolved {
		now := time.Now()
		update["$set"].(bson.M)["resolved_at"] = &now
	}
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("discrepancyTicketRepository.UpdateStatus: %w", err)
	}
	return nil
}
