package services

import (
	"context"
	"fmt"

	"github.com/alemancenter/fiber-api/internal/database"
	"gorm.io/gorm"
)

// modelTypeUser is the Laravel polymorphic type for users.
// Spatie's model_has_roles / model_has_permissions require this as a non-null column.
const modelTypeUser = "App\\Models\\User"

// AssignRoles replaces a user's roles in the polymorphic pivot table.
// GORM's many2many doesn't set model_type, so we use raw SQL inside a transaction.
func AssignRoles(db *gorm.DB, userID uint, roleIDs []uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"DELETE FROM model_has_roles WHERE model_id = ? AND model_type = ?",
			userID, modelTypeUser,
		).Error; err != nil {
			return err
		}
		for _, id := range roleIDs {
			if err := tx.Exec(
				"INSERT IGNORE INTO model_has_roles (role_id, model_type, model_id) VALUES (?, ?, ?)",
				id, modelTypeUser, userID,
			).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// AssignPermissions replaces a user's direct permissions in the polymorphic pivot table.
func AssignPermissions(db *gorm.DB, userID uint, permIDs []uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"DELETE FROM model_has_permissions WHERE model_id = ? AND model_type = ?",
			userID, modelTypeUser,
		).Error; err != nil {
			return err
		}
		for _, id := range permIDs {
			if err := tx.Exec(
				"INSERT IGNORE INTO model_has_permissions (permission_id, model_type, model_id) VALUES (?, ?, ?)",
				id, modelTypeUser, userID,
			).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ClearRoles removes all role assignments for a user.
func ClearRoles(db *gorm.DB, userID uint) error {
	return db.Exec(
		"DELETE FROM model_has_roles WHERE model_id = ? AND model_type = ?",
		userID, modelTypeUser,
	).Error
}

// ClearPermissions removes all direct permission assignments for a user.
func ClearPermissions(db *gorm.DB, userID uint) error {
	return db.Exec(
		"DELETE FROM model_has_permissions WHERE model_id = ? AND model_type = ?",
		userID, modelTypeUser,
	).Error
}

// InvalidateUserCache evicts the cached user entry from Redis.
// Call after any change to roles, permissions, status, or on logout.
func InvalidateUserCache(userID uint) {
	ctx := context.Background()
	rdb := database.Redis()
	_ = rdb.Del(ctx, rdb.Key("user", fmt.Sprintf("%d", userID)))
}
