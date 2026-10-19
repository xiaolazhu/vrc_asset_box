package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/xiaolazhu/vrc_asset_box/internal/backup"
	"github.com/xiaolazhu/vrc_asset_box/internal/catalog"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/excel"
	"github.com/xiaolazhu/vrc_asset_box/internal/localization"
	"github.com/xiaolazhu/vrc_asset_box/internal/media"
	"github.com/xiaolazhu/vrc_asset_box/internal/tags"
)

type session struct {
	preview   dto.ImportPreview
	path      string
	created   time.Time
	tokens    map[string]bool
	report    []reportRow
	committed bool
}

type Service struct {
	db       *sql.DB
	media    *media.Service
	backup   *backup.Service
	mu       sync.Mutex
	sessions map[string]*session
}

type Progress func(stage string, completed, total int, cancellable bool)

func New(db *sql.DB, m *media.Service, b *backup.Service) *Service {
	return &Service{db: db, media: m, backup: b, sessions: map[string]*session{}}
}

func clone[T any](v T) T {
	b, _ := json.Marshal(v)

	var out T

	json.Unmarshal(b, &out)

	return out
}

func splitTags(value string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range strings.FieldsFunc(value, func(r rune) bool {
		return strings.ContainsRune(",，、;；/／|｜\n\r\t", r)
	}) {
		v = strings.TrimSpace(v)
		key := tags.NormalizeName(v)
		if key != "" && !seen[key] {
			seen[key] = true
			out = append(out, v)
		}
	}

	return out
}

func column(v string) string {
	return strings.ToUpper(strings.TrimSpace(v))
}

func defaultMapping(sh excel.Sheet, categories map[string]string) dto.SheetMapping {
	m := dto.SheetMapping{Sheet: sh.Name, Included: true}
	name := sh.Name
	m.CategoryID = categories[tags.NormalizeName(name)]
	if m.CategoryID == "" {
		// 优先保留已有分类匹配，再兼容新旧预设名称。
		var aliases []string
		for index, key := range []string{"Clothing", "Hair", "Items", "Tools"} {
			names := []string{[]string{"cloth", "hair", "item", "tool"}[index]}
			for _, language := range []string{"en", "zh-CN", "zh-TW", "ja", "ko"} {
				names = append(names, localization.Text(language, key))
			}
			for _, candidate := range names {
				if tags.NormalizeName(candidate) == tags.NormalizeName(sh.Name) {
					aliases = names
					break
				}
			}
			if len(aliases) > 0 {
				break
			}
		}
		if strings.EqualFold(sh.Name, "body") {
			aliases = []string{"身体"}
		}
		for _, alias := range aliases {
			if id := categories[tags.NormalizeName(alias)]; id != "" {
				m.CategoryID = id
				break
			}
		}
	}
	if m.CategoryID == "" {
		m.NewCategoryName = name
	}

	for col, h := range sh.Headers {
		h = strings.ToLower(strings.TrimSpace(h))
		switch h {
		case "id", "booth id", "boothid", "booth_id", "商品id", "商品 id":
			m.BoothColumn = col
		case "name", "名称", "商品名称", "名字":
			m.NameColumn = col
		case "image", "图片", "封面", "img", "picture":
			m.ImageColumn = col
		case "avatar", "avatars", "avatar tags", "适配", "适配avatar", "素体标签":
			m.AvatarColumn = col
		case "notes", "note", "备注", "remark", "remarks":
			m.NotesColumn = col
		case "tags", "custom tags", "自由标签", "标签":
			m.FreeTagsColumn = col
		case "folder", "linked folder", "文件夹", "文件夹路径", "关联文件夹":
			m.FolderColumn = col
		}
	}

	return m
}

func (s *Service) Preview(ctx context.Context, req dto.ImportPreviewRequest) (dto.ImportPreview, error) {
	return s.PreviewWithProgress(ctx, req, nil)
}

func (s *Service) PreviewWithProgress(ctx context.Context, req dto.ImportPreviewRequest, progress Progress) (out dto.ImportPreview, err error) {
	w, err := excel.Read(ctx, req.Path)
	if err != nil {
		return out, err
	}

	defer w.Close()

	cats, err := catalog.New(s.db).ListCategories(ctx)
	if err != nil {
		return out, err
	}

	categoryNames := map[string]string{}
	categoryIDs := map[string]bool{}
	for _, c := range cats {
		categoryNames[tags.NormalizeName(c.Name)] = c.ID
		categoryIDs[c.ID] = true
	}

	names := map[string]string{}
	rows, err := s.db.QueryContext(ctx, "SELECT kind,name_key,tag_id FROM tag_names")
	if err != nil {
		return out, err
	}

	for rows.Next() {
		var kind, key, id string
		if err = rows.Scan(&kind, &key, &id); err != nil {
			rows.Close()
			return out, err
		}

		names[kind+":"+key] = id
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}

	provided := map[string]dto.SheetMapping{}
	for _, m := range req.Mappings {
		if _, ok := provided[m.Sheet]; ok {
			return out, dto.Error("VALIDATION", "Duplicate sheet mapping")
		}

		provided[m.Sheet] = m
	}

	out = dto.ImportPreview{
		PreviewID:    uuid.NewString(),
		SourceSHA256: w.SHA256,
		Sheets:       []dto.SheetMapping{},
		Rows:         []dto.ImportRow{},
	}
	sess := &session{path: req.Path, created: time.Now(), tokens: map[string]bool{}}
	defer func() {
		if err != nil {
			for token := range sess.tokens {
				s.media.Discard(context.Background(), token)
			}
		}
	}()

	seenBooth := map[string]string{}
	total := 0
	for _, sh := range w.Sheets {
		total += len(sh.Rows)
	}

	for _, sh := range w.Sheets {
		m := defaultMapping(sh, categoryNames)
		if len(req.Mappings) > 0 {
			var ok bool

			m, ok = provided[sh.Name]
			if !ok {
				m = dto.SheetMapping{Sheet: sh.Name}
			}
		}

		m.BoothColumn = column(m.BoothColumn)
		m.NameColumn = column(m.NameColumn)
		m.ImageColumn = column(m.ImageColumn)
		m.AvatarColumn = column(m.AvatarColumn)
		m.NotesColumn = column(m.NotesColumn)
		m.FreeTagsColumn = column(m.FreeTagsColumn)
		m.FolderColumn = column(m.FolderColumn)
		m.NewCategoryName = strings.TrimSpace(m.NewCategoryName)
		if m.CategoryID != "" && !categoryIDs[m.CategoryID] {
			return out, dto.Error("VALIDATION", "The mapped category does not exist")
		}

		if m.CategoryID != "" && m.NewCategoryName != "" {
			return out, dto.Error("VALIDATION", "Select an existing or new category")
		}

		if m.NewCategoryName == "Uncategorized" || m.NewCategoryName == "All assets" || m.NewCategoryName == "未分类" || m.NewCategoryName == "全部素材" {
			return out, dto.Error("VALIDATION", "System views cannot be used as new categories")
		}

		out.Sheets = append(out.Sheets, m)
		if !m.Included {
			continue
		}

		for _, row := range sh.Rows {
			if err = ctx.Err(); err != nil {
				return out, err
			}

			r := dto.ImportRow{
				Key:          w.SHA256 + ":" + sh.Name + ":" + strconv.Itoa(row.Number),
				Sheet:        sh.Name,
				Row:          row.Number,
				Issues:       append([]string{}, sh.Issues...),
				ImageOptions: []dto.MediaDraft{},
			}
			d := dto.AssetDraft{
				Name:       strings.TrimSpace(row.Values[m.NameColumn]),
				BoothID:    strings.TrimSpace(row.Values[m.BoothColumn]),
				Notes:      row.Values[m.NotesColumn],
				FolderPath: strings.TrimSpace(row.Values[m.FolderColumn]),
				CategoryID: m.CategoryID,
				AvatarIDs:  []string{},
				FreeTagIDs: []string{},
				NewTags:    []dto.NewTag{},
			}
			if m.NewCategoryName != "" {
				d.CategoryID = "new-category:" + sh.Name
			}

			if d.BoothID == "无" || strings.EqualFold(d.BoothID, "None") {
				d.BoothID = ""
			}

			if d.Name == "" {
				r.Issues = append(r.Issues, "Name is empty. Enter a name or skip this row.")
			}

			if d.BoothID != "" && strings.IndexFunc(d.BoothID, func(r rune) bool {
				return r < '0' || r > '9'
			}) >= 0 {
				r.Issues = append(r.Issues, "Booth ID can only contain digits")
			}

			for _, kind := range []string{"avatar", "free"} {
				value := row.Values[m.AvatarColumn]
				if kind == "free" {
					value = row.Values[m.FreeTagsColumn]
				}

				for _, v := range splitTags(value) {
					if kind == "avatar" && (v == "通用" || strings.EqualFold(v, "Universal")) {
						d.Universal = true
						continue
					}

					key := kind + ":" + tags.NormalizeName(v)
					id := names[key]
					if id == "" {
						id = "new-tag:" + key
						d.NewTags = append(d.NewTags, dto.NewTag{Key: id, Kind: kind, Name: v, Aliases: []string{}})
						r.Issues = append(r.Issues, "New tag needs confirmation: "+v)
					}

					if kind == "avatar" {
						d.AvatarIDs = append(d.AvatarIDs, id)
					} else {
						d.FreeTagIDs = append(d.FreeTagIDs, id)
					}
				}
			}

			for _, image := range row.Images {
				if image.Issue != "" {
					r.Issues = append(r.Issues, image.Issue)
				}

				if m.ImageColumn == "" || strings.TrimRightFunc(image.Cell, unicode.IsDigit) != m.ImageColumn {
					continue
				}

				if image.Part == "" {
					continue
				}

				b, e := w.Image(image)
				if e != nil {
					r.Issues = append(r.Issues, fmt.Sprintf("Failed to read image: %v", e))
					continue
				}

				imageDraft, e := s.media.StageBytes(ctx, b)
				if e != nil {
					r.Issues = append(r.Issues, fmt.Sprintf("Unsupported image format: %v", e))
					continue
				}

				sess.tokens[imageDraft.Token] = true
				r.ImageOptions = append(r.ImageOptions, imageDraft)
			}

			if value := strings.TrimSpace(row.Values[m.ImageColumn]); m.ImageColumn != "" && value != "" {
				imageDraft, e := s.stageImagePath(ctx, req.Path, value)
				if e != nil {
					r.Issues = append(r.Issues, fmt.Sprintf("Cannot import local image path: %v", e))
				} else {
					sess.tokens[imageDraft.Token] = true
					r.ImageOptions = append(r.ImageOptions, imageDraft)
				}
			}

			if len(r.ImageOptions) == 1 {
				d.CoverToken = r.ImageOptions[0].Token
			} else if len(r.ImageOptions) > 1 {
				r.Issues = append(r.Issues, "Multiple images found. Choose a cover.")
			}

			err = s.db.QueryRowContext(
				ctx,
				`SELECT a.id,a.revision FROM import_sources s JOIN assets a ON a.id=s.asset_id WHERE s.source_sha256=? AND s.sheet=? AND s.row_number=?`,
				w.SHA256,
				sh.Name,
				row.Number,
			).Scan(&r.ExistingAssetID, &r.ExistingRevision)
			if err != nil && err != sql.ErrNoRows {
				return out, err
			}

			if r.ExistingAssetID == "" && d.BoothID != "" {
				err = s.db.QueryRowContext(ctx, "SELECT id,revision FROM assets WHERE booth_id=?", d.BoothID).Scan(&r.ExistingAssetID, &r.ExistingRevision)
				if err != nil && err != sql.ErrNoRows {
					return out, err
				}
			}

			if r.ExistingAssetID != "" {
				r.Issues = append(r.Issues, "Record already exists; skipped by default. Choose which fields to keep before merging.")
				existing, e := catalog.New(s.db).Get(ctx, r.ExistingAssetID)
				if e != nil {
					return out, e
				}

				d.AvatarIDs = union(d.AvatarIDs, existing.AvatarIDs)
				d.FreeTagIDs = union(d.FreeTagIDs, existing.FreeTagIDs)
			}

			if d.BoothID != "" {
				if prior := seenBooth[d.BoothID]; prior != "" {
					r.ExistingRowKey = prior
					r.Issues = append(r.Issues, "Duplicate Booth IDs in this workbook. Merge or skip them.")
				} else {
					seenBooth[d.BoothID] = r.Key
				}
			}

			r.Draft = d
			out.Rows = append(out.Rows, r)
			if progress != nil {
				progress("Generating preview…", len(out.Rows), total, true)
			}
		}
	}

	for name := range provided {
		found := false
		for _, sh := range w.Sheets {
			if sh.Name == name {
				found = true
			}
		}

		if !found {
			return out, dto.Error("VALIDATION", "The mapped sheet does not exist")
		}
	}

	if err = ctx.Err(); err != nil {
		return out, err
	}

	sess.preview = clone(out)
	s.mu.Lock()
	for id, v := range s.sessions {
		if time.Since(v.created) > 24*time.Hour {
			for token := range v.tokens {
				s.media.Discard(context.Background(), token)
			}

			delete(s.sessions, id)
		}
	}

	s.sessions[out.PreviewID] = sess
	s.mu.Unlock()

	return out, nil
}

func (s *Service) stageImagePath(ctx context.Context, workbookPath, value string) (dto.MediaDraft, error) {
	if strings.HasPrefix(value, "=") {
		return dto.MediaDraft{}, fmt.Errorf("Only static local file paths are accepted; formulas are not executed")
	}

	if !filepath.IsAbs(value) {
		u, e := url.Parse(value)
		if e != nil || u.Scheme != "" || strings.HasPrefix(value, "//") {
			return dto.MediaDraft{}, fmt.Errorf("Only local image files are accepted; network URLs are not fetched")
		}

		value = filepath.Join(filepath.Dir(workbookPath), value)
	}

	return s.media.StagePath(ctx, value)
}

func union(a, b []string) []string {
	out := append([]string{}, a...)
	seen := map[string]bool{}
	for _, v := range a {
		seen[v] = true
	}

	for _, v := range b {
		if !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}

	return out
}

func (s *Service) Discard(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v := s.sessions[id]
	if v == nil {
		return nil
	}

	for token := range v.tokens {
		s.media.Discard(ctx, token)
	}

	delete(s.sessions, id)

	return nil
}
