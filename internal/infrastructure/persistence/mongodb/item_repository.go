package mongodb

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

const collItems = "items"

type itemDoc struct {
	ID       string          `bson:"_id"`
	OrgID    string          `bson:"org_id"`
	Name     string          `bson:"name"`
	SKU      string          `bson:"sku"`
	Type     models.ItemType `bson:"type"`
	BaseUnit string          `bson:"base_unit"`
}

func itemToDoc(item *models.Item) *itemDoc {
	return &itemDoc{
		ID:       item.ID,
		OrgID:    item.OrgID,
		Name:     item.Name,
		SKU:      item.SKU,
		Type:     item.Type,
		BaseUnit: item.BaseUnit,
	}
}

func docToItem(d *itemDoc) *models.Item {
	return &models.Item{
		ID:       d.ID,
		OrgID:    d.OrgID,
		Name:     d.Name,
		SKU:      d.SKU,
		Type:     d.Type,
		BaseUnit: d.BaseUnit,
	}
}

type itemRepository struct {
	col *mongo.Collection
}

func NewItemRepository(client *Client, dbName string) services.ItemRepository {
	col := client.DB(dbName).Collection(collItems)
	return &itemRepository{col: col}
}

func (r *itemRepository) Create(ctx context.Context, item *models.Item) error {
	_, err := r.col.InsertOne(ctx, itemToDoc(item))
	if err != nil {
		return fmt.Errorf("itemRepository.Create: %w", err)
	}
	return nil
}

func (r *itemRepository) FindByID(ctx context.Context, id string) (*models.Item, error) {
	var doc itemDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("itemRepository.FindByID: %w", err)
	}
	return docToItem(&doc), nil
}

func (r *itemRepository) FindAll(ctx context.Context) ([]*models.Item, error) {
	cur, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("itemRepository.FindAll: %w", err)
	}
	defer cur.Close(ctx)
	
	var items []*models.Item
	for cur.Next(ctx) {
		var doc itemDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		items = append(items, docToItem(&doc))
	}
	return items, cur.Err()
}

func (r *itemRepository) FindByOrgID(ctx context.Context, orgID string) ([]*models.Item, error) {
	cur, err := r.col.Find(ctx, bson.M{"org_id": orgID})
	if err != nil {
		return nil, fmt.Errorf("itemRepository.FindByOrgID: %w", err)
	}
	defer cur.Close(ctx)

	var items []*models.Item
	for cur.Next(ctx) {
		var doc itemDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		items = append(items, docToItem(&doc))
	}
	return items, cur.Err()
}

func (r *itemRepository) Update(ctx context.Context, item *models.Item) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": item.ID}, itemToDoc(item))
	if err != nil {
		return fmt.Errorf("itemRepository.Update: %w", err)
	}
	return nil
}

func (r *itemRepository) Delete(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("itemRepository.Delete: %w", err)
	}
	return nil
}
