package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/xiaolazhu/vrc_asset_box/internal/catalog"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/media"
	"github.com/xiaolazhu/vrc_asset_box/internal/tags"
)

func sourceHash(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}

	defer f.Close()

	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *Service) Commit(ctx context.Context, req dto.ImportCommitRequest) (dto.ImportResult, error) {
	return s.CommitWithProgress(ctx, req, nil)
}

func (s *Service) CommitWithProgress(ctx context.Context, req dto.ImportCommitRequest, progress Progress) (result dto.ImportResult, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v := s.sessions[req.PreviewID]
	if v == nil || time.Since(v.created) > 24*time.Hour {
		return result, dto.Error("NOT_FOUND", "Preview expired. Generate a new preview.")
	}

	if v.committed {
		return result, dto.Error("STALE_DRAFT", "This preview has already been committed. Generate a new preview.")
	}

	// 先校验全部决定，再生成逐行报告；此阶段不写资料库。
	selected := map[string]dto.ImportDecision{}
	validKeys := map[string]bool{}
	for _, row := range v.preview.Rows {
		validKeys[row.Key] = true
	}

	for _, d := range req.Decisions {
		if !validKeys[d.RowKey] || selected[d.RowKey].RowKey != "" {
			return result, dto.Error("VALIDATION", "Import decisions contain unknown or duplicate rows")
		}

		if d.Action != "create" && d.Action != "merge" && d.Action != "skip" {
			return result, dto.Error("VALIDATION", "Choose Add, Merge or Skip")
		}

		selected[d.RowKey] = clone(d)
	}

	v.report = nil
	writeCount := 0
	for _, row := range v.preview.Rows {
		d, ok := selected[row.Key]
		if !ok {
			d = dto.ImportDecision{RowKey: row.Key, Action: "skip"}
			selected[row.Key] = d
		}

		status := "pending"
		reason := strings.Join(row.Issues, "\n")
		if d.Action == "skip" {
			status = "skipped"
			if reason != "" {
				reason += "\n"
			}

			reason += "Skipped by user or unresolved"
			result.Skipped++
		} else {
			writeCount++
		}

		v.report = append(v.report, reportRow{
			Sheet:  row.Sheet,
			Row:    row.Row,
			Action: d.Action,
			Result: status,
			Reason: reason,
			Name:   row.Draft.Name,
		})
	}

	// 事务失败时，报告必须反映整批回滚，而非部分成功。
	defer func() {
		if err != nil {
			result.Created = 0
			result.Merged = 0
			result.Failed = writeCount
			for i := range v.report {
				if v.report[i].Result != "skipped" {
					v.report[i].Result = "failed"
					if v.report[i].Reason != "" {
						v.report[i].Reason += "\n"
					}

					v.report[i].Reason += "This batch was not committed: " + err.Error()
				}
			}
		}
	}()

	if e := ctx.Err(); e != nil {
		return result, e
	}

	hash, e := sourceHash(v.path)
	if e != nil {
		return result, e
	}

	if hash != v.preview.SourceSHA256 {
		return result, dto.Error("STALE_SOURCE", "Source file has changed. Generate a new preview.")
	}

	if writeCount == 0 {
		v.committed = true
		for token := range v.tokens {
			s.media.Discard(context.Background(), token)
		}

		return result, nil
	}
	// 创建文件或备份前，先解析全部操作并检查目标记录的修订号。
	baseRevisions := map[string]int{}
	priorRows := map[string]dto.ImportDecision{}
	for _, row := range v.preview.Rows {
		d := selected[row.Key]
		if d.Action == "skip" {
			continue
		}

		if strings.TrimSpace(d.ResolvedDraft.Name) == "" {
			return result, dto.Error("VALIDATION", "A row to import has an empty name")
		}

		if token := d.ResolvedDraft.CoverToken; token != "" {
			if !v.tokens[token] {
				return result, dto.Error("VALIDATION", "Cover must come from this preview")
			}
		}

		if d.Action == "create" {
			if row.ExistingAssetID != "" {
				return result, dto.Error("DUPLICATE_BOOTH", "Source row already has a record. Merge or skip it.")
			}

			if d.TargetAssetID != "" || d.ResolvedDraft.ID != "" {
				return result, dto.Error("VALIDATION", "Cannot specify an existing asset when creating one")
			}
		}

		if d.Action == "merge" {
			if strings.HasPrefix(d.TargetAssetID, "row:") {
				prior, ok := priorRows[strings.TrimPrefix(d.TargetAssetID, "row:")]
				if !ok ||
					d.TargetRevision != 0 ||
					strings.TrimSpace(d.ResolvedDraft.BoothID) == "" ||
					strings.TrimSpace(d.ResolvedDraft.BoothID) != strings.TrimSpace(prior.ResolvedDraft.BoothID) {
					return result, dto.Error("VALIDATION", "The merge target must be an earlier row selected for creation or merging with the same Booth ID")
				}

				priorRows[row.Key] = d
				continue
			}

			if d.TargetAssetID == "" || d.TargetRevision < 1 {
				return result, dto.Error("VALIDATION", "Merging requires a target asset and version")
			}

			var revision int

			e = s.db.QueryRowContext(ctx, "SELECT revision FROM assets WHERE id=?", d.TargetAssetID).Scan(&revision)
			if e == sql.ErrNoRows {
				return result, dto.Error("STALE_DRAFT", "Merge target not found. Generate a new preview.")
			}

			if e != nil {
				return result, e
			}

			if revision != d.TargetRevision {
				return result, dto.Error("STALE_DRAFT", "Merge target has changed. Generate a new preview.")
			}

			baseRevisions[d.TargetAssetID] = revision
		}

		priorRows[row.Key] = d
	}

	var existing int
	if e = s.db.QueryRowContext(ctx, "SELECT count(*) FROM assets").Scan(&existing); e != nil {
		return result, e
	}

	if existing > 0 {
		if s.backup == nil {
			return result, dto.Error("STORAGE", "Backup service unavailable before import")
		}

		if progress != nil {
			progress("Back up before import", 0, writeCount, true)
		}

		if e = s.backup.Automatic(ctx, "critical"); e != nil {
			return result, fmt.Errorf("Backup before import failed: %w", e)
		}
	}

	prepared := map[string]media.PreparedCover{}
	for _, row := range v.preview.Rows {
		d := selected[row.Key]
		if d.Action == "skip" || d.ResolvedDraft.CoverToken == "" {
			continue
		}

		token := d.ResolvedDraft.CoverToken
		if _, ok := prepared[token]; ok {
			continue
		}

		p, e := s.media.PrepareCover(ctx, token)
		if e != nil {
			return result, e
		}

		prepared[token] = p
		if progress != nil {
			progress("Processing images…", len(prepared), writeCount, true)
		}
	}

	if e = ctx.Err(); e != nil {
		return result, e
	}

	hash, e = sourceHash(v.path)
	if e != nil {
		return result, e
	}

	if hash != v.preview.SourceSHA256 {
		return result, dto.Error("STALE_SOURCE", "Source file has changed. Generate a new preview.")
	}
	// 一旦开始写入数据库，就必须保证原子提交，此阶段不允许取消。
	if progress != nil {
		progress("Saving…", 0, writeCount, false)
	}

	commitCtx := context.WithoutCancel(ctx)
	tx, e := s.db.BeginTx(commitCtx, nil)
	if e != nil {
		return result, e
	}

	defer tx.Rollback()

	for id, revision := range baseRevisions {
		var actual int
		if e = tx.QueryRowContext(commitCtx, "SELECT revision FROM assets WHERE id=?", id).Scan(&actual); e != nil {
			return result, e
		}

		if actual != revision {
			return result, dto.Error("STALE_DRAFT", "Merge target has changed. Generate a new preview.")
		}
	}

	for _, row := range v.preview.Rows {
		if selected[row.Key].Action != "create" {
			continue
		}

		var owner string

		e = tx.QueryRowContext(
			commitCtx,
			"SELECT asset_id FROM import_sources WHERE source_sha256=? AND sheet=? AND row_number=?",
			v.preview.SourceSHA256,
			row.Sheet,
			row.Row,
		).Scan(&owner)
		if e == nil {
			return result, dto.Error("STALE_DRAFT", "Source row has already been imported. Generate a new preview.")
		}

		if e != sql.ErrNoRows {
			return result, e
		}
	}

	runID := uuid.NewString()
	if _, e = tx.ExecContext(
		commitCtx,
		"INSERT INTO import_runs(id,source_sha256,imported_at) VALUES(?,?,?)",
		runID,
		v.preview.SourceSHA256,
		time.Now().UTC().Format(time.RFC3339Nano),
	); e != nil {
		return result, e
	}

	categories := map[string]string{}
	for _, m := range v.preview.Sheets {
		if !m.Included || m.NewCategoryName == "" {
			continue
		}

		key := "new-category:" + m.Sheet

		var id string

		e = tx.QueryRowContext(commitCtx, "SELECT id FROM categories WHERE name_key=?", tags.NormalizeName(m.NewCategoryName)).Scan(&id)
		if e == sql.ErrNoRows {
			id = uuid.NewString()
			_, e = tx.ExecContext(
				commitCtx,
				"INSERT INTO categories(id,name,name_key,position) VALUES(?,?,?,(SELECT COALESCE(MAX(position),-1)+1 FROM categories))",
				id,
				m.NewCategoryName,
				tags.NormalizeName(m.NewCategoryName),
			)
		}

		if e != nil {
			return result, e
		}

		categories[key] = id
	}

	orderedDecisions := make([]dto.ImportDecision, 0, len(v.preview.Rows))
	for _, row := range v.preview.Rows {
		orderedDecisions = append(orderedDecisions, selected[row.Key])
	}

	tagIDs, e := createTags(commitCtx, tx, orderedDecisions)
	if e != nil {
		return result, e
	}

	mediaIDs := map[string]string{}
	for token, p := range prepared {
		id, e := p.InsertInTx(commitCtx, tx)
		if e != nil {
			return result, e
		}

		mediaIDs[token] = id
	}

	revisions := map[string]int{}
	for id, revision := range baseRevisions {
		revisions[id] = revision
	}

	baselines := map[string]dto.AssetDraft{}
	accumulated := map[string]dto.AssetDraft{}
	rowTargets := map[string]string{}
	done := 0
	for i, row := range v.preview.Rows {
		decision := selected[row.Key]
		if decision.Action == "skip" {
			continue
		}

		d := decision.ResolvedDraft
		if decision.Action == "merge" {
			d.ID = decision.TargetAssetID
			if strings.HasPrefix(d.ID, "row:") {
				d.ID = rowTargets[strings.TrimPrefix(d.ID, "row:")]
				if d.ID == "" {
					return result, dto.Error("VALIDATION", "The preceding target row has not been committed")
				}
			}

			d.Revision = revisions[d.ID]
		} else {
			d.ID = ""
			d.Revision = 0
		}

		if strings.HasPrefix(d.CategoryID, "new-category:") {
			mapped := categories[d.CategoryID]
			if mapped == "" {
				return result, dto.Error("VALIDATION", "New category does not belong to this preview")
			}

			d.CategoryID = mapped
		}

		if d.CoverToken != "" {
			d.CoverID = mediaIDs[d.CoverToken]
			d.CoverToken = ""
		}

		d.AvatarIDs = replaceIDs(d.AvatarIDs, tagIDs)
		d.FreeTagIDs = replaceIDs(d.FreeTagIDs, tagIDs)
		d.NewTags = nil
		d.Name = strings.TrimSpace(d.Name)
		d.BoothID = strings.TrimSpace(d.BoothID)
		if decision.Action == "merge" {
			current, ok := accumulated[d.ID]
			if !ok {
				current, e = loadDraftInTx(commitCtx, tx, d.ID)
				if e != nil {
					return result, e
				}

				baselines[d.ID] = current
				revisions[d.ID] = current.Revision
				d.Revision = current.Revision
			}

			d, e = accumulateDraft(baselines[d.ID], current, d)
			if e != nil {
				return result, fmt.Errorf("%s row %d: %w", row.Sheet, row.Row, e)
			}
		}

		id, e := catalog.New(s.db).SaveInTx(commitCtx, tx, d)
		if e != nil {
			return result, fmt.Errorf("%s row %d: %w", row.Sheet, row.Row, e)
		}

		_, e = tx.ExecContext(
			commitCtx,
			`INSERT INTO import_sources(source_sha256,sheet,row_number,asset_id,run_id) VALUES(?,?,?,?,?) ON CONFLICT(source_sha256,sheet,row_number) DO UPDATE SET asset_id=excluded.asset_id,run_id=excluded.run_id`,
			v.preview.SourceSHA256,
			row.Sheet,
			row.Row,
			id,
			runID,
		)
		if e != nil {
			return result, e
		}

		if decision.Action == "create" {
			result.Created++

			revisions[id] = 1
		} else {
			result.Merged++
			revisions[id]++
		}

		d.ID = id
		d.Revision = revisions[id]
		accumulated[id] = d
		rowTargets[row.Key] = id
		if decision.Action == "create" {
			baselines[id] = dto.AssetDraft{}
		}

		v.report[i].Result = "success"
		done++
		if progress != nil {
			progress("Saving…", done, writeCount, false)
		}
	}

	if e = tx.Commit(); e != nil {
		return result, e
	}

	v.committed = true
	for token := range v.tokens {
		s.media.Discard(context.Background(), token)
	}

	return result, nil
}

func replaceIDs(ids []string, replacements map[string]string) []string {
	out := append([]string{}, ids...)
	for i, id := range out {
		if v := replacements[id]; v != "" {
			out[i] = v
		}
	}

	return out
}

func createTags(ctx context.Context, tx *sql.Tx, decisions []dto.ImportDecision) (map[string]string, error) {
	replacements := map[string]string{}
	keys := map[string]string{}
	batchNames := map[string]string{}
	for _, decision := range decisions {
		if decision.Action == "skip" {
			continue
		}

		for _, d := range decision.ResolvedDraft.NewTags {
			if strings.TrimSpace(d.Key) == "" ||
				d.Kind != "avatar" &&
					d.Kind != "free" ||
				d.Kind == "free" &&
					len(d.Aliases) > 0 {
				return nil, dto.Error("VALIDATION", "Invalid new tag definition")
			}

			names := append([]string{d.Name}, d.Aliases...)
			canonical := d.Kind + ":" + tags.NormalizeName(d.Name)
			signature := canonical
			seen := map[string]bool{}
			for _, name := range names {
				key := d.Kind + ":" + tags.NormalizeName(name)
				if tags.NormalizeName(name) == "" || seen[key] {
					return nil, dto.Error("TAG_CONFLICT", "Tag name or alias is empty or duplicated")
				}

				seen[key] = true
				signature += "\x00" + key
			}

			if old, ok := keys[d.Key]; ok && old != signature {
				return nil, dto.Error("TAG_CONFLICT", "The same temporary tag key points to different names")
			}

			keys[d.Key] = signature
			if id := batchNames[canonical]; id != "" {
				if replacements[d.Key] != "" && replacements[d.Key] != id {
					return nil, dto.Error("TAG_CONFLICT", "Temporary tag key conflict")
				}

				var actual int

				for _, name := range names {
					if e := tx.QueryRowContext(ctx, "SELECT count(*) FROM tag_names WHERE tag_id=? AND kind=? AND name_key=?", id, d.Kind, tags.NormalizeName(name)).Scan(&actual); e != nil {
						return nil, e
					}

					if actual != 1 {
						return nil, dto.Error("TAG_CONFLICT", "New tags with the same name have inconsistent alias definitions")
					}
				}

				replacements[d.Key] = id
				continue
			}

			id := uuid.NewString()
			for _, name := range names {
				var owner string

				e := tx.QueryRowContext(ctx, "SELECT tag_id FROM tag_names WHERE kind=? AND name_key=?", d.Kind, tags.NormalizeName(name)).Scan(&owner)
				if e == nil {
					return nil, dto.Error("TAG_CONFLICT", "Tags or aliases have changed. Generate a new preview.")
				}

				if e != sql.ErrNoRows {
					return nil, e
				}
			}

			if _, e := tx.ExecContext(ctx, "INSERT INTO tags(id,kind,name) VALUES(?,?,?)", id, d.Kind, strings.TrimSpace(d.Name)); e != nil {
				return nil, e
			}

			for i, name := range names {
				if _, e := tx.ExecContext(
					ctx,
					"INSERT INTO tag_names(tag_id,kind,name,name_key,canonical) VALUES(?,?,?,?,?)",
					id,
					d.Kind,
					strings.TrimSpace(name),
					tags.NormalizeName(name),
					i == 0,
				); e != nil {
					return nil, e
				}
			}

			batchNames[canonical] = id
			replacements[d.Key] = id
		}
	}

	return replacements, nil
}
