package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/service"
)

func respondEntitlementError(c *gin.Context, err error) bool {
	var denied *service.EntitlementDeniedError
	isEntitlementError := errors.As(err, &denied) ||
		errors.Is(err, service.ErrAccountFrozen) ||
		errors.Is(err, service.ErrSubscriptionExpired) ||
		errors.Is(err, service.ErrPlanLimitReached) ||
		errors.Is(err, service.ErrFeatureNotAvailable)
	if !isEntitlementError {
		return false
	}

	code := "FEATURE_NOT_AVAILABLE"
	message := "feature not available in current plan"
	switch {
	case errors.Is(err, service.ErrAccountFrozen):
		code = "ACCOUNT_FROZEN"
		message = "account is frozen"
	case errors.Is(err, service.ErrSubscriptionExpired):
		code = "SUBSCRIPTION_EXPIRED"
		message = "subscription expired"
	case errors.Is(err, service.ErrPlanLimitReached):
		code = "PLAN_LIMIT_REACHED"
		message = "plan limit reached"
	}
	response := gin.H{
		"error": message,
		"code":  code,
	}
	if denied != nil {
		response["capability"] = denied.Capability
		response["reason"] = denied.Reason
	}
	c.JSON(http.StatusForbidden, response)
	return true
}
