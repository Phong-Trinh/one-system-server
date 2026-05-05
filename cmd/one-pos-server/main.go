package main

import (
    "fmt"
    "log"
    "net/http"
    "one-system-server/internal/domain/models"
)

func main() {
    fmt.Println("Starting OnePos Server...")

    // Example initialization of domain models
    hq := models.Node{
        ID:   "hq-1",
        Type: models.NodeHQ,
        Name: "One System HQ",
    }

    log.Printf("Successfully initialized node: %s (%s)", hq.Name, hq.Type)

    // Placeholder for router setup
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Welcome to OnePos (One System) API")
    })

    port := ":8080"
    log.Printf("Server listening on port %s", port)
    if err := http.ListenAndServe(port, nil); err != nil {
        log.Fatal(err)
    }
}
