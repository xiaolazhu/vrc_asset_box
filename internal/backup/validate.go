package backup

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/xiaolazhu/vrc_asset_box/internal/database"
)

const referencedMediaSQL = `SELECT DISTINCT m.original_path,m.thumbnail_path FROM media m JOIN assets a ON a.cover_id=m.id`

// 不可用 database.Open 检查不受信任的数据库：它会执行迁移，
// 可能将缺失或不完整的备份补成表面上有效的数据库。
func openReadOnly(ctx context.Context, filename string) (*sql.DB, error) {
	absolute, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("Database is not a regular file")
	}

	path := filepath.ToSlash(absolute)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	uri := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func schemaObjects(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT type,name,sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' AND sql IS NOT NULL ORDER BY type,name`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	objects := map[string]string{}
	for rows.Next() {
		var kind, name, ddl string
		if err = rows.Scan(&kind, &name, &ddl); err != nil {
			return nil, err
		}

		objects[kind+":"+name] = strings.Join(strings.Fields(ddl), " ")
	}

	return objects, rows.Err()
}

func validateSchema(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}

	if version != 1 {
		return fmt.Errorf("Backup database version %d is unsupported", version)
	}

	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return err
	}

	if integrity != "ok" {
		return fmt.Errorf("Database integrity check failed: %s", integrity)
	}

	expected, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return err
	}

	defer expected.Close()

	expected.SetMaxOpenConns(1)
	if err = database.Migrate(ctx, expected); err != nil {
		return err
	}

	want, err := schemaObjects(ctx, expected)
	if err != nil {
		return err
	}

	got, err := schemaObjects(ctx, db)
	if err != nil {
		return err
	}

	if len(got) != len(want) {
		return fmt.Errorf("Backup database table schema is incomplete or incompatible")
	}

	for name, ddl := range want {
		if got[name] != ddl {
			return fmt.Errorf("Incompatible backup database schema: %s", name)
		}
	}

	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}

	defer rows.Close()

	if rows.Next() {
		return fmt.Errorf("Backup database contains invalid relationships")
	}

	return rows.Err()
}

func managedPath(root, name string) (string, error) {
	if !allowed(name) || name == "library.db" {
		return "", fmt.Errorf("Invalid cover path: %s", name)
	}

	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("Cover paths must be inside the library folder")
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}

	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("Invalid cover file")
	}

	return resolved, nil
}

func validateGeneration(ctx context.Context, root string, archive *manifest) error {
	db, err := openReadOnly(ctx, filepath.Join(root, "library.db"))
	if err != nil {
		return err
	}

	defer db.Close()

	if err = validateSchema(ctx, db); err != nil {
		return err
	}

	var count int
	if err = db.QueryRowContext(ctx, "SELECT count(*) FROM assets").Scan(&count); err != nil {
		return err
	}

	if archive != nil && count != archive.AssetCount {
		return fmt.Errorf("Backup asset count does not match the manifest")
	}

	paths := map[string]bool{"library.db": true}
	rows, err := db.QueryContext(ctx, referencedMediaSQL)
	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var original, thumbnail string
		if err = rows.Scan(&original, &thumbnail); err != nil {
			return err
		}

		for _, p := range []string{original, thumbnail} {
			name := filepath.ToSlash(p)
			if _, err = managedPath(root, name); err != nil {
				return err
			}

			paths[name] = true
		}
	}

	if err = rows.Err(); err != nil {
		return err
	}

	if archive != nil {
		if len(paths) != len(archive.Hashes) {
			return fmt.Errorf("Backup files do not match referenced covers")
		}

		for name := range paths {
			if archive.Hashes[name] == "" {
				return fmt.Errorf("Backup is missing a referenced cover: %s", name)
			}
		}
	}

	return nil
}

func validateArchive(ctx context.Context, root string, m manifest) error {
	return validateGeneration(ctx, root, &m)
}
