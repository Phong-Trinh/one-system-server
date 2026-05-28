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

const collSupplierInvoices = "supplier_invoices"
const collSupplierInvoiceLines = "supplier_invoice_lines"

// ── SupplierInvoice ───────────────────────────────────────────────────────────

type supplierInvoiceDoc struct {
	ID              string                       `bson:"_id"`
	OrgID           string                       `bson:"org_id"`
	PurchaseOrderID string                       `bson:"purchase_order_id"`
	SupplierID      string                       `bson:"supplier_id"`
	GRID            *string                      `bson:"gr_id,omitempty"`
	InvoiceNumber   string                       `bson:"invoice_number"`
	TotalAmount     float64                      `bson:"total_amount"`
	TaxAmount       float64                      `bson:"tax_amount"`
	ImageURL        string                       `bson:"image_url"`
	Status          models.SupplierInvoiceStatus `bson:"status"`
	MatchedBy       *string                      `bson:"matched_by,omitempty"`
	MatchedAt       *time.Time                   `bson:"matched_at,omitempty"`
	PaidAt          *time.Time                   `bson:"paid_at,omitempty"`
	InvoiceDate     time.Time                    `bson:"invoice_date"`
	CreatedAt       time.Time                    `bson:"created_at"`
	UpdatedAt       time.Time                    `bson:"updated_at"`
}

func invoiceToDoc(inv *models.SupplierInvoice) *supplierInvoiceDoc {
	return &supplierInvoiceDoc{
		ID:              inv.ID,
		OrgID:           inv.OrgID,
		PurchaseOrderID: inv.PurchaseOrderID,
		SupplierID:      inv.SupplierID,
		GRID:            inv.GRID,
		InvoiceNumber:   inv.InvoiceNumber,
		TotalAmount:     inv.TotalAmount,
		TaxAmount:       inv.TaxAmount,
		ImageURL:        inv.ImageURL,
		Status:          inv.Status,
		MatchedBy:       inv.MatchedBy,
		MatchedAt:       inv.MatchedAt,
		PaidAt:          inv.PaidAt,
		InvoiceDate:     inv.InvoiceDate,
		CreatedAt:       inv.CreatedAt,
		UpdatedAt:       inv.UpdatedAt,
	}
}

func docToInvoice(d *supplierInvoiceDoc) *models.SupplierInvoice {
	return &models.SupplierInvoice{
		ID:              d.ID,
		OrgID:           d.OrgID,
		PurchaseOrderID: d.PurchaseOrderID,
		SupplierID:      d.SupplierID,
		GRID:            d.GRID,
		InvoiceNumber:   d.InvoiceNumber,
		TotalAmount:     d.TotalAmount,
		TaxAmount:       d.TaxAmount,
		ImageURL:        d.ImageURL,
		Status:          d.Status,
		MatchedBy:       d.MatchedBy,
		MatchedAt:       d.MatchedAt,
		PaidAt:          d.PaidAt,
		InvoiceDate:     d.InvoiceDate,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

type invoiceRepository struct {
	col *mongo.Collection
}

func NewSupplierInvoiceRepository(client *Client, dbName string) services.SupplierInvoiceRepository {
	col := client.DB(dbName).Collection(collSupplierInvoices)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "purchase_order_id", Value: 1}},
	})
	return &invoiceRepository{col: col}
}

func (r *invoiceRepository) Create(ctx context.Context, inv *models.SupplierInvoice) error {
	_, err := r.col.InsertOne(ctx, invoiceToDoc(inv))
	if err != nil {
		return fmt.Errorf("invoiceRepository.Create: %w", err)
	}
	return nil
}

func (r *invoiceRepository) FindByID(ctx context.Context, id string) (*models.SupplierInvoice, error) {
	var doc supplierInvoiceDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("invoiceRepository.FindByID: %w", err)
	}
	return docToInvoice(&doc), nil
}

func (r *invoiceRepository) FindByPurO(ctx context.Context, purOID string) ([]*models.SupplierInvoice, error) {
	cur, err := r.col.Find(ctx, bson.M{"purchase_order_id": purOID})
	if err != nil {
		return nil, fmt.Errorf("invoiceRepository.FindByPurO: %w", err)
	}
	defer cur.Close(ctx)

	var invoices []*models.SupplierInvoice
	for cur.Next(ctx) {
		var doc supplierInvoiceDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		invoices = append(invoices, docToInvoice(&doc))
	}
	return invoices, cur.Err()
}

func (r *invoiceRepository) Update(ctx context.Context, inv *models.SupplierInvoice) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": inv.ID}, invoiceToDoc(inv))
	if err != nil {
		return fmt.Errorf("invoiceRepository.Update: %w", err)
	}
	return nil
}

// ── SupplierInvoiceLine ───────────────────────────────────────────────────────

type invoiceLineDoc struct {
	ID          string  `bson:"_id"`
	InvoiceID   string  `bson:"invoice_id"`
	ItemID      *string `bson:"item_id,omitempty"`
	RawLineText string  `bson:"raw_line_text"`
	Qty         float64 `bson:"qty"`
	UnitPrice   float64 `bson:"unit_price"`
	LineTotal   float64 `bson:"line_total"`
}

func invoiceLineToDoc(line *models.SupplierInvoiceLine) *invoiceLineDoc {
	return &invoiceLineDoc{
		ID:          line.ID,
		InvoiceID:   line.InvoiceID,
		ItemID:      line.ItemID,
		RawLineText: line.RawLineText,
		Qty:         line.Qty,
		UnitPrice:   line.UnitPrice,
		LineTotal:   line.LineTotal,
	}
}

func docToInvoiceLine(d *invoiceLineDoc) *models.SupplierInvoiceLine {
	return &models.SupplierInvoiceLine{
		ID:          d.ID,
		InvoiceID:   d.InvoiceID,
		ItemID:      d.ItemID,
		RawLineText: d.RawLineText,
		Qty:         d.Qty,
		UnitPrice:   d.UnitPrice,
		LineTotal:   d.LineTotal,
	}
}

type invoiceLineRepository struct {
	col *mongo.Collection
}

func NewSupplierInvoiceLineRepository(client *Client, dbName string) services.SupplierInvoiceLineRepository {
	col := client.DB(dbName).Collection(collSupplierInvoiceLines)
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "invoice_id", Value: 1}},
	})
	return &invoiceLineRepository{col: col}
}

func (r *invoiceLineRepository) AddLine(ctx context.Context, line *models.SupplierInvoiceLine) error {
	_, err := r.col.InsertOne(ctx, invoiceLineToDoc(line))
	if err != nil {
		return fmt.Errorf("invoiceLineRepository.AddLine: %w", err)
	}
	return nil
}

func (r *invoiceLineRepository) ListByInvoice(ctx context.Context, invoiceID string) ([]*models.SupplierInvoiceLine, error) {
	cur, err := r.col.Find(ctx, bson.M{"invoice_id": invoiceID})
	if err != nil {
		return nil, fmt.Errorf("invoiceLineRepository.ListByInvoice: %w", err)
	}
	defer cur.Close(ctx)

	var lines []*models.SupplierInvoiceLine
	for cur.Next(ctx) {
		var doc invoiceLineDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		lines = append(lines, docToInvoiceLine(&doc))
	}
	return lines, cur.Err()
}
