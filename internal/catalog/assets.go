package catalog

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/tags"
)

func assetStorage(err error) error {
	return &dto.AppError{
		Code:    "STORAGE",
		Message: "Failed to save or read asset. Please retry.",
		Details: err.Error(),
	}
}

func validation(field, message string) error {
	return &dto.AppError{Code: "VALIDATION", Message: message, Field: field}
}

func nullable(s string) any {
	if s == "" {
		return nil
	}

	return s
}

func digits(s string) bool {
	if s == "" {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

func unique(ids []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}

	return out
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func idArgs(ids []string) []any {
	a := make([]any, len(ids))
	for i, id := range ids {
		a[i] = id
	}

	return a
}

func (s *Service) Save(ctx context.Context, d dto.AssetDraft) (dto.Asset, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return dto.Asset{}, assetStorage(err)
	}

	defer tx.Rollback()

	id, err := s.SaveInTx(ctx, tx, d)
	if err != nil {
		return dto.Asset{}, err
	}

	if err = tx.Commit(); err != nil {
		return dto.Asset{}, assetStorage(err)
	}

	return s.Get(ctx, id)
}

// SaveInTx 将提交或回滚交给调用方，使迁移能够原子保存所有行及来源记录。
// 此方法只使用传入的事务，不使用服务持有的 DB 连接。
func (s *Service) SaveInTx(ctx context.Context, tx *sql.Tx, d dto.AssetDraft) (string, error) {
	d.Name = strings.TrimSpace(d.Name)
	d.BoothID = strings.TrimSpace(d.BoothID)
	if d.Name == "" {
		return "", validation("name", "Enter an asset name")
	}

	if d.BoothID != "" && !digits(d.BoothID) {
		return "", validation("boothId", "Booth ID can only contain digits")
	}

	if d.CoverToken != "" {
		return "", validation("coverToken", "Cover is not ready yet")
	}

	if d.ID != "" {
		var revision int

		err := tx.QueryRowContext(ctx, "SELECT revision FROM assets WHERE id=?", d.ID).Scan(&revision)
		if errors.Is(err, sql.ErrNoRows) {
			return "", dto.Error("NOT_FOUND", "Asset not found")
		}

		if err != nil {
			return "", assetStorage(err)
		}

		if revision != d.Revision {
			return "", dto.Error("STALE_DRAFT", "Asset has changed. Reopen it before editing.")
		}
	}

	if d.BoothID != "" {
		var owner string

		err := tx.QueryRowContext(ctx, "SELECT id FROM assets WHERE booth_id=? AND id<>?", d.BoothID, d.ID).Scan(&owner)
		if err == nil {
			return "", &dto.AppError{
				Code:            "DUPLICATE_BOOTH",
				Message:         "This Booth product already exists",
				Field:           "boothId",
				ExistingAssetID: owner,
			}
		}

		if !errors.Is(err, sql.ErrNoRows) {
			return "", assetStorage(err)
		}
	}

	if d.CategoryID != "" {
		if err := categoryExists(ctx, tx, d.CategoryID); err != nil {
			return "", err
		}
	}

	if d.CoverID != "" {
		var id string

		err := tx.QueryRowContext(ctx, "SELECT id FROM media WHERE id=?", d.CoverID).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return "", validation("coverId", "Cover not found. Choose it again.")
		}

		if err != nil {
			return "", assetStorage(err)
		}
	}

	replacements, err := createDraftTags(ctx, tx, d.NewTags)
	if err != nil {
		return "", err
	}

	avatars, err := resolveTags(ctx, tx, d.AvatarIDs, replacements, "avatar")
	if err != nil {
		return "", err
	}

	free, err := resolveTags(ctx, tx, d.FreeTagIDs, replacements, "free")
	if err != nil {
		return "", err
	}

	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if d.ID == "" {
		d.ID = uuid.NewString()
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO assets(id,booth_id,name,name_key,category_id,cover_id,universal,notes,folder_path,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			d.ID,
			nullable(d.BoothID),
			d.Name,
			tags.NormalizeName(d.Name),
			nullable(d.CategoryID),
			nullable(d.CoverID),
			d.Universal,
			d.Notes,
			d.FolderPath,
			stamp,
			stamp,
		)
	} else {
		var result sql.Result

		result, err = tx.ExecContext(
			ctx,
			`UPDATE assets SET booth_id=?,name=?,name_key=?,category_id=?,cover_id=?,universal=?,notes=?,folder_path=?,updated_at=?,revision=revision+1 WHERE id=? AND revision=?`,
			nullable(d.BoothID),
			d.Name,
			tags.NormalizeName(d.Name),
			nullable(d.CategoryID),
			nullable(d.CoverID),
			d.Universal,
			d.Notes,
			d.FolderPath,
			stamp,
			d.ID,
			d.Revision,
		)
		if err == nil {
			n, e := result.RowsAffected()
			if e != nil {
				return "", assetStorage(e)
			}

			if n != 1 {
				return "", dto.Error("STALE_DRAFT", "Asset has changed. Reopen it before editing.")
			}
		}
	}

	if err != nil {
		return "", assetStorage(err)
	}

	if _, err = tx.ExecContext(ctx, "DELETE FROM asset_tags WHERE asset_id=?", d.ID); err != nil {
		return "", assetStorage(err)
	}

	for _, id := range append(avatars, free...) {
		if _, err = tx.ExecContext(ctx, "INSERT INTO asset_tags(asset_id,tag_id) VALUES(?,?)", d.ID, id); err != nil {
			return "", assetStorage(err)
		}
	}

	return d.ID, nil
}

func createDraftTags(ctx context.Context, tx *sql.Tx, drafts []dto.NewTag) (map[string]string, error) {
	replacements := map[string]string{}
	for _, d := range drafts {
		if strings.TrimSpace(d.Key) == "" || replacements[d.Key] != "" {
			return nil, validation("newTags", "New tag temporary key is invalid or duplicated")
		}

		if d.Kind != "avatar" && d.Kind != "free" {
			return nil, validation("newTags", "Invalid tag type")
		}

		if d.Kind == "free" && len(d.Aliases) > 0 {
			return nil, validation("newTags", "Custom tags do not support aliases")
		}

		names := append([]string{d.Name}, d.Aliases...)
		seen := map[string]bool{}
		for i, name := range names {
			name = strings.TrimSpace(name)
			key := tags.NormalizeName(name)
			if key == "" || seen[key] {
				return nil, validation("newTags", "Tag names and aliases cannot be empty or duplicated")
			}

			seen[key] = true
			names[i] = name

			var owner string

			err := tx.QueryRowContext(ctx, "SELECT tag_id FROM tag_names WHERE kind=? AND name_key=?", d.Kind, key).Scan(&owner)
			if err == nil {
				return nil, &dto.AppError{
					Code:    "TAG_CONFLICT",
					Message: "Tag name or alias is already in use",
					Field:   "newTags",
					Details: name,
				}
			}

			if !errors.Is(err, sql.ErrNoRows) {
				return nil, assetStorage(err)
			}
		}

		id := uuid.NewString()
		if _, err := tx.ExecContext(ctx, "INSERT INTO tags(id,kind,name) VALUES(?,?,?)", id, d.Kind, names[0]); err != nil {
			return nil, assetStorage(err)
		}

		for i, name := range names {
			if _, err := tx.ExecContext(
				ctx,
				"INSERT INTO tag_names(tag_id,kind,name,name_key,canonical) VALUES(?,?,?,?,?)",
				id,
				d.Kind,
				name,
				tags.NormalizeName(name),
				i == 0,
			); err != nil {
				return nil, assetStorage(err)
			}
		}

		replacements[d.Key] = id
	}

	return replacements, nil
}

func resolveTags(ctx context.Context, tx *sql.Tx, ids []string, replacements map[string]string, kind string) ([]string, error) {
	resolved := make([]string, 0, len(ids))
	for _, id := range ids {
		if replacement := replacements[id]; replacement != "" {
			id = replacement
		}

		resolved = append(resolved, id)
	}

	resolved = unique(resolved)
	if err := validateTagIDs(ctx, tx, resolved, kind); err != nil {
		return nil, err
	}

	return resolved, nil
}

func validateTagIDs(ctx context.Context, tx *sql.Tx, ids []string, kind string) error {
	if len(ids) == 0 {
		return nil
	}

	rows, err := tx.QueryContext(ctx, "SELECT id,kind FROM tags WHERE id IN ("+placeholders(len(ids))+")", idArgs(ids)...)
	if err != nil {
		return assetStorage(err)
	}

	found := map[string]string{}
	for rows.Next() {
		var id, k string
		if err = rows.Scan(&id, &k); err != nil {
			rows.Close()
			return assetStorage(err)
		}

		found[id] = k
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return assetStorage(err)
	}

	for _, id := range ids {
		if found[id] == "" {
			return dto.Error("NOT_FOUND", "Tag not found")
		}

		if kind != "" && found[id] != kind {
			return validation("tagIds", "Tag type mismatch")
		}
	}

	return nil
}

const assetColumns = `a.id,COALESCE(a.booth_id,''),a.name,COALESCE(a.category_id,''),COALESCE(a.cover_id,''),a.universal,a.notes,a.folder_path,a.created_at,a.updated_at,a.revision`

type rowScanner interface{ Scan(...any) error }

func scanAsset(row rowScanner) (dto.Asset, error) {
	a := dto.Asset{
		AssetDraft: dto.AssetDraft{AvatarIDs: []string{}, FreeTagIDs: []string{}, NewTags: []dto.NewTag{}},
		Avatars:    []dto.Tag{},
		FreeTags:   []dto.Tag{},
	}
	err := row.Scan(
		&a.ID,
		&a.BoothID,
		&a.Name,
		&a.CategoryID,
		&a.CoverID,
		&a.Universal,
		&a.Notes,
		&a.FolderPath,
		&a.CreatedAt,
		&a.UpdatedAt,
		&a.Revision,
	)
	if a.CoverID != "" {
		base := "/media/" + url.PathEscape(a.CoverID)
		a.CoverURL = base + "/original"
		a.ThumbnailURL = base + "/thumbnail"
	}

	return a, err
}

func (s *Service) Get(ctx context.Context, id string) (dto.Asset, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return dto.Asset{}, assetStorage(err)
	}

	defer tx.Rollback()

	a, err := scanAsset(tx.QueryRowContext(ctx, "SELECT "+assetColumns+" FROM assets a WHERE a.id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return dto.Asset{}, dto.Error("NOT_FOUND", "Asset not found")
	}

	if err != nil {
		return dto.Asset{}, assetStorage(err)
	}

	result := []dto.Asset{a}
	if err = loadAssetTags(ctx, tx, result); err != nil {
		return dto.Asset{}, err
	}

	if err = tx.Commit(); err != nil {
		return dto.Asset{}, assetStorage(err)
	}

	return result[0], nil
}

// 使用两次查询补全整页数据，查询次数不随素材或标签数量增加。
func loadAssetTags(ctx context.Context, tx *sql.Tx, assets []dto.Asset) error {
	if len(assets) == 0 {
		return nil
	}

	ids := make([]string, len(assets))
	indices := map[string]int{}
	for i, a := range assets {
		ids[i] = a.ID
		indices[a.ID] = i
	}

	filter := placeholders(len(ids))
	args := idArgs(ids)
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT n.tag_id,n.name,n.name_key FROM tag_names n JOIN asset_tags at ON at.tag_id=n.tag_id WHERE n.canonical=0 AND at.asset_id IN (`+filter+`) ORDER BY n.name_key,n.tag_id`, args...)
	if err != nil {
		return assetStorage(err)
	}

	aliases := map[string][]string{}
	for rows.Next() {
		var id, name, key string
		if err = rows.Scan(&id, &name, &key); err != nil {
			rows.Close()
			return assetStorage(err)
		}

		aliases[id] = append(aliases[id], name)
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return assetStorage(err)
	}

	rows, err = tx.QueryContext(ctx, `SELECT at.asset_id,t.id,t.kind,t.name,(SELECT count(*) FROM asset_tags usage WHERE usage.tag_id=t.id) FROM asset_tags at JOIN tags t ON t.id=at.tag_id JOIN tag_names n ON n.tag_id=t.id AND n.canonical=1 WHERE at.asset_id IN (`+filter+`) ORDER BY n.name_key,t.id`, args...)
	if err != nil {
		return assetStorage(err)
	}

	defer rows.Close()

	for rows.Next() {
		var aid string
		var tag dto.Tag
		if err = rows.Scan(&aid, &tag.ID, &tag.Kind, &tag.Name, &tag.Count); err != nil {
			return assetStorage(err)
		}

		tag.Aliases = append([]string{}, aliases[tag.ID]...)
		a := &assets[indices[aid]]
		if tag.Kind == "avatar" {
			a.AvatarIDs = append(a.AvatarIDs, tag.ID)
			a.Avatars = append(a.Avatars, tag)
		} else {
			a.FreeTagIDs = append(a.FreeTagIDs, tag.ID)
			a.FreeTags = append(a.FreeTags, tag)
		}
	}

	if err = rows.Err(); err != nil {
		return assetStorage(err)
	}

	return nil
}
