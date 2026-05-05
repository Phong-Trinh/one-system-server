package models

import "time"

type SupplyRequestStatus string

const (
    ReqStatusPending   SupplyRequestStatus = "PENDING"
    ReqStatusApproved  SupplyRequestStatus = "APPROVED"
    ReqStatusRejected  SupplyRequestStatus = "REJECTED"
    ReqStatusFulfilled SupplyRequestStatus = "FULFILLED"
)

type SupplyRequest struct {
    ID              string           `json:"id"`
    RequesterNodeID string           `json:"requester_node_id"`
    ProviderNodeID  string           `json:"provider_node_id"`
    Items           []SupplyItem     `json:"items"`
    Status          SupplyRequestStatus `json:"status"`
    CreatedAt       time.Time        `json:"created_at"`
    UpdatedAt       time.Time        `json:"updated_at"`
}

type SupplyItem struct {
    ItemID   string  `json:"item_id"`
    Quantity float64 `json:"quantity"`
}
