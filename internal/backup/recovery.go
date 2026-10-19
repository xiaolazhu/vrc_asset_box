package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/xiaolazhu/vrc_asset_box/internal/database"
)

type recoveryJournal struct {
	Version  int    `json:"version"`
	Stage    string `json:"stage"`
	Previous string `json:"previous"`
}

func journalPath(root string) string {
	return root + ".recovery.json"
}

func writeJournal(root string, j recoveryJournal) error {
	file, err := os.CreateTemp(filepath.Dir(root), ".recovery-*.tmp")
	if err != nil {
		return err
	}

	temp := file.Name()
	defer os.Remove(temp)

	if err = json.NewEncoder(file).Encode(j); err != nil {
		file.Close()
		return err
	}

	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}

	if err = file.Close(); err != nil {
		return err
	}
	// 使用硬链接原子发布已完整同步到磁盘的日志，且拒绝覆盖尚未处理的恢复日志。
	// 临时日志与目标日志位于同一卷。
	return os.Link(temp, journalPath(root))
}

func readJournal(root string) (recoveryJournal, error) {
	var j recoveryJournal

	file, err := os.Open(journalPath(root))
	if err != nil {
		return j, err
	}

	defer file.Close()

	decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&j); err != nil {
		return j, err
	}

	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		return j, fmt.Errorf("Restore log contains extra or invalid data")
	}

	if j.Version != 1 {
		return j, fmt.Errorf("Restore log version is unsupported")
	}

	for prefix, path := range map[string]string{".restore-": j.Stage, ".previous-": j.Previous} {
		if !filepath.IsAbs(path) ||
			filepath.Clean(path) != path ||
			filepath.Dir(path) != filepath.Dir(root) ||
			!strings.HasPrefix(path, root+prefix) ||
			path == root+prefix {
			return j, fmt.Errorf("Invalid restore log path")
		}
	}

	return j, nil
}

func generationExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("Invalid restore folder: %s", path)
	}

	return true, nil
}

// Recover 必须在 database.Open 以及任何服务或图片读取操作启动前执行。
// 若存在未处理的恢复日志，总是回滚到完整的旧版资料库，
// 并将中断的新版本保留在同级目录。
// 资料库版本缺失或无效时返回错误；恢复流程不会创建数据库。
func Recover(root string) error {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	root = absolute
	j, err := readJournal(root)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("Failed to read restore log: %w", err)
	}

	previous, err := generationExists(j.Previous)
	if err != nil {
		return err
	}

	current, err := generationExists(root)
	if err != nil {
		return err
	}

	if previous {
		if err = validateGeneration(context.Background(), j.Previous, nil); err != nil {
			return fmt.Errorf("Previous library validation failed. All data has been preserved: %w", err)
		}

		if current {
			// 若尚未执行安装，解压目录可能仍然存在。
			// 保留所有资料库版本，包括打开失败或只打开了一部分的新版本。
			destination := j.Stage
			if _, e := os.Lstat(destination); e == nil {
				destination = root + ".restore-interrupted-" + uuid.NewString()
			} else if !os.IsNotExist(e) {
				return e
			}

			if err = os.Rename(root, destination); err != nil {
				return fmt.Errorf("Failed to preserve data from the interrupted restore: %w", err)
			}
		}

		if err = os.Rename(j.Previous, root); err != nil {
			return fmt.Errorf("Failed to restore the previous library: %w", err)
		}
	} else {
		// 恢复日志已发布，但旧版资料库尚未重命名。
		if !current {
			return fmt.Errorf("Restore log exists but no complete library was found. Staging directory preserved: %s", j.Stage)
		}

		if err = validateGeneration(context.Background(), root, nil); err != nil {
			return fmt.Errorf("Current library validation failed: %w", err)
		}
	}

	if err = os.Remove(journalPath(root)); err != nil {
		return fmt.Errorf("Failed to clear restore log: %w", err)
	}

	return nil
}

// Restore 执行时必须独占所有服务及图片读取操作。
// 返回的数据库连接也会存入 Service.DB。即使 err != nil，调用方也必须
// 将各服务重新绑定到该连接，因为回滚会重新打开一个不同的连接。
// 前置检查失败时返回原有的有效连接；若无法打开任何完整的资料库版本，
// 则返回 nil，此时应用必须显示启动或恢复错误，
// 不能继续使用已关闭的旧服务。
func (s *Service) Restore(ctx context.Context, filename string) (*sql.DB, error) {
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return s.DB, err
	}

	if _, err = os.Lstat(journalPath(root)); !os.IsNotExist(err) {
		if err == nil {
			err = fmt.Errorf("An unfinished restoration exists. Restart to recover the complete library.")
		}

		return s.DB, err
	}

	stage, err := os.MkdirTemp(filepath.Dir(root), filepath.Base(root)+".restore-")
	if err != nil {
		return s.DB, err
	}

	journalWritten := false
	defer func() {
		if !journalWritten {
			os.RemoveAll(stage)
		}
	}()

	m, _, err := extract(ctx, filename, stage)
	if err != nil {
		return s.DB, err
	}

	if err = validateArchive(ctx, stage, m); err != nil {
		return s.DB, err
	}

	if err = s.Automatic(ctx, "critical"); err != nil {
		return s.DB, err
	}

	if err = ctx.Err(); err != nil {
		return s.DB, err
	}

	if _, err = generationExists(root); err != nil {
		return s.DB, err
	}

	j := recoveryJournal{Version: 1, Stage: stage, Previous: root + ".previous-" + uuid.NewString()}
	if err = writeJournal(root, j); err != nil {
		return s.DB, err
	}

	journalWritten = true
	// 发布后禁用取消：使用新的上下文完成或回滚整个资料库版本的切换，
	// 避免取消操作导致切换停在中间状态。
	if s.DB == nil {
		return s.rollback(root, fmt.Errorf("Current database is not open"))
	}

	oldDB := s.DB
	s.DB = nil
	if err = oldDB.Close(); err != nil {
		return s.rollback(root, err)
	}

	if err = os.Rename(root, j.Previous); err != nil {
		return s.rollback(root, err)
	}

	if err = os.Rename(stage, root); err != nil {
		return s.rollback(root, err)
	}

	live, err := database.Open(context.Background(), root)
	if err != nil {
		return s.rollback(root, err)
	}

	if err = os.Remove(journalPath(root)); err != nil {
		live.Close()
		return s.rollback(root, err)
	}

	s.DB = live
	// 保留旧版资料库及其自动安全备份。
	// 使用唯一目录名，后续恢复时无需删除旧数据。
	return live, nil
}

func (s *Service) rollback(root string, cause error) (*sql.DB, error) {
	recoveryErr := Recover(root)
	// Recover 失败时也会保留文件；即使出错，仍尽可能重新打开有效的当前资料库，
	// 让应用获得可用的数据库连接。
	var db *sql.DB

	openErr := validateGeneration(context.Background(), root, nil)
	if openErr == nil {
		db, openErr = database.Open(context.Background(), root)
	}

	s.DB = db

	return db, errors.Join(cause, recoveryErr, openErr)
}
