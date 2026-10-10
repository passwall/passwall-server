package http

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// Clients identify themselves so the server can roll out behavior changes
// without breaking apps that are already installed:
//
//	X-Passwall-Client: <app>/<version>          e.g. "vault/1.72.0"
//	X-Passwall-Crypto-Caps: <cap>[,<cap>...]    e.g. "hidepw-client"
const (
	ClientHeader     = "X-Passwall-Client"
	CryptoCapsHeader = "X-Passwall-Crypto-Caps"
)

// Client capabilities.
const (
	// CapHidePasswordsClient: the client hides passwords itself when an item's
	// permissions deny view_password, so the server may send the encrypted
	// data (needed for autofill) instead of withholding it.
	CapHidePasswordsClient = "hidepw-client"
	// CapItemPermissions: the client accepts the "permissions" object on
	// organization item sync responses. Shipped Vault builds validate that
	// response strictly and reject unknown fields, so it is opt-in.
	CapItemPermissions = "item-permissions"
)

// ClientHasCapability reports whether the request's client declared cap.
func ClientHasCapability(c *gin.Context, capability string) bool {
	for _, value := range strings.Split(c.GetHeader(CryptoCapsHeader), ",") {
		if strings.EqualFold(strings.TrimSpace(value), capability) {
			return true
		}
	}
	return false
}
