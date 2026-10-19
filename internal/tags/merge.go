package tags

import (
	"context"
	"strings"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func (s *Service) Merge(ctx context.Context, sourceID, targetID string) error {
	if strings.TrimSpace(sourceID) == "" || strings.TrimSpace(targetID) == "" || sourceID == targetID {
		return dto.Error("VALIDATION", "Select two different tags")
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return storage(err)
	}

	defer tx.Rollback()

	sourceKind, err := tagKind(ctx, tx, sourceID)
	if err != nil {
		return err
	}

	targetKind, err := tagKind(ctx, tx, targetID)
	if err != nil {
		return err
	}

	if sourceKind != targetKind {
		return dto.Error("VALIDATION", "Only tags of the same type can be merged")
	}
	// 共享名称空间已保证源标签的各个名称均唯一。
	// 在同一事务内转移这些名称，且仅保留目标标签原有的正式名称。
	if sourceKind == "avatar" {
		if _, err = tx.ExecContext(ctx, "UPDATE tag_names SET tag_id=?,canonical=0 WHERE tag_id=?", targetID, sourceID); err != nil {
			return storage(err)
		}
	}

	if _, err = tx.ExecContext(
		ctx,
		"UPDATE assets SET revision=revision+1,updated_at=? WHERE id IN (SELECT asset_id FROM asset_tags WHERE tag_id=?)",
		time.Now().UTC().Format(time.RFC3339Nano),
		sourceID,
	); err != nil {
		return storage(err)
	}

	if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO asset_tags(asset_id,tag_id) SELECT asset_id,? FROM asset_tags WHERE tag_id=?", targetID, sourceID); err != nil {
		return storage(err)
	}

	if _, err = tx.ExecContext(ctx, "DELETE FROM tags WHERE id=?", sourceID); err != nil {
		return storage(err)
	}

	if err = tx.Commit(); err != nil {
		return storage(err)
	}

	return nil
}
