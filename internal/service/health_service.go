package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/health"
)

type HealthService struct {
	db *pgxpool.Pool
}

func NewHealthService(db *pgxpool.Pool) *HealthService {
	return &HealthService{db: db}
}

func (s *HealthService) Check(ctx context.Context) health.Report {
	report := health.Report{
		Status: health.StatusOK,
		Checks: make(map[string]health.CheckResult),
	}

	// Check postgres
	result := health.CheckResult{Status: health.StatusOK}
	if err := s.db.Ping(ctx); err != nil {
		result.Status = health.StatusFail
		result.Message = err.Error()
		report.Status = health.StatusFail
	}
	report.Checks["postgres"] = result

	return report
}
