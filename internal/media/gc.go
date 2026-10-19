package media

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var managedFilename = regexp.MustCompile(`^[a-f0-9]{64}(-thumb)?\.(png|jpg|webp)$`)

// Collect 只能在持有应用维护锁时调用。
// 宽限期用于保留最近准备的图片文件，供保存失败的草稿继续使用。
func (s *Service) Collect(ctx context.Context, before time.Time) error {
	if _, err := s.DB.ExecContext(ctx, "DELETE FROM media WHERE created_at<? AND NOT EXISTS(SELECT 1 FROM assets WHERE cover_id=media.id)", before.UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}

	rows, err := s.DB.QueryContext(ctx, "SELECT original_path,thumbnail_path FROM media")
	if err != nil {
		return err
	}

	keep := map[string]bool{}
	for rows.Next() {
		var a, b string
		if err = rows.Scan(&a, &b); err != nil {
			rows.Close()
			return err
		}

		keep[filepath.Clean(a)] = true
		keep[filepath.Clean(b)] = true
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(filepath.Join(s.Root, "media"))
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return err
		}

		if entry.IsDir() || !managedFilename.MatchString(entry.Name()) {
			continue
		}

		rel := filepath.Join("media", entry.Name())
		if keep[rel] {
			continue
		}

		info, e := entry.Info()
		if e != nil {
			return e
		}

		if info.ModTime().Before(before) {
			if err = os.Remove(filepath.Join(s.Root, rel)); err != nil {
				return err
			}
		}
	}

	return nil
}
