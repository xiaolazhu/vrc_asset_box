package media_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http/httptest"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/database"
	"github.com/xiaolazhu/vrc_asset_box/internal/media"
)

func TestCoverSurvivesStagingAndRejectsBadInput(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := database.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	s := media.New(root, db)

	var b bytes.Buffer

	png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 100, 80)))

	m, err := s.StageBytes(ctx, b.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	if m.Width != 100 || m.Height != 80 {
		t.Fatal(m)
	}

	id, err := s.CommitCover(ctx, m.Token)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	s.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/media/"+id+"/original", nil))
	if r.Code != 200 || len(r.Body.Bytes()) == 0 {
		t.Fatal(r.Code)
	}

	if _, err = s.StageBytes(ctx, []byte("not an image")); err == nil {
		t.Fatal("invalid image accepted")
	}

	r = httptest.NewRecorder()
	s.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/media/../../library.db", nil))
	if r.Code == 200 {
		t.Fatal("path escape")
	}
}
