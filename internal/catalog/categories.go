package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func categoryStorage(err error) error {
	return &dto.AppError{
		Code:    "STORAGE",
		Message: "Failed to save or read category. Please retry.",
		Details: err.Error(),
	}
}

func (s *Service) ListCategories(ctx context.Context) ([]dto.Category, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT c.id,c.name,c.position,COUNT(DISTINCT a.id)
 FROM categories c LEFT JOIN assets a ON a.category_id=c.id GROUP BY c.id,c.name,c.position ORDER BY c.position,c.id`)
	if err != nil {
		return nil, categoryStorage(err)
	}

	defer rows.Close()

	result := make([]dto.Category, 0)
	for rows.Next() {
		var c dto.Category
		if err = rows.Scan(&c.ID, &c.Name, &c.Position, &c.Count); err != nil {
			return nil, categoryStorage(err)
		}

		result = append(result, c)
	}

	if err = rows.Err(); err != nil {
		return nil, categoryStorage(err)
	}

	return result, nil
}

func (s *Service) SaveCategory(ctx context.Context, d dto.CategoryDraft) (dto.Category, error) {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" || d.Name == "All assets" || d.Name == "Uncategorized" || d.Name == "全部素材" || d.Name == "未分类" || d.Position < 0 {
		return dto.Category{}, dto.Error("VALIDATION", "Enter a valid category name and order. System view names cannot be used.")
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return dto.Category{}, categoryStorage(err)
	}

	defer tx.Rollback()

	var oldName string
	if d.ID != "" {
		if err = tx.QueryRowContext(ctx, "SELECT name FROM categories WHERE id=?", d.ID).Scan(&oldName); errors.Is(err, sql.ErrNoRows) {
			return dto.Category{}, dto.Error("NOT_FOUND", "Category not found")
		} else if err != nil {
			return dto.Category{}, categoryStorage(err)
		}
	}

	key := strings.ToLower(d.Name)

	var owner string

	err = tx.QueryRowContext(ctx, "SELECT id FROM categories WHERE name_key=?", key).Scan(&owner)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return dto.Category{}, categoryStorage(err)
	}

	if err == nil && owner != d.ID {
		return dto.Category{}, dto.Error("VALIDATION", "Category name already exists")
	}

	if d.ID == "" {
		d.ID = uuid.NewString()
		_, err = tx.ExecContext(ctx, "INSERT INTO categories(id,name,name_key,position) VALUES(?,?,?,?)", d.ID, d.Name, key, d.Position)
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE categories SET name=?,name_key=?,position=? WHERE id=?", d.Name, key, d.Position, d.ID)
		if err == nil && oldName != d.Name {
			_, err = tx.ExecContext(
				ctx,
				"UPDATE assets SET revision=revision+1,updated_at=? WHERE category_id=?",
				time.Now().UTC().Format(time.RFC3339Nano),
				d.ID,
			)
		}
	}

	if err != nil {
		return dto.Category{}, categoryStorage(err)
	}

	out := dto.Category{ID: d.ID, Name: d.Name, Position: d.Position}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(DISTINCT id) FROM assets WHERE category_id=?", d.ID).Scan(&out.Count); err != nil {
		return dto.Category{}, categoryStorage(err)
	}

	if err = tx.Commit(); err != nil {
		return dto.Category{}, categoryStorage(err)
	}

	return out, nil
}

func categoryExists(ctx context.Context, tx *sql.Tx, id string) error {
	var found string

	err := tx.QueryRowContext(ctx, "SELECT id FROM categories WHERE id=?", id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.Error("NOT_FOUND", "Category not found")
	}

	if err != nil {
		return categoryStorage(err)
	}

	return nil
}

func (s *Service) DeleteCategory(ctx context.Context, id, targetID string) error {
	if strings.TrimSpace(id) == "" || id == targetID {
		return dto.Error("VALIDATION", "Select a category and a different destination category")
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return categoryStorage(err)
	}

	defer tx.Rollback()

	if err = categoryExists(ctx, tx, id); err != nil {
		return err
	}

	var target any
	if targetID != "" {
		if err = categoryExists(ctx, tx, targetID); err != nil {
			return err
		}

		target = targetID
	}

	if _, err = tx.ExecContext(
		ctx,
		"UPDATE assets SET category_id=?,revision=revision+1,updated_at=? WHERE category_id=?",
		target,
		time.Now().UTC().Format(time.RFC3339Nano),
		id,
	); err != nil {
		return categoryStorage(err)
	}

	if _, err = tx.ExecContext(ctx, "DELETE FROM categories WHERE id=?", id); err != nil {
		return categoryStorage(err)
	}

	if err = tx.Commit(); err != nil {
		return categoryStorage(err)
	}

	return nil
}

// ReorderCategories 仅接受完整排序，拒绝过期或不完整的分类列表，
// 防止旧管理页面在用户不知情时改变新增分类的顺序。
func (s *Service) ReorderCategories(ctx context.Context, ids []string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return categoryStorage(err)
	}

	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, "SELECT id FROM categories")
	if err != nil {
		return categoryStorage(err)
	}

	remaining := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return categoryStorage(err)
		}

		remaining[id] = true
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return categoryStorage(err)
	}

	if len(ids) != len(remaining) {
		return dto.Error("VALIDATION", "Sort order must include every category exactly once")
	}

	for _, id := range ids {
		if !remaining[id] {
			return dto.Error("VALIDATION", "Sort order contains unknown or duplicate categories")
		}

		delete(remaining, id)
	}

	for position, id := range ids {
		if _, err = tx.ExecContext(ctx, "UPDATE categories SET position=? WHERE id=?", position, id); err != nil {
			return categoryStorage(err)
		}
	}

	if err = tx.Commit(); err != nil {
		return categoryStorage(err)
	}

	return nil
}
