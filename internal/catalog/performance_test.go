package catalog_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/catalog"
	"github.com/xiaolazhu/vrc_asset_box/internal/database"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func TestScaleAcceptance(t *testing.T) {
	if os.Getenv("VRC_RUN_SCALE") != "1" {
		t.Skip("set VRC_RUN_SCALE=1 for the fixed 10k/200k query acceptance dataset")
	}

	ctx := context.Background()
	db, err := database.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	defer tx.Rollback()

	for i := 0; i < 200; i++ {
		kind := "avatar"
		if i >= 100 {
			kind = "free"
		}

		id := fmt.Sprintf("tag-%d", i)
		name := fmt.Sprintf("Avatar%03d", i)
		if _, err = tx.ExecContext(ctx, "INSERT INTO tags VALUES(?,?,?)", id, kind, name); err != nil {
			t.Fatal(err)
		}

		if _, err = tx.ExecContext(ctx, "INSERT INTO tag_names VALUES(?,?,?,?,1)", id, kind, name, fmt.Sprintf("avatar%03d", i)); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := 0; i < 10000; i++ {
		id := fmt.Sprintf("asset-%05d", i)
		name := fmt.Sprintf("素材 %05d", i)
		if _, err = tx.ExecContext(
			ctx,
			"INSERT INTO assets(id,name,name_key,category_id,universal,notes,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)",
			id,
			name,
			name,
			[]string{"cloth", "hair", "item", "tool"}[i%4],
			i%11 == 0,
			fmt.Sprintf("测试备注 group%d", i%7),
			now,
			now,
		); err != nil {
			t.Fatal(err)
		}

		for j := 0; j < 20; j++ {
			n := (i%10)*10 + j%10
			if j >= 10 {
				n += 100
			}

			if _, err = tx.ExecContext(ctx, "INSERT INTO asset_tags VALUES(?,?)", id, fmt.Sprintf("tag-%d", n)); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}

	s := catalog.New(db)
	query := dto.AssetQuery{
		AvatarMode:  "selected",
		AvatarMatch: "any",
		FreeMatch:   "all",
		Folder:      "all",
		Sort:        "updatedAt",
		Descending:  true,
		Limit:       60,
	}
	if _, err = s.List(ctx, query); err != nil {
		t.Fatal(err)
	}

	durations := []time.Duration{}
	for i := 0; i < 20; i++ {
		q := query
		switch i % 5 {
		case 1:
			q.Keyword = "group3"
		case 2:
			q.AvatarIDs = []string{"tag-1", "tag-11"}
		case 3:
			q.CategoryIDs = []string{"cloth"}
			q.FreeTagIDs = []string{"tag-101"}
		case 4:
			q.AvatarIDs = []string{"tag-2", "tag-3"}
			q.AvatarMatch = "all"
			q.Keyword = "素材"
		}

		start := time.Now()
		if _, err = s.List(ctx, q); err != nil {
			t.Fatal(err)
		}

		durations = append(durations, time.Since(start))
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	p95 := durations[18]
	t.Logf("10,000 assets / 200,000 associations; 20 warmed queries: p95=%s max=%s", p95, durations[19])
	if p95 > 500*time.Millisecond {
		t.Fatalf("query p95 exceeds 500ms target: %s", p95)
	}
}
