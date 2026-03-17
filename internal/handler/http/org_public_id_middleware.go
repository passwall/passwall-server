package http

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/pkg/constants"
)

// OrgPublicIDResolver resolves an organization public_id to a domain object.
type OrgPublicIDResolver interface {
	GetByPublicID(ctx context.Context, publicID string) (*domain.Organization, error)
}

// OrgPublicIDResolverMiddleware reads the ":id" route parameter as a short
// alphanumeric public_id, resolves it to the numeric organization ID via a
// DB lookup, and stores the result in the gin context so downstream handlers
// and middleware can retrieve it with GetResolvedOrgID.
func OrgPublicIDResolverMiddleware(resolver OrgPublicIDResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		publicID := c.Param("id")
		if publicID == "" {
			c.Next()
			return
		}

		if len(publicID) != domain.PublicIDLength {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization identifier"})
			c.Abort()
			return
		}

		org, err := resolver.GetByPublicID(c.Request.Context(), publicID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			c.Abort()
			return
		}

		c.Set(constants.ContextKeyOrgID, org.ID)
		c.Next()
	}
}
