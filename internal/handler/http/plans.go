package http

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/service"
)

// PlansHandler exposes subscription plans for authenticated clients.
// NOTE: This is intentionally in the http handler package (to match router wiring).
type PlansHandler struct {
	planRepo interface {
		List(ctx context.Context) ([]*domain.Plan, error)
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	}
	logger service.Logger
}

func NewPlansHandler(
	planRepo interface {
		List(ctx context.Context) ([]*domain.Plan, error)
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	},
	logger service.Logger,
) *PlansHandler {
	return &PlansHandler{
		planRepo: planRepo,
		logger:   logger,
	}
}

// ListPlans returns all active plans.
// GET /api/plans
func (h *PlansHandler) ListPlans(c *gin.Context) {
	ctx := c.Request.Context()

	plans, err := h.planRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list plans", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve plans"})
		return
	}

	dtos := make([]*domain.PlanDTO, 0, len(plans))
	for _, plan := range plans {
		dtos = append(dtos, domain.ToPlanDTO(plan))
	}

	c.JSON(http.StatusOK, dtos)
}

// GetPlan returns a plan by code.
// GET /api/plans/:code
func (h *PlansHandler) GetPlan(c *gin.Context) {
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

