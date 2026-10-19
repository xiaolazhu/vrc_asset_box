package database_test

import (
	"context"
	"strings"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/database"
)

func TestPresetCategoriesUseInitializationLanguage(t *testing.T) {
	for language, names := range map[string][]string{
		"en":    {"Clothing", "Hair", "Items", "Tools"},
		"zh-CN": {"衣服", "头发", "物品", "工具"},
		"zh-TW": {"衣服", "頭髮", "物品", "工具"},
		"ja":    {"衣装", "髪型", "アイテム", "ツール"},
		"ko":    {"의상", "헤어", "아이템", "도구"},
		"fr-FR": {"Clothing", "Hair", "Items", "Tools"},
	} {
		t.Run(language, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			db, err := database.OpenWithLanguage(ctx, dir, language)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for position, want := range names {
				var name string
				if err := db.QueryRow(`SELECT name FROM categories WHERE position=?`, position).Scan(&name); err != nil || name != want {
					t.Fatalf("category %d: %q, want %q; %v", position, name, want, err)
				}
			}
			if err := database.MigrateWithLanguage(ctx, db, "en"); err != nil {
				t.Fatal(err)
			}
			var name string
			if err := db.QueryRow(`SELECT name FROM categories WHERE position=0`).Scan(&name); err != nil || name != names[0] {
				t.Fatalf("existing category renamed: %q %v", name, err)
			}
		})
	}
}

func TestLibraryConstraintsAndReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := database.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	if err = database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	var count int
	if err = db.QueryRow(`SELECT count(*) FROM categories`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("categories=%d err=%v", count, err)
	}

	for position, name := range []string{"Clothing", "Hair", "Items", "Tools"} {
		var got, key string
		if err := db.QueryRow(`SELECT name,name_key FROM categories WHERE position=?`, position).Scan(&got, &key); err != nil || got != name || key != strings.ToLower(name) {
			t.Fatalf("category %d: name=%q key=%q err=%v", position, got, key, err)
		}
	}

	// 已初始化的素材库保留用户分类，不重新应用预设。
	if _, err := db.Exec(`UPDATE categories SET name='衣服',name_key='衣服' WHERE id='cloth'`); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var existingName string
	if err := db.QueryRow(`SELECT name FROM categories WHERE id='cloth'`).Scan(&existingName); err != nil || existingName != "衣服" {
		t.Fatalf("existing category changed: %q %v", existingName, err)
	}

	_, err = db.Exec(`INSERT INTO assets(id,name,name_key,created_at,updated_at) VALUES ('a','A','a','2026','2026'),('b','B','b','2026','2026')`)
	if err != nil {
		t.Fatalf("missing Booth IDs must coexist: %v", err)
	}

	_, err = db.Exec(`UPDATE assets SET booth_id='6964558' WHERE id='a'`)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = db.Exec(`UPDATE assets SET booth_id='6964558' WHERE id='b'`); err == nil {
		t.Fatal("duplicate Booth ID accepted")
	}

	if _, err = db.Exec(`UPDATE assets SET booth_id='invalid' WHERE id='b'`); err == nil {
		t.Fatal("invalid Booth ID accepted")
	}

	if _, err = db.Exec(`UPDATE assets SET category_id='missing' WHERE id='b'`); err == nil {
		t.Fatal("foreign keys disabled")
	}

	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = database.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	if err = db.QueryRow(`SELECT count(*) FROM assets`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("persisted=%d err=%v", count, err)
	}
}
