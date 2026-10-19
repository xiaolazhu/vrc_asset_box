package database

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	"github.com/xiaolazhu/vrc_asset_box/internal/localization"
)

//go:embed migrations/001_initial.sql
var schema string

func Migrate(ctx context.Context, db *sql.DB) error {
	return MigrateWithLanguage(ctx, db, "en")
}

// MigrateWithLanguage 以有效界面语言生成初始分类，不在升级或重开时改名。
func MigrateWithLanguage(ctx context.Context, db *sql.DB, language string) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}

	if version == 1 {
		return nil
	}

	if version > 1 {
		return fmt.Errorf("Library version %d is newer than this app supports and cannot be migrated", version)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, schema); err != nil {
		return err
	}

	for index, key := range []string{"Clothing", "Hair", "Items", "Tools"} {
		name := localization.Text(language, key)
		if _, err = tx.ExecContext(
			ctx,
			`INSERT INTO categories(id,name,name_key,position) VALUES (?,?,?,?)`,
			[]string{"cloth", "hair", "item", "tool"}[index],
			name,
			strings.ToLower(name),
			index,
		); err != nil {
			return err
		}
	}

	if _, err = tx.ExecContext(ctx, "PRAGMA user_version=1"); err != nil {
		return err
	}

	return tx.Commit()
}
