package repository

import (
	"context"
	"dialectrelease/internal/domain"
)

type Store interface {
	Create(context.Context, *domain.Aggregate, domain.AuditEvent, string) error
	Update(context.Context, *domain.Aggregate, int64, domain.AuditEvent, string) error
	Get(context.Context, string) (*domain.Aggregate, error)
	FindIdempotent(context.Context, string) (*domain.Aggregate, bool, error)
	Events(context.Context, string) ([]domain.AuditEvent, error)
	Close() error
}
