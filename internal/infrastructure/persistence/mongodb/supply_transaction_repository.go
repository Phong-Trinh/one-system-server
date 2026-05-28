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

const collTransactions = "transactions"

type transactionDoc struct {
	ID          string                    `bson:"_id"`
	NodeID      string                    `bson:"node_id"`
	OrgID       string                    `bson:"org_id"`
	Amount      float64                   `bson:"amount"`
	Type        models.TransactionType    `bson:"type"`
	RefType     models.TransactionRefType `bson:"ref_type"`
	ReferenceID string                    `bson:"reference_id"`
	Description string                    `bson:"description"`
	Timestamp   time.Time                 `bson:"timestamp"`
}

func txToDoc(tx *models.Transaction) *transactionDoc {
	return &transactionDoc{
		ID:          tx.ID,
		NodeID:      tx.NodeID,
		OrgID:       tx.OrgID,
		Amount:      tx.Amount,
		Type:        tx.Type,
		RefType:     tx.RefType,
		ReferenceID: tx.ReferenceID,
		Description: tx.Description,
		Timestamp:   tx.Timestamp,
	}
}

func docToTx(d *transactionDoc) *models.Transaction {
	return &models.Transaction{
		ID:          d.ID,
		NodeID:      d.NodeID,
		OrgID:       d.OrgID,
		Amount:      d.Amount,
		Type:        d.Type,
		RefType:     d.RefType,
		ReferenceID: d.ReferenceID,
		Description: d.Description,
		Timestamp:   d.Timestamp,
	}
}

type transactionRepository struct {
	col *mongo.Collection
}

func NewTransactionRepository(client *Client, dbName string) services.TransactionRepository {
	col := client.DB(dbName).Collection(collTransactions)
	_, _ = col.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "node_id", Value: 1}}},
		{Keys: bson.D{{Key: "ref_type", Value: 1}, {Key: "reference_id", Value: 1}}},
	})
	return &transactionRepository{col: col}
}

func (r *transactionRepository) Create(ctx context.Context, tx *models.Transaction) error {
	_, err := r.col.InsertOne(ctx, txToDoc(tx))
	if err != nil {
		return fmt.Errorf("transactionRepository.Create: %w", err)
	}
	return nil
}

func (r *transactionRepository) FindByID(ctx context.Context, id string) (*models.Transaction, error) {
	var doc transactionDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("transactionRepository.FindByID: %w", err)
	}
	return docToTx(&doc), nil
}

func (r *transactionRepository) ListByNode(ctx context.Context, nodeID string, txType *models.TransactionType) ([]*models.Transaction, error) {
	filter := bson.M{"node_id": nodeID}
	if txType != nil {
		filter["type"] = *txType
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("transactionRepository.ListByNode: %w", err)
	}
	defer cur.Close(ctx)

	var txs []*models.Transaction
	for cur.Next(ctx) {
		var doc transactionDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		txs = append(txs, docToTx(&doc))
	}
	return txs, cur.Err()
}

func (r *transactionRepository) ListByRef(ctx context.Context, refType models.TransactionRefType, refID string) ([]*models.Transaction, error) {
	cur, err := r.col.Find(ctx, bson.M{"ref_type": refType, "reference_id": refID})
	if err != nil {
		return nil, fmt.Errorf("transactionRepository.ListByRef: %w", err)
	}
	defer cur.Close(ctx)

	var txs []*models.Transaction
	for cur.Next(ctx) {
		var doc transactionDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		txs = append(txs, docToTx(&doc))
	}
	return txs, cur.Err()
}
