package gormrepo

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/pkg/database"
	"gorm.io/gorm"
)

// AdminDirectoryStatsRepository loads count-only usage signals for the admin directory.
// Queries select counts and timestamps. They do not select item payloads, metadata,
// titles, tokens, device ids, or client names.
type AdminDirectoryStatsRepository struct {
	db *gorm.DB
}

// NewAdminDirectoryStatsRepository creates the directory stats repository.
func NewAdminDirectoryStatsRepository(db *gorm.DB) *AdminDirectoryStatsRepository {
	return &AdminDirectoryStatsRepository{db: db}
}

// LoadAccountSignals returns one signal record per requested user id.
func (r *AdminDirectoryStatsRepository) LoadAccountSignals(ctx context.Context, users []*domain.User, now time.Time) (map[uint]domain.AdminDirectorySignals, error) {
	out := make(map[uint]domain.AdminDirectorySignals)
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("admin directory stats repository is not configured")
	}

	ids := make([]uint, 0, len(users))
	seen := make(map[uint]struct{}, len(users))
	for _, user := range users {
		if user == nil || user.ID == 0 {
			continue
		}
		if _, ok := seen[user.ID]; ok {
			continue
		}
		seen[user.ID] = struct{}{}
		ids = append(ids, user.ID)
		out[user.ID] = domain.AdminDirectorySignals{}
	}
	if len(ids) == 0 {
		return out, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	if err := r.fillActivity(ctx, ids, now, out); err != nil {
		return nil, err
	}
	if err := r.fillTokens(ctx, ids, now, out); err != nil {
		return nil, err
	}
	if err := r.fillOrganizationItems(ctx, ids, out); err != nil {
		return nil, err
	}
	if err := r.fillMembershipCounts(ctx, ids, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AdminDirectoryStatsRepository) fillActivity(ctx context.Context, userIDs []uint, now time.Time, out map[uint]domain.AdminDirectorySignals) error {
	cutoff := now.Add(-domain.UserActivityRetention)
	type row struct {
		UserID         uint         `gorm:"column:user_id"`
		LastActivityAt nullableTime `gorm:"column:last_activity_at"`
		SignInCount    int64        `gorm:"column:signin_count"`
		ActivityCount  int64        `gorm:"column:activity_count"`
	}
	var rows []row
	err := dbFromContext(ctx, r.db).
		Model(&domain.UserActivity{}).
		Select(
			"user_id, MAX(CASE WHEN activity_type <> ? THEN created_at END) AS last_activity_at, COUNT(CASE WHEN activity_type = ? THEN 1 END) AS signin_count, COUNT(*) AS activity_count",
			domain.ActivityTypeSignIn,
			domain.ActivityTypeSignIn,
		).
		Where("user_id IN ? AND created_at >= ?", userIDs, cutoff).
		Group("user_id").
		Scan(&rows).Error
	if err != nil {
		return fmt.Errorf("count account activity: %w", err)
	}
	for _, row := range rows {
		sig := out[row.UserID]
		if row.LastActivityAt.Time != nil && !row.LastActivityAt.Time.IsZero() {
			ts := row.LastActivityAt.Time.UTC()
			sig.LastActivityAt = &ts
		}
		sig.SignInCount = intFromCount(row.SignInCount)
		sig.ActivityCount = intFromCount(row.ActivityCount)
		out[row.UserID] = sig
	}
	return nil
}

func (r *AdminDirectoryStatsRepository) fillTokens(ctx context.Context, userIDs []uint, now time.Time, out map[uint]domain.AdminDirectorySignals) error {
	type row struct {
		UserID      int   `gorm:"column:user_id"`
		DeviceCount int64 `gorm:"column:device_count"`
		ClientCount int64 `gorm:"column:client_count"`
	}
	var rows []row
	// device_id and app are used only inside aggregates. Their values are not selected.
	err := dbFromContext(ctx, r.db).
		Model(&domain.Token{}).
		Select(
			"user_id, COUNT(DISTINCT CASE WHEN device_id IS NOT NULL AND CAST(device_id AS TEXT) NOT IN (?, '') THEN CAST(device_id AS TEXT) END) AS device_count, COUNT(DISTINCT CASE WHEN app IS NOT NULL AND app <> '' THEN app END) AS client_count",
			uuid.Nil.String(),
		).
		Where("user_id IN ? AND expiry_time > ?", intIDs(userIDs), now).
		Group("user_id").
		Scan(&rows).Error
	if err != nil {
		return fmt.Errorf("count account tokens: %w", err)
	}
	for _, row := range rows {
		if row.UserID <= 0 {
			continue
		}
		id := uint(row.UserID)
		sig := out[id]
		sig.DeviceCount = intFromCount(row.DeviceCount)
		sig.ClientCount = intFromCount(row.ClientCount)
		out[id] = sig
	}
	return nil
}

func (r *AdminDirectoryStatsRepository) fillOrganizationItems(ctx context.Context, userIDs []uint, out map[uint]domain.AdminDirectorySignals) error {
	rows, err := r.countItemsByType(ctx, &domain.OrganizationItem{}, "created_by_user_id", "created_by_user_id IN ? AND deleted_at IS NULL", userIDs)
	if err != nil {
		return fmt.Errorf("count organization items: %w", err)
	}
	for _, row := range rows {
		sig := out[row.UserID]
		if sig.OrgItemsByType == nil {
			sig.OrgItemsByType = make(map[domain.ItemType]int)
		}
		sig.OrgItemsByType[domain.ItemType(row.ItemType)] += intFromCount(row.ItemCount)
		out[row.UserID] = sig
	}
	return nil
}

func (r *AdminDirectoryStatsRepository) fillMembershipCounts(ctx context.Context, userIDs []uint, out map[uint]domain.AdminDirectorySignals) error {
	type row struct {
		UserID            uint  `gorm:"column:user_id"`
		OrganizationCount int64 `gorm:"column:organization_count"`
		CollectionCount   int64 `gorm:"column:collection_count"`
	}
	statuses := []string{
		string(domain.OrgUserStatusAccepted),
		string(domain.OrgUserStatusConfirmed),
		string(domain.OrgUserStatusSuspended),
	}
	var rows []row
	err := dbFromContext(ctx, r.db).
		Table("organization_users AS ou").
		Select("ou.user_id AS user_id, COUNT(DISTINCT ou.organization_id) AS organization_count, COUNT(DISTINCT c.id) AS collection_count").
		Joins("JOIN organizations AS o ON o.id = ou.organization_id AND o.deleted_at IS NULL").
		Joins("LEFT JOIN collections AS c ON c.organization_id = o.id AND c.deleted_at IS NULL").
		Where("ou.user_id IN ? AND ou.status IN ?", userIDs, statuses).
		Group("ou.user_id").
		Scan(&rows).Error
	if err != nil {
		return fmt.Errorf("count organizations: %w", err)
	}
	for _, row := range rows {
		sig := out[row.UserID]
		sig.OrganizationCount = intFromCount(row.OrganizationCount)
		sig.CollectionCount = intFromCount(row.CollectionCount)
		out[row.UserID] = sig
	}
	return nil
}

type itemTypeCountRow struct {
	UserID    uint  `gorm:"column:user_id"`
	ItemType  int64 `gorm:"column:item_type"`
	ItemCount int64 `gorm:"column:item_count"`
}

func (r *AdminDirectoryStatsRepository) countItemsByType(ctx context.Context, model interface{}, userColumn string, where string, userIDs []uint) ([]itemTypeCountRow, error) {
	if err := database.ValidateTableName(userColumn); err != nil {
		return nil, fmt.Errorf("invalid user column: %w", err)
	}
	var rows []itemTypeCountRow
	err := dbFromContext(ctx, r.db).
		Model(model).
		Select(userColumn+" AS user_id, item_type, COUNT(*) AS item_count").
		Where(where, userIDs).
		Group(userColumn + ", item_type").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func intIDs(ids []uint) []int {
	out := make([]int, len(ids))
	for i, id := range ids {
		out[i] = int(id)
	}
	return out
}

func intFromCount(n int64) int {
	if n <= 0 {
		return 0
	}
	return int(n)
}

// nullableTime scans a timestamp from either PostgreSQL (time.Time) or SQLite (string).
// Value is implemented so GORM treats the field as a column rather than a relation.
type nullableTime struct {
	Time *time.Time
}

func (n nullableTime) Value() (driver.Value, error) {
	if n.Time == nil || n.Time.IsZero() {
		return nil, nil
	}
	return *n.Time, nil
}

func (n *nullableTime) Scan(value interface{}) error {
	if n == nil {
		return fmt.Errorf("nullableTime is nil")
	}
	n.Time = nil
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case time.Time:
		ts := typed
		n.Time = &ts
		return nil
	case *time.Time:
		if typed == nil {
			return nil
		}
		ts := *typed
		n.Time = &ts
		return nil
	case string:
		return n.parse(typed)
	case []byte:
		return n.parse(string(typed))
	default:
		return fmt.Errorf("cannot scan %T into time", value)
	}
}

func (n *nullableTime) parse(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, raw)
		if err != nil {
			continue
		}
		n.Time = &parsed
		return nil
	}
	return fmt.Errorf("cannot parse time %q", raw)
}
