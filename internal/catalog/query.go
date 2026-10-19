package catalog

import (
	"context"
	"database/sql"
	"net/url"
	"strings"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/tags"
)

// boothLink 仅识别公开的 Booth 商品链接，其他文本仍按普通字面内容搜索。
// 仅输入关键词不会发起网络请求。
func boothLink(input string) string {
	u, err := url.Parse(input)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}

	host := strings.ToLower(u.Hostname())
	if host != "booth.pm" && !strings.HasSuffix(host, ".booth.pm") {
		return ""
	}

	if u.User != nil || u.Port() != "" {
		return ""
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 3 && parts[0] == "zh-cn" {
		parts = parts[1:]
	} else if len(parts) == 3 && (parts[0] == "en" || parts[0] == "ja" || parts[0] == "ko" || parts[0] == "zh-tw") {
		parts = parts[1:]
	}

	if len(parts) == 2 && parts[0] == "items" && digits(parts[1]) {
		return parts[1]
	}

	return ""
}

func queryWhere(query dto.AssetQuery) (string, []any, string, error) {
	clauses := []string{"1=1"}
	args := []any{}
	keyword := tags.NormalizeName(query.Keyword)
	priority := ""

	// 商品链接按 ID 精确匹配；普通输入按词组合搜索。
	if id := boothLink(strings.TrimSpace(query.Keyword)); id != "" {
		clauses = append(clauses, "a.booth_id=?")
		args = append(args, id)
	} else {
		terms := strings.Fields(keyword)
		if len(terms) == 1 && digits(terms[0]) {
			priority = terms[0]
		}

		for _, term := range terms {
			// instr 将 %、_ 和反斜杠按字面内容匹配，无需处理 LIKE 转义规则。
			clauses = append(clauses, `(instr(a.name_key,?)>0 OR instr(lower(a.notes),?)>0 OR instr(COALESCE(a.booth_id,''),?)>0 OR EXISTS(SELECT 1 FROM asset_tags kt JOIN tag_names kn ON kn.tag_id=kt.tag_id WHERE kt.asset_id=a.id AND instr(kn.name_key,?)>0))`)
			args = append(args, term, term, term, term)
		}
	}

	// 分类采用 OR 组合，随后与其他筛选组做 AND 组合。
	categories := unique(query.CategoryIDs)
	if len(categories) > 0 || query.Uncategorized {
		categoryClauses := []string{}
		if len(categories) > 0 {
			categoryClauses = append(categoryClauses, "a.category_id IN ("+placeholders(len(categories))+")")
			args = append(args, idArgs(categories)...)
		}

		if query.Uncategorized {
			categoryClauses = append(categoryClauses, "a.category_id IS NULL")
		}

		clauses = append(clauses, "("+strings.Join(categoryClauses, " OR ")+")")
	}

	if query.AvatarMatch != "" && query.AvatarMatch != "any" && query.AvatarMatch != "all" {
		return "", nil, "", validation("avatarMatch", "Invalid avatar matching method")
	}

	if query.FreeMatch != "" && query.FreeMatch != "any" && query.FreeMatch != "all" {
		return "", nil, "", validation("freeMatch", "Invalid custom tag matching method")
	}
	if query.FreeMode != "" && query.FreeMode != "include" && query.FreeMode != "exclude" {
		return "", nil, "", validation("freeMode", "Invalid custom tag filter mode")
	}

	// 通用素材始终参与具体 Avatar 的 any / all 筛选。
	switch query.AvatarMode {
	case "", "selected":
		ids := unique(query.AvatarIDs)
		if len(ids) > 0 {
			condition := tagFilter("avatar", ids, query.AvatarMatch)
			clauses = append(clauses, "(a.universal=1 OR "+condition+")")
			args = append(args, idArgs(ids)...)
			if query.AvatarMatch == "all" {
				args = append(args, len(ids))
			}
		}
	case "universal":
		clauses = append(clauses, "a.universal=1")
	case "unknown":
		clauses = append(clauses, `a.universal=0 AND NOT EXISTS(SELECT 1 FROM asset_tags at JOIN tags t ON t.id=at.tag_id WHERE at.asset_id=a.id AND t.kind='avatar')`)
	default:
		return "", nil, "", validation("avatarMode", "Invalid avatar filter mode")
	}

	// 自由标签与 Avatar 分开匹配。
	freeTagIDs := unique(query.FreeTagIDs)
	if len(freeTagIDs) > 0 {
		condition := tagFilter("free", freeTagIDs, query.FreeMatch)
		if query.FreeMode == "exclude" {
			condition = "NOT (" + condition + ")"
		}
		clauses = append(clauses, condition)
		args = append(args, idArgs(freeTagIDs)...)
		if query.FreeMatch == "all" {
			args = append(args, len(freeTagIDs))
		}
	}

	switch query.Folder {
	case "", "all":
	case "linked":
		clauses = append(clauses, "a.folder_path<>''")
	case "unlinked":
		clauses = append(clauses, "a.folder_path=''")
	default:
		return "", nil, "", validation("folder", "Invalid folder filter")
	}

	return strings.Join(clauses, " AND "), args, priority, nil
}

func tagFilter(kind string, ids []string, match string) string {
	predicate := `FROM asset_tags at JOIN tags t ON t.id=at.tag_id WHERE at.asset_id=a.id AND t.kind='` + kind + `' AND at.tag_id IN (` + placeholders(len(ids)) + `)`
	if match == "all" {
		return "(SELECT count(DISTINCT at.tag_id) " + predicate + ")=?"
	}

	return "EXISTS(SELECT 1 " + predicate + ")"
}

func queryOrder(query dto.AssetQuery, priority string) (string, []any, error) {
	direction := " ASC"
	if query.Descending || query.Sort == "" {
		direction = " DESC"
	}

	parts := []string{}
	args := []any{}
	if priority != "" {
		parts = append(parts, "(a.booth_id=?) DESC")
		args = append(args, priority)
	}

	switch query.Sort {
	case "", "createdAt":
		parts = append(parts, "a.created_at"+direction)
	case "updatedAt":
		parts = append(parts, "a.updated_at"+direction)
	case "name":
		parts = append(parts, "a.name_key"+direction)
	case "boothId":
		// 使用 ltrim 处理前导零，避免将任意长度的数字字符串转换为机器整数。
		parts = append(parts, "(a.booth_id IS NULL) ASC", "length(ltrim(a.booth_id,'0'))"+direction, "ltrim(a.booth_id,'0')"+direction)
	default:
		return "", nil, validation("sort", "Invalid sort field")
	}

	parts = append(parts, "a.id ASC")

	return strings.Join(parts, ","), args, nil
}

func (s *Service) List(ctx context.Context, query dto.AssetQuery) (dto.AssetPage, error) {
	page := dto.AssetPage{Items: []dto.Asset{}}
	if query.Offset < 0 || query.Limit < 0 {
		return page, validation("limit", "Pagination parameters cannot be negative")
	}

	if query.Limit == 0 {
		query.Limit = 60
	}

	where, args, priority, err := queryWhere(query)
	if err != nil {
		return page, err
	}

	order, orderArgs, err := queryOrder(query, priority)
	if err != nil {
		return page, err
	}

	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return page, assetStorage(err)
	}

	defer tx.Rollback()

	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM assets a WHERE "+where, args...).Scan(&page.Total); err != nil {
		return page, assetStorage(err)
	}

	values := append(append([]any{}, args...), orderArgs...)
	values = append(values, query.Limit, query.Offset)
	rows, err := tx.QueryContext(ctx, "SELECT "+assetColumns+" FROM assets a WHERE "+where+" ORDER BY "+order+" LIMIT ? OFFSET ?", values...)
	if err != nil {
		return page, assetStorage(err)
	}

	for rows.Next() {
		asset, scanErr := scanAsset(rows)
		if scanErr != nil {
			rows.Close()
			return page, assetStorage(scanErr)
		}

		page.Items = append(page.Items, asset)
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, assetStorage(err)
	}

	if err = loadAssetTags(ctx, tx, page.Items); err != nil {
		return page, err
	}

	if err = tx.Commit(); err != nil {
		return page, assetStorage(err)
	}

	return page, nil
}
