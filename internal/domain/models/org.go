package models

import "time"

type NodeType string

const (
    NodeHQ      NodeType = "HQ"
    NodeStore   NodeType = "STORE"
    NodeFactory NodeType = "FACTORY"
    NodeRefactor NodeType = "REFACTOR"
)

type Organization struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type Node struct {
    ID        string    `json:"id"`
    OrgID     string    `json:"org_id"`
    Type      NodeType  `json:"type"`
    Name      string    `json:"name"`
    Address   string    `json:"address"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
