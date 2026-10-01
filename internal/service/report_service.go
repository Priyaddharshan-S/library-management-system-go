package service

import (
	"context"

	"library-system/internal/models"
	"library-system/internal/repository"
)

type ReportService struct {
	reportRepo repository.ReportRepository
}

func NewReportService(reportRepo repository.ReportRepository) *ReportService {
	return &ReportService{reportRepo: reportRepo}
}

func (s *ReportService) GetSummary(ctx context.Context) (*models.ReportSummary, error) {
	return s.reportRepo.GetSummary(ctx)
}
