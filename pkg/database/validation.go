package database

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// schemaNameRegex validates PostgreSQL schema names
	// Must start with a letter, followed by letters, numbers, or underscores
	schemaNameRegex = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

	// Reserved PostgreSQL schema prefixes and names
	reservedSchemaPrefixes = []string{
		"pg_",
		"information_schema",
		"pg_catalog",
		"pg_toast",
		"pg_temp",
	}

	// Additional reserved schema names
	reservedSchemaNames = []string{
		"information_schema",
		"pg_catalog",
		"pg_toast",
	}
)

// ValidateSchemaName validates schema name format according to PostgreSQL identifier rules
// Returns error if schema name is invalid or contains potentially dangerous characters
func ValidateSchemaName(schema string) error {
	if schema == "" {
		return fmt.Errorf("schema name cannot be empty")
	}

	// "public" is always valid
	if schema == "public" {
		return nil
	}

	// Length check (PostgreSQL identifier max length is 63 bytes)
	if len(schema) > 63 {
		return fmt.Errorf("schema name too long (max 63 characters)")
	}

	// Format check: lowercase alphanumeric + underscore, must start with letter
	// This is more restrictive than PostgreSQL allows, but safer
	if !schemaNameRegex.MatchString(schema) {
		return fmt.Errorf("invalid schema name format: must start with lowercase letter and contain only lowercase letters, numbers, and underscores")
	}

	// Check for reserved prefixes
	for _, prefix := range reservedSchemaPrefixes {
		if strings.HasPrefix(schema, prefix) {
			return fmt.Errorf("schema name cannot start with reserved prefix: %s", prefix)
		}
	}

	// Check for exact reserved names
	for _, reserved := range reservedSchemaNames {
		if schema == reserved {
			return fmt.Errorf("schema name is reserved: %s", reserved)
		}
	}

	return nil
}

// SanitizeIdentifier safely quotes PostgreSQL identifiers to prevent SQL injection
// This function properly escapes quotes within the identifier
func SanitizeIdentifier(identifier string) string {
	// Escape any existing quotes by doubling them (PostgreSQL standard)
	escaped := strings.ReplaceAll(identifier, `"`, `""`)
	// Wrap in double quotes to make it a safe identifier
	return fmt.Sprintf(`"%s"`, escaped)
}

// ValidateTableName validates table name format
// Uses same rules as schema names for consistency
func ValidateTableName(tableName string) error {
	if tableName == "" {
		return fmt.Errorf("table name cannot be empty")
	}

	if len(tableName) > 63 {
		return fmt.Errorf("table name too long (max 63 characters)")
	}

	if !schemaNameRegex.MatchString(tableName) {
		return fmt.Errorf("invalid table name format: must start with lowercase letter and contain only lowercase letters, numbers, and underscores")
	}

	return nil
}

// ValidateOrderDirection validates SQL ORDER BY direction
// Only allows ASC and DESC (case-insensitive)
func ValidateOrderDirection(direction string) error {
	upper := strings.ToUpper(direction)
	if upper != "ASC" && upper != "DESC" {
		return fmt.Errorf("invalid order direction: must be ASC or DESC")
	}
	return nil
}

// IsAllowedSortColumn checks if a column name is in the allowed whitelist
// This prevents ORDER BY injection attacks
func IsAllowedSortColumn(column string, allowedColumns []string) bool {
	for _, allowed := range allowedColumns {
		if column == allowed {
			return true
		}
	}
	return false
}

// BuildQualifiedTableName builds a safe schema.table_name string
// Validates both schema and table name, then properly quotes them
func BuildQualifiedTableName(schema, tableName string) (string, error) {
	if err := ValidateSchemaName(schema); err != nil {
		return "", fmt.Errorf("invalid schema: %w", err)
	}

	if err := ValidateTableName(tableName); err != nil {
		return "", fmt.Errorf("invalid table name: %w", err)
	}

	// Build qualified name with proper quoting
	safeSchema := SanitizeIdentifier(schema)
	safeTable := SanitizeIdentifier(tableName)

	return fmt.Sprintf("%s.%s", safeSchema, safeTable), nil
}

