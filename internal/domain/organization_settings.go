package domain

// Organization Settings sections and keys.
// Settings are stored in the existing preferences table with owner_type="organization".

const (
	OrgSettingOwnerType = "organization"

	// --- Section: general ---
	OrgSettingSectionGeneral = "general"
	OrgSettingKeyLogo        = "logo"
	OrgSettingKeyBusinessID  = "business_identifier"
	OrgSettingKeyTimezone    = "timezone"

	// --- Section: domains ---
	OrgSettingSectionDomains      = "domains"
	OrgSettingKeyClaimedDomains   = "claimed_domains"
	OrgSettingKeyApprovedEmails   = "approved_email_domains"
	OrgSettingKeyAutoJoinVerified = "auto_join_verified_domain"

	// --- Section: members ---
	OrgSettingSectionMembers          = "members"
	OrgSettingKeyDefaultRole          = "default_member_role"
	OrgSettingKeyAutoDeleteSuspended  = "auto_delete_suspended_days"
	OrgSettingKeyInvitationExpiryDays = "invitation_expiry_days"

	// --- Section: security ---
	OrgSettingSectionSecurity           = "security"
	OrgSettingKeyAllowed2FAMethods      = "allowed_2fa_methods"
	OrgSettingKeyBreachDetection        = "breach_detection_enabled"
	OrgSettingKeyVaultHealthReports     = "vault_health_reports_enabled"
	OrgSettingKeyEmergencyAccessEnabled = "emergency_access_enabled"

	// --- Section: audit ---
	OrgSettingSectionAudit        = "audit"
	OrgSettingKeyLogRetentionDays = "log_retention_days"
	OrgSettingKeyLogExportEnabled = "log_export_enabled"
	OrgSettingKeyComplianceMode   = "compliance_mode"

	// --- Section: notifications ---
	OrgSettingSectionNotifications        = "notifications"
	OrgSettingKeyAdminNotifications       = "admin_notifications_enabled"
	OrgSettingKeyMemberNotifications      = "member_notifications_enabled"
	OrgSettingKeyBreachAlertNotifications = "breach_alert_notifications_enabled"
	OrgSettingKeyWeeklySecurityReport     = "weekly_security_report_enabled"

	// --- Section: billing ---
	OrgSettingSectionBilling        = "billing"
	OrgSettingKeyBillingContact     = "billing_contact"
	OrgSettingKeySeatManagementMode = "seat_management_mode"
	OrgSettingKeyInvoicePONumber    = "invoice_po_number"
)

// OrgSettingsDefinition describes a single organization setting (not stored in DB)
type OrgSettingsDefinition struct {
	Section      string `json:"section"`
	Key          string `json:"key"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Type         string `json:"type"` // string, number, boolean, json
	DefaultValue string `json:"default_value"`
	Tier         string `json:"tier"` // "all", "team", "business", "enterprise"
}

// AllOrgSettingsDefinitions returns the full catalog of organization settings.
func AllOrgSettingsDefinitions() []OrgSettingsDefinition {
	return []OrgSettingsDefinition{
		// General
		{Section: OrgSettingSectionGeneral, Key: OrgSettingKeyLogo, Name: "Organization Logo", Description: "Logo URL for branding", Type: "string", DefaultValue: "", Tier: "business"},
		{Section: OrgSettingSectionGeneral, Key: OrgSettingKeyBusinessID, Name: "Business Identifier", Description: "Tax ID, VAT number, or company registration number", Type: "string", DefaultValue: "", Tier: "business"},
		{Section: OrgSettingSectionGeneral, Key: OrgSettingKeyTimezone, Name: "Timezone", Description: "Timezone for audit logs and reports", Type: "string", DefaultValue: "UTC", Tier: "all"},

		// Domains
		{Section: OrgSettingSectionDomains, Key: OrgSettingKeyClaimedDomains, Name: "Claimed Domains", Description: "JSON array of verified organization domains", Type: "json", DefaultValue: "[]", Tier: "business"},
		{Section: OrgSettingSectionDomains, Key: OrgSettingKeyApprovedEmails, Name: "Approved Email Domains", Description: "JSON array of approved email domains for members", Type: "json", DefaultValue: "[]", Tier: "business"},
		{Section: OrgSettingSectionDomains, Key: OrgSettingKeyAutoJoinVerified, Name: "Auto-join on Verified Domain", Description: "Automatically invite users registering with a verified domain", Type: "boolean", DefaultValue: "false", Tier: "enterprise"},

		// Members
		{Section: OrgSettingSectionMembers, Key: OrgSettingKeyDefaultRole, Name: "Default Member Role", Description: "Default role assigned to new members", Type: "string", DefaultValue: "member", Tier: "all"},
		{Section: OrgSettingSectionMembers, Key: OrgSettingKeyAutoDeleteSuspended, Name: "Auto-delete Suspended Users", Description: "Days after suspension before automatic deletion (0 = disabled)", Type: "number", DefaultValue: "0", Tier: "business"},
		{Section: OrgSettingSectionMembers, Key: OrgSettingKeyInvitationExpiryDays, Name: "Invitation Expiry", Description: "Days before invitation links expire", Type: "number", DefaultValue: "7", Tier: "all"},

		// Security
		{Section: OrgSettingSectionSecurity, Key: OrgSettingKeyAllowed2FAMethods, Name: "Allowed 2FA Methods", Description: "JSON array of allowed 2FA methods (totp, webauthn, duo)", Type: "json", DefaultValue: "[\"totp\",\"webauthn\"]", Tier: "business"},
		{Section: OrgSettingSectionSecurity, Key: OrgSettingKeyBreachDetection, Name: "Password Breach Detection", Description: "Check member passwords against breach databases", Type: "boolean", DefaultValue: "false", Tier: "team"},
		{Section: OrgSettingSectionSecurity, Key: OrgSettingKeyVaultHealthReports, Name: "Vault Health Reports", Description: "Enable weak, reused, and old password reporting", Type: "boolean", DefaultValue: "false", Tier: "team"},
		{Section: OrgSettingSectionSecurity, Key: OrgSettingKeyEmergencyAccessEnabled, Name: "Emergency Access", Description: "Allow trusted contacts to access vaults after waiting period", Type: "boolean", DefaultValue: "false", Tier: "business"},

		// Audit
		{Section: OrgSettingSectionAudit, Key: OrgSettingKeyLogRetentionDays, Name: "Activity Log Retention", Description: "Days to retain audit logs (0 = unlimited)", Type: "number", DefaultValue: "90", Tier: "business"},
		{Section: OrgSettingSectionAudit, Key: OrgSettingKeyLogExportEnabled, Name: "Audit Log Export", Description: "Allow SIEM export of audit logs (CSV, JSON, Syslog)", Type: "boolean", DefaultValue: "false", Tier: "enterprise"},
		{Section: OrgSettingSectionAudit, Key: OrgSettingKeyComplianceMode, Name: "Compliance Mode", Description: "Compliance framework (none, hipaa, soc2, gdpr)", Type: "string", DefaultValue: "none", Tier: "enterprise"},

		// Notifications
		{Section: OrgSettingSectionNotifications, Key: OrgSettingKeyAdminNotifications, Name: "Admin Notifications", Description: "Email admins on new members, removals, and security events", Type: "boolean", DefaultValue: "true", Tier: "all"},
		{Section: OrgSettingSectionNotifications, Key: OrgSettingKeyMemberNotifications, Name: "Member Notifications", Description: "Email members on policy changes and vault warnings", Type: "boolean", DefaultValue: "true", Tier: "all"},
		{Section: OrgSettingSectionNotifications, Key: OrgSettingKeyBreachAlertNotifications, Name: "Breach Alert Notifications", Description: "Notify when data breaches are detected", Type: "boolean", DefaultValue: "false", Tier: "team"},
		{Section: OrgSettingSectionNotifications, Key: OrgSettingKeyWeeklySecurityReport, Name: "Weekly Security Report", Description: "Send weekly security summary to admins", Type: "boolean", DefaultValue: "false", Tier: "business"},

		// Billing
		{Section: OrgSettingSectionBilling, Key: OrgSettingKeyBillingContact, Name: "Billing Contact", Description: "JSON with billing contact details", Type: "json", DefaultValue: "{}", Tier: "all"},
		{Section: OrgSettingSectionBilling, Key: OrgSettingKeySeatManagementMode, Name: "Seat Management", Description: "Seat addition mode: auto or manual_approval", Type: "string", DefaultValue: "auto", Tier: "business"},
		{Section: OrgSettingSectionBilling, Key: OrgSettingKeyInvoicePONumber, Name: "Invoice PO Number", Description: "Purchase order number for invoices", Type: "string", DefaultValue: "", Tier: "business"},
	}
}
