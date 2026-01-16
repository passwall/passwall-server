package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
)

type PlanHandler struct {
	planRepo interface {
		List(ctx context.Context) ([]*domain.Plan, error)
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	}
	logger interface {
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
	}
}

// NewPlanHandler creates a new plan handler
func NewPlanHandler(
	planRepo interface {
		List(ctx context.Context) ([]*domain.Plan, error)
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	},
	logger interface {
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
	},
) *PlanHandler {
	return &PlanHandler{
		planRepo: planRepo,
		logger:   logger,
	}
}

// ListPlans retrieves all active plans
// GET /api/plans
func (h *PlanHandler) ListPlans(c *gin.Context) {
	ctx := c.Request.Context()

	plans, err := h.planRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list plans", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve plans"})
		return
	}

	// Convert to DTOs
	dtos := make([]*domain.PlanDTO, len(plans))
	for i, plan := range plans {
		dtos[i] = domain.ToPlanDTO(plan)
	}

	c.JSON(http.StatusOK, dtos)
}

// GetPlan retrieves a plan by code
// GET /api/plans/:code
func (h *PlanHandler) GetPlan(c *gin.Context) {
	ctx := c.Request.Context()

	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan code required"})
		return
	}

	plan, err := h.planRepo.GetByCode(ctx, code)
	if err != nil {
		h.logger.Error("failed to get plan", "code", code, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "plan not found"})
		return
	}

	c.JSON(http.StatusOK, domain.ToPlanDTO(plan))
}
