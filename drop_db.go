package main

import (
	"context"
	"fmt"
	"time"

	mongorepo "one-system-server/internal/infrastructure/persistence/mongodb"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongorepo.NewClient(ctx, "mongodb://localhost:27017")
	if err != nil {
		panic(err)
	}

	err = client.DB("one_system").Drop(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Println("Database dropped successfully.")
}
