package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"library-system/internal/service"
)

type ReportController struct {
	reportService *service.ReportService
}

func NewReportController(reportService *service.ReportService) *ReportController {
	return &ReportController{reportService: reportService}
}

func (ctl *ReportController) Summary(c *gin.Context) {
	summary, err := ctl.reportService.GetSummary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build report"})
		return
	}
	c.JSON(http.StatusOK, summary)
}
