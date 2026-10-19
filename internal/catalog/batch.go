package catalog

import (
	"context"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func (s *Service) Batch(ctx context.Context, req dto.BatchRequest) error {
	ids := unique(req.IDs)
	if len(ids) == 0 {
		return validation("ids", "Select assets")
	}

	switch req.Operation {
	case "category", "addTags", "removeTags", "delete":
	default:
		return validation("operation", "Invalid bulk action")
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return assetStorage(err)
	}

	defer tx.Rollback()

	args := idArgs(ids)
	filter := placeholders(len(ids))

	var count int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM assets WHERE id IN ("+filter+")", args...).Scan(&count); err != nil {
		return assetStorage(err)
	}

	if count != len(ids) {
		return dto.Error("NOT_FOUND", "Some assets no longer exist. Refresh and retry.")
	}

	if req.Operation == "category" && req.CategoryID != "" {
		if err = categoryExists(ctx, tx, req.CategoryID); err != nil {
			return err
		}
	}

	tagIDs := unique(req.TagIDs)
	if req.Operation == "addTags" || req.Operation == "removeTags" {
		if len(tagIDs) == 0 {
			return validation("tagIds", "Select a tag")
		}

		if err = validateTagIDs(ctx, tx, tagIDs, ""); err != nil {
			return err
		}
	}

	if req.Operation == "delete" {
		_, err = tx.ExecContext(ctx, "DELETE FROM assets WHERE id IN ("+filter+")", args...)
	} else {
		stamp := time.Now().UTC().Format(time.RFC3339Nano)
		if req.Operation == "category" {
			_, err = tx.ExecContext(ctx, "UPDATE assets SET category_id=?,revision=revision+1,updated_at=? WHERE id IN ("+filter+")", append([]any{nullable(req.CategoryID), stamp}, args...)...)
		} else {
			_, err = tx.ExecContext(ctx, "UPDATE assets SET revision=revision+1,updated_at=? WHERE id IN ("+filter+")", append([]any{stamp}, args...)...)
			if err == nil {
				if req.Operation == "addTags" {
					for _, tag := range tagIDs {
						if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO asset_tags(asset_id,tag_id) SELECT id,? FROM assets WHERE id IN ("+filter+")", append([]any{tag}, args...)...); err != nil {
							break
						}
					}
				} else {
					_, err = tx.ExecContext(ctx, "DELETE FROM asset_tags WHERE asset_id IN ("+filter+") AND tag_id IN ("+placeholders(len(tagIDs))+")", append(append([]any{}, args...), idArgs(tagIDs)...)...)
				}
			}
		}
	}

	if err != nil {
		return assetStorage(err)
	}

	if err = tx.Commit(); err != nil {
		return assetStorage(err)
	}

	return nil
}
