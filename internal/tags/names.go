package tags

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func prepareNames(d dto.TagDraft) ([]string, error) {
	if !validKind(d.Kind) {
		return nil, dto.Error("VALIDATION", "Invalid tag type")
	}

	if d.Kind == "free" && len(d.Aliases) > 0 {
		return nil, dto.Error("VALIDATION", "Custom tags do not support aliases")
	}

	names := make([]string, 0, len(d.Aliases)+1)
	seen := map[string]bool{}
	for _, name := range append([]string{d.Name}, d.Aliases...) {
		name = strings.TrimSpace(name)
		key := NormalizeName(name)
		if key == "" {
			return nil, dto.Error("VALIDATION", "Tag names and aliases cannot be empty")
		}

		if seen[key] {
			return nil, dto.Error("VALIDATION", "Tag names and aliases cannot be duplicated")
		}

		seen[key] = true
		names = append(names, name)
	}

	return names, nil
}

func checkNames(ctx context.Context, tx *sql.Tx, id, kind string, names []string) error {
	for _, name := range names {
		var owner string

		err := tx.QueryRowContext(ctx, "SELECT tag_id FROM tag_names WHERE kind=? AND name_key=?", kind, NormalizeName(name)).Scan(&owner)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}

		if err != nil {
			return storage(err)
		}

		if owner != id {
			return &dto.AppError{
				Code:    "TAG_CONFLICT",
				Message: "Name or alias is already used by another tag",
				Field:   "name",
				Details: name,
			}
		}
	}

	return nil
}

func tagKind(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	var kind string

	err := tx.QueryRowContext(ctx, "SELECT kind FROM tags WHERE id=?", id).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return "", dto.Error("NOT_FOUND", "Tag not found")
	}

	if err != nil {
		return "", storage(err)
	}

	return kind, nil
}
