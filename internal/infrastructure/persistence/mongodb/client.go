package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Client wraps the MongoDB client and exposes a helper to get a collection.
type Client struct {
	inner *mongo.Client
}

// NewClient connects to MongoDB Atlas and pings the deployment to verify connectivity.
func NewClient(ctx context.Context, uri string) (*Client, error) {
	opts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongodb: connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongodb: ping: %w", err)
	}
	return &Client{inner: client}, nil
}

// DB returns the mongo.Database for the given name.
func (c *Client) DB(name string) *mongo.Database {
	return c.inner.Database(name)
}

// Disconnect cleanly closes the connection pool.
func (c *Client) Disconnect(ctx context.Context) error {
	return c.inner.Disconnect(ctx)
}
