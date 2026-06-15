package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/usecase"
)

const collOrders = "orders"

type orderItemDoc struct {
	ItemID   string  `bson:"item_id"`
	Quantity int     `bson:"quantity"`
	Price    float64 `bson:"price"`
}

type orderDoc struct {
	ID          string               `bson:"_id"`
	NodeID      string               `bson:"node_id"`
	Source      models.OrderSource   `bson:"source"`
	Platform    *string              `bson:"platform,omitempty"`
	Status      models.OrderStatus   `bson:"status"`
	TotalAmount float64              `bson:"total_amount"`
	Items       []orderItemDoc       `bson:"items"`
	DeadlineAt  *time.Time           `bson:"deadline_at,omitempty"`
	CreatedAt   time.Time            `bson:"created_at"`
	UpdatedAt   time.Time            `bson:"updated_at"`
}

func orderToDoc(o *models.Order) *orderDoc {
	items := make([]orderItemDoc, len(o.Items))
	for i, it := range o.Items {
		items[i] = orderItemDoc{ItemID: it.ItemID, Quantity: it.Quantity, Price: it.Price}
	}
	return &orderDoc{
		ID:          o.ID,
		NodeID:      o.NodeID,
		Source:      o.Source,
		Platform:    o.Platform,
		Status:      o.Status,
		TotalAmount: o.TotalAmount,
		Items:       items,
		DeadlineAt:  o.DeadlineAt,
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
	}
}

func docToOrder(d *orderDoc) *models.Order {
	items := make([]models.OrderItem, len(d.Items))
	for i, it := range d.Items {
		items[i] = models.OrderItem{ItemID: it.ItemID, Quantity: it.Quantity, Price: it.Price}
	}
	return &models.Order{
		ID:          d.ID,
		NodeID:      d.NodeID,
		Source:      d.Source,
		Platform:    d.Platform,
		Status:      d.Status,
		TotalAmount: d.TotalAmount,
		Items:       items,
		DeadlineAt:  d.DeadlineAt,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

type orderRepository struct {
	col *mongo.Collection
}

func NewOrderRepository(client *Client, dbName string) usecase.OrderRepository {
	col := client.DB(dbName).Collection(collOrders)
	return &orderRepository{col: col}
}

func (r *orderRepository) Create(ctx context.Context, order *models.Order) error {
	_, err := r.col.InsertOne(ctx, orderToDoc(order))
	if err != nil {
		return fmt.Errorf("orderRepository.Create: %w", err)
	}
	return nil
}

func (r *orderRepository) FindByID(ctx context.Context, id string) (*models.Order, error) {
	var doc orderDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("orderRepository.FindByID: %w", err)
	}
	return docToOrder(&doc), nil
}

func (r *orderRepository) FindByNode(ctx context.Context, nodeID string) ([]*models.Order, error) {
	cur, err := r.col.Find(ctx, bson.M{"node_id": nodeID})
	if err != nil {
		return nil, fmt.Errorf("orderRepository.FindByNode: %w", err)
	}
	defer cur.Close(ctx)

	var orders []*models.Order
	for cur.Next(ctx) {
		var doc orderDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		orders = append(orders, docToOrder(&doc))
	}
	return orders, cur.Err()
}

func (r *orderRepository) UpdateStatus(ctx context.Context, id string, status models.OrderStatus) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"status": status, "updated_at": time.Now()}},
	)
	if err != nil {
		return fmt.Errorf("orderRepository.UpdateStatus: %w", err)
	}
	return nil
}
