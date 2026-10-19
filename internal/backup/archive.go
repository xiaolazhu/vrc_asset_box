package backup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

// Service 要求调用方在执行 Create、Automatic 和 Restore 时持有应用维护锁。
// Restore 期间还必须阻止读取操作。
type Service struct {
	Root string
	DB   *sql.DB
}

type manifest struct {
	Version       int               `json:"version"`
	SchemaVersion int               `json:"schemaVersion"`
	CreatedAt     string            `json:"createdAt"`
	Reason        string            `json:"reason"`
	AssetCount    int               `json:"assetCount"`
	Hashes        map[string]string `json:"hashes"`
}

func New(root string, db *sql.DB) *Service {
	return &Service{Root: root, DB: db}
}

func (s *Service) Create(ctx context.Context, destination, reason string) (dto.BackupInfo, error) {
	if err := ctx.Err(); err != nil {
		return dto.BackupInfo{}, err
	}

	// 先取得一致的数据库快照，再从快照读取封面引用。
	stage, err := os.MkdirTemp(s.Root, "snapshot-")
	if err != nil {
		return dto.BackupInfo{}, err
	}

	defer os.RemoveAll(stage)

	snapshot := filepath.Join(stage, "library.db")
	if _, err = s.DB.ExecContext(ctx, "VACUUM INTO ?", snapshot); err != nil {
		return dto.BackupInfo{}, err
	}

	snapshotDB, err := openReadOnly(ctx, snapshot)
	if err != nil {
		return dto.BackupInfo{}, err
	}

	defer snapshotDB.Close()

	if err = validateSchema(ctx, snapshotDB); err != nil {
		return dto.BackupInfo{}, err
	}

	backupManifest, paths, err := s.snapshotContents(ctx, snapshotDB, snapshot, reason)
	if err != nil {
		return dto.BackupInfo{}, err
	}

	if err = os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return dto.BackupInfo{}, err
	}

	out, err := os.CreateTemp(filepath.Dir(destination), "backup-*.tmp")
	if err != nil {
		return dto.BackupInfo{}, err
	}

	tmp := out.Name()
	defer os.Remove(tmp)

	// 每个归档条目同步计算哈希，最后写入完整清单。
	writer := zip.NewWriter(out)
	for name, filename := range paths {
		if err = ctx.Err(); err != nil {
			return closeArchiveWithError(writer, out, err)
		}

		if !allowed(name) {
			return closeArchiveWithError(writer, out, fmt.Errorf("Invalid cover path"))
		}

		file, err := os.Open(filename)
		if err != nil {
			return closeArchiveWithError(writer, out, err)
		}

		entry, err := writer.Create(name)
		if err != nil {
			_ = file.Close()
			return closeArchiveWithError(writer, out, err)
		}

		h := sha256.New()
		_, err = io.Copy(io.MultiWriter(entry, h), file)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}

		if err != nil {
			return closeArchiveWithError(writer, out, err)
		}

		backupManifest.Hashes[name] = hex.EncodeToString(h.Sum(nil))
	}

	if err = writeManifest(writer, backupManifest); err != nil {
		return closeArchiveWithError(writer, out, err)
	}

	if err = writer.Close(); err != nil {
		_ = out.Close()
		return dto.BackupInfo{}, err
	}

	if err = out.Sync(); err != nil {
		_ = out.Close()
		return dto.BackupInfo{}, err
	}

	if err = out.Close(); err != nil {
		return dto.BackupInfo{}, err
	}

	// 新归档完整生成前，不能覆盖已有备份。
	if _, err = os.Stat(destination); err == nil {
		return dto.BackupInfo{}, dto.Error("VALIDATION", "This backup file already exists. Choose a new filename.")
	}

	if err = os.Rename(tmp, destination); err != nil {
		return dto.BackupInfo{}, err
	}

	info, err := os.Stat(destination)
	if err != nil {
		return dto.BackupInfo{}, err
	}

	return dto.BackupInfo{
		CreatedAt:     backupManifest.CreatedAt,
		FormatVersion: "1",
		SchemaVersion: backupManifest.SchemaVersion,
		AssetCount:    backupManifest.AssetCount,
		TotalBytes:    info.Size(),
	}, nil
}

func (s *Service) snapshotContents(
	ctx context.Context,
	snapshotDB *sql.DB,
	snapshotPath string,
	reason string,
) (manifest, map[string]string, error) {
	backupManifest := manifest{
		Version:       1,
		SchemaVersion: 1,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Reason:        reason,
		Hashes:        map[string]string{},
	}

	if err := snapshotDB.QueryRowContext(ctx, "SELECT count(*) FROM assets").Scan(&backupManifest.AssetCount); err != nil {
		return manifest{}, nil, err
	}

	paths := map[string]string{"library.db": snapshotPath}
	rows, err := snapshotDB.QueryContext(ctx, referencedMediaSQL)
	if err != nil {
		return manifest{}, nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var originalPath, thumbnailPath string
		if err := rows.Scan(&originalPath, &thumbnailPath); err != nil {
			return manifest{}, nil, err
		}

		for _, relativePath := range []string{originalPath, thumbnailPath} {
			archiveName := filepath.ToSlash(relativePath)
			resolvedPath, err := managedPath(s.Root, archiveName)
			if err != nil {
				return manifest{}, nil, err
			}

			paths[archiveName] = resolvedPath
		}
	}

	if err := rows.Err(); err != nil {
		return manifest{}, nil, err
	}

	return backupManifest, paths, nil
}

func writeManifest(writer *zip.Writer, backupManifest manifest) error {
	contents, err := json.Marshal(backupManifest)
	if err != nil {
		return err
	}

	entry, err := writer.Create("manifest.json")
	if err != nil {
		return err
	}

	_, err = entry.Write(contents)

	return err
}

func closeArchiveWithError(writer *zip.Writer, file *os.File, cause error) (dto.BackupInfo, error) {
	_ = writer.Close()
	_ = file.Close()

	return dto.BackupInfo{}, cause
}

func allowed(name string) bool {
	clean := filepath.ToSlash(filepath.Clean(name))
	return name == clean &&
		!strings.Contains(name, "\\") &&
		!strings.Contains(name, ":") &&
		(name == "library.db" || strings.HasPrefix(name, "media/") && !strings.Contains(name, ".."))
}

func extract(ctx context.Context, filename, root string) (manifest, int64, error) {
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return manifest{}, 0, err
	}

	defer archive.Close()

	var backupManifest manifest

	// 写入任何文件前，先检查整个归档的路径、类型与解压规模。
	seen := map[string]bool{}
	total := int64(0)
	for _, entry := range archive.File {
		if entry.Mode()&os.ModeSymlink != 0 || entry.FileInfo().IsDir() {
			return backupManifest, 0, fmt.Errorf("Backups cannot contain directories or symbolic links")
		}

		if seen[entry.Name] {
			return backupManifest, 0, fmt.Errorf("Backup contains duplicate files")
		}

		seen[entry.Name] = true
		if entry.Name != "manifest.json" && !allowed(entry.Name) {
			return backupManifest, 0, fmt.Errorf("Backup contains disallowed files")
		}

		if entry.UncompressedSize64 > 4<<30 {
			return backupManifest, 0, fmt.Errorf("A file in the backup is too large")
		}

		total += int64(entry.UncompressedSize64)
		if total > 20<<30 {
			return backupManifest, 0, fmt.Errorf("Total extracted backup size exceeds the limit")
		}
	}

	// 先读取清单，随后检查版本及数据库哈希是否齐全。
	for _, entry := range archive.File {
		if entry.Name == "manifest.json" {
			if entry.UncompressedSize64 > 8<<20 {
				return backupManifest, 0, fmt.Errorf("Backup manifest is too large")
			}

			input, readErr := entry.Open()
			if readErr != nil {
				return backupManifest, 0, readErr
			}

			readErr = json.NewDecoder(input).Decode(&backupManifest)
			input.Close()
			if readErr != nil {
				return backupManifest, 0, readErr
			}
		}
	}

	if backupManifest.Version != 1 || backupManifest.SchemaVersion != 1 || backupManifest.Hashes["library.db"] == "" {
		return backupManifest, 0, fmt.Errorf("Backup version is unsupported or its manifest is missing")
	}

	// 清单校验通过后，才允许提取并校验受管文件。
	for _, entry := range archive.File {
		if err = ctx.Err(); err != nil {
			return backupManifest, 0, err
		}

		if entry.Name == "manifest.json" {
			continue
		}

		expectedHash, ok := backupManifest.Hashes[entry.Name]
		if !ok {
			return backupManifest, 0, fmt.Errorf("Manifest does not list %s", entry.Name)
		}

		input, readErr := entry.Open()
		if readErr != nil {
			return backupManifest, 0, readErr
		}

		hasher := sha256.New()

		var target *os.File
		var writer io.Writer = hasher
		if root != "" {
			dest := filepath.Join(root, filepath.FromSlash(entry.Name))
			if readErr = os.MkdirAll(filepath.Dir(dest), 0700); readErr != nil {
				input.Close()
				return backupManifest, 0, readErr
			}

			target, readErr = os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if readErr != nil {
				input.Close()
				return backupManifest, 0, readErr
			}

			writer = io.MultiWriter(hasher, target)
		}

		_, readErr = io.Copy(writer, io.LimitReader(input, int64(entry.UncompressedSize64)+1))
		input.Close()
		if target != nil {
			if readErr == nil {
				readErr = target.Sync()
			}

			closeErr := target.Close()
			if readErr == nil {
				readErr = closeErr
			}
		}

		if readErr != nil {
			return backupManifest, 0, readErr
		}

		if hex.EncodeToString(hasher.Sum(nil)) != expectedHash {
			return backupManifest, 0, fmt.Errorf("File integrity check failed: %s", entry.Name)
		}
	}

	// 同时检查清单中的条目是否全部存在，避免遗漏文件。
	for name := range backupManifest.Hashes {
		if !seen[name] {
			return backupManifest, 0, fmt.Errorf("Backup is missing %s", name)
		}
	}

	return backupManifest, total, nil
}

func (s *Service) Inspect(ctx context.Context, filename string) (dto.BackupInfo, error) {
	stage, err := os.MkdirTemp("", "vrc-backup-inspect-")
	if err != nil {
		return dto.BackupInfo{}, err
	}

	defer os.RemoveAll(stage)

	backupManifest, total, err := extract(ctx, filename, stage)
	if err != nil {
		return dto.BackupInfo{}, err
	}

	if err = validateArchive(ctx, stage, backupManifest); err != nil {
		return dto.BackupInfo{}, err
	}

	return dto.BackupInfo{
		CreatedAt:     backupManifest.CreatedAt,
		SchemaVersion: backupManifest.SchemaVersion,
		FormatVersion: "1",
		AssetCount:    backupManifest.AssetCount,
		TotalBytes:    total,
	}, nil
}

func (s *Service) Automatic(ctx context.Context, reason string) error {
	if reason != "daily" && reason != "critical" {
		return fmt.Errorf("Invalid automatic backup reason")
	}

	folder := filepath.Join(s.Root, "backups")
	name := reason + "-" + time.Now().Format("20060102-150405.000000000") + ".zip"
	if reason == "daily" {
		matches, _ := filepath.Glob(filepath.Join(folder, "daily-"+time.Now().Format("20060102")+"*.zip"))
		if len(matches) > 0 {
			return nil
		}
	}

	if _, err := s.Create(ctx, filepath.Join(folder, name), reason); err != nil {
		return err
	}

	pattern := "critical-*.zip"
	keep := 3
	if reason == "daily" {
		pattern = "daily-*.zip"
		keep = 7
	}

	files, err := filepath.Glob(filepath.Join(folder, pattern))
	if err != nil {
		return err
	}

	sort.Strings(files)
	for len(files) > keep {
		if err = os.Remove(files[0]); err != nil {
			return err
		}

		files = files[1:]
	}

	return nil
}
