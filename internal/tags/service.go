package tags

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

type Service struct {
	DB *sql.DB
}

func New(db *sql.DB) *Service {
	return &Service{DB: db}
}

func storage(err error) error {
	return &dto.AppError{
		Code:    "STORAGE",
		Message: "Failed to save or read tag. Please retry.",
		Details: err.Error(),
	}
}

func validKind(kind string) bool {
	return kind == "avatar" || kind == "free"
}

func (s *Service) List(ctx context.Context, kind, keyword string) ([]dto.Tag, error) {
	if !validKind(kind) {
		return nil, dto.Error("VALIDATION", "Invalid tag type")
	}
	// 使用 EXISTS，避免同时匹配多个别名时重复累计使用次数。
	rows, err := s.DB.QueryContext(ctx, `SELECT t.id,t.kind,t.name,COUNT(DISTINCT a.asset_id)
 FROM tags t LEFT JOIN asset_tags a ON a.tag_id=t.id
 WHERE t.kind=? AND (?='' OR EXISTS(SELECT 1 FROM tag_names n WHERE n.tag_id=t.id AND n.kind=t.kind AND instr(n.name_key,?)>0))
 GROUP BY t.id,t.kind,t.name ORDER BY (SELECT name_key FROM tag_names WHERE tag_id=t.id AND canonical=1),t.id`, kind, NormalizeName(keyword), NormalizeName(keyword))
	if err != nil {
		return nil, storage(err)
	}

	result := make([]dto.Tag, 0)
	indices := map[string]int{}
	for rows.Next() {
		var t dto.Tag

		t.Aliases = []string{}
		if err = rows.Scan(&t.ID, &t.Kind, &t.Name, &t.Count); err != nil {
			rows.Close()
			return nil, storage(err)
		}

		indices[t.ID] = len(result)
		result = append(result, t)
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, storage(err)
	}

	rows, err = s.DB.QueryContext(ctx, "SELECT tag_id,name FROM tag_names WHERE kind=? AND canonical=0 ORDER BY name_key", kind)
	if err != nil {
		return nil, storage(err)
	}

	defer rows.Close()

	for rows.Next() {
		var id, name string
		if err = rows.Scan(&id, &name); err != nil {
			return nil, storage(err)
		}

		if i, ok := indices[id]; ok {
			result[i].Aliases = append(result[i].Aliases, name)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, storage(err)
	}

	return result, nil
}

func (s *Service) Save(ctx context.Context, d dto.TagDraft) (dto.Tag, error) {
	names, err := prepareNames(d)
	if err != nil {
		return dto.Tag{}, err
	}

	d.Name = names[0]
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return dto.Tag{}, storage(err)
	}

	defer tx.Rollback()

	if d.ID != "" {
		var kind string

		err = tx.QueryRowContext(ctx, "SELECT kind FROM tags WHERE id=?", d.ID).Scan(&kind)
		if errors.Is(err, sql.ErrNoRows) {
			return dto.Tag{}, dto.Error("NOT_FOUND", "Tag not found")
		}

		if err != nil {
			return dto.Tag{}, storage(err)
		}

		if kind != d.Kind {
			return dto.Tag{}, dto.Error("VALIDATION", "Cannot change tag type")
		}
	}

	if err = checkNames(ctx, tx, d.ID, d.Kind, names); err != nil {
		return dto.Tag{}, err
	}

	if d.ID == "" {
		d.ID = uuid.NewString()
		_, err = tx.ExecContext(ctx, "INSERT INTO tags(id,kind,name) VALUES(?,?,?)", d.ID, d.Kind, d.Name)
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE tags SET name=? WHERE id=?", d.Name, d.ID)
		if err == nil {
			_, err = tx.ExecContext(
				ctx,
				"UPDATE assets SET revision=revision+1,updated_at=? WHERE id IN (SELECT asset_id FROM asset_tags WHERE tag_id=?)",
				time.Now().UTC().Format(time.RFC3339Nano),
				d.ID,
			)
		}
	}

	if err != nil {
		return dto.Tag{}, storage(err)
	}

	if _, err = tx.ExecContext(ctx, "DELETE FROM tag_names WHERE tag_id=?", d.ID); err != nil {
		return dto.Tag{}, storage(err)
	}

	for i, name := range names {
		canonical := 0
		if i == 0 {
			canonical = 1
		}

		if _, err = tx.ExecContext(
			ctx,
			"INSERT INTO tag_names(tag_id,kind,name,name_key,canonical) VALUES(?,?,?,?,?)",
			d.ID,
			d.Kind,
			name,
			NormalizeName(name),
			canonical,
		); err != nil {
			return dto.Tag{}, storage(err)
		}
	}

	out := dto.Tag{ID: d.ID, Kind: d.Kind, Name: d.Name, Aliases: append([]string{}, names[1:]...)}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(DISTINCT asset_id) FROM asset_tags WHERE tag_id=?", d.ID).Scan(&out.Count); err != nil {
		return dto.Tag{}, storage(err)
	}

	if err = tx.Commit(); err != nil {
		return dto.Tag{}, storage(err)
	}

	return out, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return dto.Error("VALIDATION", "Select a tag")
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return storage(err)
	}

	defer tx.Rollback()

	if _, err = tagKind(ctx, tx, id); err != nil {
		return err
	}

	if _, err = tx.ExecContext(
		ctx,
		"UPDATE assets SET revision=revision+1,updated_at=? WHERE id IN (SELECT asset_id FROM asset_tags WHERE tag_id=?)",
		time.Now().UTC().Format(time.RFC3339Nano),
		id,
	); err != nil {
		return storage(err)
	}

	if _, err = tx.ExecContext(ctx, "DELETE FROM tags WHERE id=?", id); err != nil {
		return storage(err)
	}

	if err = tx.Commit(); err != nil {
		return storage(err)
	}

	return nil
}
