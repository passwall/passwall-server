package database

import "context"

type contextKey string

const (
	// SchemaKey is the context key for database schema
	SchemaKey contextKey = "db_schema"
)

// WithSchema returns a new context with the schema value
func WithSchema(ctx context.Context, schema string) context.Context {
	return context.WithValue(ctx, SchemaKey, schema)
}

// GetSchema extracts the schema from context
// Returns empty string if not found (will use default schema)
func GetSchema(ctx context.Context) string {
	schema, ok := ctx.Value(SchemaKey).(string)
	if !ok {
		return "public" // Default schema
	}
	return schema
}

// MustGetSchema extracts the schema from context
// Panics if not found - use in handlers after auth middleware
func MustGetSchema(ctx context.Context) string {
	schema, ok := ctx.Value(SchemaKey).(string)
	if !ok || schema == "" {
		panic("schema not found in context - ensure auth middleware is applied")
	}
	return schema
}
