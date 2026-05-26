package services

import (
	"context"

	"one-system-server/internal/domain/models"
)

// EquipmentTypeRepository defines persistence operations for EquipmentType.
type EquipmentTypeRepository interface {
	Create(ctx context.Context, st *models.EquipmentType) error
	FindByID(ctx context.Context, id string) (*models.EquipmentType, error)
	FindAll(ctx context.Context) ([]*models.EquipmentType, error)
	Update(ctx context.Context, st *models.EquipmentType) error
	Delete(ctx context.Context, id string) error
}

// MachineRepository defines persistence operations for Machine.
type MachineRepository interface {
	Create(ctx context.Context, m *models.Machine) error
	FindByID(ctx context.Context, id string) (*models.Machine, error)
	FindByNodeID(ctx context.Context, nodeID string) ([]*models.Machine, error)
	FindAll(ctx context.Context) ([]*models.Machine, error)
	// FindIdleByStationType returns all IDLE machines of a given station type at a node.
	// Used by the bin-packing allocation engine (Layer 5).
	FindIdleByStationType(ctx context.Context, nodeID, stationTypeID string) ([]*models.Machine, error)
	UpdateStatus(ctx context.Context, id string, status models.MachineStatus, batchID *string) error
	Update(ctx context.Context, m *models.Machine) error
	Delete(ctx context.Context, id string) error
}

// StaffRepository defines persistence operations for Staff.
type StaffRepository interface {
	Create(ctx context.Context, s *models.Staff) error
	FindByID(ctx context.Context, id string) (*models.Staff, error)
	FindByNodeID(ctx context.Context, nodeID string) ([]*models.Staff, error)
	Update(ctx context.Context, s *models.Staff) error
	Delete(ctx context.Context, id string) error
}
