package main

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/xiaolazhu/vrc_asset_box/internal/desktop"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/localization"
)

func defaultPreferences() dto.Preferences {
	return dto.Preferences{
		Language: "system",
		Theme:    "system",
		View:     "cards",
		CardSize: "medium",
		Query: dto.AssetQuery{
			CategoryIDs: []string{},
			AvatarIDs:   []string{},
			FreeTagIDs:  []string{},
			AvatarMode:  "selected",
			AvatarMatch: "any",
			FreeMatch:   "any",
			Folder:      "all",
			Sort:        "updatedAt",
			Descending:  true,
			Limit:       60,
		},
	}
}

func (a *App) preferences() (dto.Preferences, error) {
	p := defaultPreferences()

	var raw string

	err := a.db.QueryRowContext(a.ctx, "SELECT value_json FROM settings WHERE key='preferences'").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}

	if err != nil {
		return p, err
	}

	if err = json.Unmarshal([]byte(raw), &p); err != nil {
		return defaultPreferences(), err
	}
	if !localization.ValidPreference(p.Language) {
		p.Language = "system"
	}

	return p, nil
}

func (a *App) GetPreferences() (dto.Preferences, error) {
	if err := a.read(); err != nil {
		return dto.Preferences{}, err
	}

	defer a.gate.RUnlock()

	return a.preferences()
}

func (a *App) SavePreferences(p dto.Preferences) error {
	if err := a.write(); err != nil {
		return err
	}

	defer a.gate.Unlock()
	// 兼容尚未携带语言字段的旧客户端。
	if p.Language == "" {
		p.Language = "system"
	}
	if !localization.ValidPreference(p.Language) {
		return dto.Error("VALIDATION", "Invalid language preference")
	}

	if p.Theme != "system" &&
		p.Theme != "light" &&
		p.Theme != "dark" ||
		p.View != "cards" &&
			p.View != "table" ||
		p.CardSize != "small" &&
			p.CardSize != "medium" &&
			p.CardSize != "large" {
		return dto.Error("VALIDATION", "Invalid display preferences")
	}

	if p.Query.Limit < 1 || p.Query.Limit > 200 {
		p.Query.Limit = 60
	}

	p.Query.Offset = 0
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}

	_, err = a.db.ExecContext(a.ctx, "INSERT INTO settings(key,value_json) VALUES('preferences',?) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json", string(b))
	if err == nil {
		a.language.Store(p.Language)
	}

	return err
}

func (a *App) GetBootstrap() (dto.Bootstrap, error) {
	if err := a.read(); err != nil {
		return dto.Bootstrap{}, err
	}

	defer a.gate.RUnlock()

	var b dto.Bootstrap
	b.SystemLocale, _ = a.systemLocale.Load().(string)
	var err error
	if b.Categories, err = a.catalog.ListCategories(a.ctx); err != nil {
		return b, err
	}

	if b.AvatarTags, err = a.tags.List(a.ctx, "avatar", ""); err != nil {
		return b, err
	}

	if b.FreeTags, err = a.tags.List(a.ctx, "free", ""); err != nil {
		return b, err
	}

	b.Preferences, err = a.preferences()

	return b, err
}

// SetLanguage 立即更新原生对话框语言，不等待前端偏好保存的防抖延迟。
func (a *App) SetLanguage(language string) error {
	if !localization.ValidPreference(language) {
		return dto.Error("VALIDATION", "Invalid language preference")
	}
	a.language.Store(language)
	return nil
}

// dialogText 使用独立的原子快照，避免对话框与维护锁互相等待。
func (a *App) dialogText(key string) string {
	language, _ := a.language.Load().(string)
	systemLocale, _ := a.systemLocale.Load().(string)
	return localization.Text(localization.Resolve(language, systemLocale), key)
}

// RefreshSystemLocale 同步界面与原生对话框的系统语言，不修改用户保存的偏好。
func (a *App) RefreshSystemLocale() string {
	language := desktop.SystemLocale()
	a.systemLocale.Store(language)
	return language
}

func (a *App) GetWorkspaceInfo() (dto.WorkspaceInfo, error) {
	if err := a.read(); err != nil {
		return dto.WorkspaceInfo{}, err
	}

	defer a.gate.RUnlock()

	info := dto.WorkspaceInfo{DataPath: a.root, SchemaVersion: 1}
	err := a.db.QueryRowContext(a.ctx, "SELECT count(*) FROM assets").Scan(&info.AssetCount)

	return info, err
}
