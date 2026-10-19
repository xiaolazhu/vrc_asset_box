package migration

import (
	"context"
	"database/sql"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

// 始终以目标记录的原始值作为比较基准：后续行中未修改的快照，
// 不能覆盖本事务内前面行已经补全的字段。
func accumulateDraft(base, current, incoming dto.AssetDraft) (dto.AssetDraft, error) {
	for _, field := range []struct {
		name          string
		base, current string
		incoming      *string
	}{
		{"name", base.Name, current.Name, &incoming.Name},
		{"boothId", base.BoothID, current.BoothID, &incoming.BoothID},
		{"categoryId", base.CategoryID, current.CategoryID, &incoming.CategoryID},
		{"coverId", base.CoverID, current.CoverID, &incoming.CoverID},
		{"notes", base.Notes, current.Notes, &incoming.Notes},
		{"folderPath", base.FolderPath, current.FolderPath, &incoming.FolderPath},
	} {
		switch {
		case *field.incoming == field.base:
			*field.incoming = field.current
		case field.current == field.base || *field.incoming == field.current:
		default:
			return dto.AssetDraft{}, &dto.AppError{
				Code:    "IMPORT_CONFLICT",
				Message: "Rows merging into the same target have conflicting choices for a field. Select the same value for those rows and retry.",
				Field:   field.name,
			}
		}
	}

	incoming.AvatarIDs = union(current.AvatarIDs, incoming.AvatarIDs)
	incoming.FreeTagIDs = union(current.FreeTagIDs, incoming.FreeTagIDs)
	incoming.Universal = current.Universal || incoming.Universal

	return incoming, nil
}

func loadDraftInTx(ctx context.Context, tx *sql.Tx, id string) (dto.AssetDraft, error) {
	d := dto.AssetDraft{ID: id, AvatarIDs: []string{}, FreeTagIDs: []string{}}
	err := tx.QueryRowContext(ctx, `SELECT revision,name,COALESCE(booth_id,''),COALESCE(category_id,''),COALESCE(cover_id,''),universal,notes,folder_path FROM assets WHERE id=?`, id).Scan(
		&d.Revision,
		&d.Name,
		&d.BoothID,
		&d.CategoryID,
		&d.CoverID,
		&d.Universal,
		&d.Notes,
		&d.FolderPath,
	)
	if err != nil {
		return d, err
	}

	rows, err := tx.QueryContext(ctx, "SELECT t.id,t.kind FROM asset_tags a JOIN tags t ON t.id=a.tag_id WHERE a.asset_id=?", id)
	if err != nil {
		return d, err
	}

	defer rows.Close()

	for rows.Next() {
		var tag, kind string
		if err = rows.Scan(&tag, &kind); err != nil {
			return d, err
		}

		if kind == "avatar" {
			d.AvatarIDs = append(d.AvatarIDs, tag)
		} else {
			d.FreeTagIDs = append(d.FreeTagIDs, tag)
		}
	}

	return d, rows.Err()
}
