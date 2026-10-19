package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

const MaxBytes = 20 << 20

type staged struct {
	draft   dto.MediaDraft
	bytes   []byte
	mime    string
	created time.Time
}

type Service struct {
	Root  string
	DB    *sql.DB
	mu    sync.Mutex
	items map[string]staged
}

func New(root string, db *sql.DB) *Service {
	return &Service{Root: root, DB: db, items: map[string]staged{}}
}

func (s *Service) StagePath(ctx context.Context, path string) (dto.MediaDraft, error) {
	file, err := os.Open(path)
	if err != nil {
		return dto.MediaDraft{}, err
	}

	defer file.Close()

	imageBytes, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil {
		return dto.MediaDraft{}, err
	}

	return s.StageBytes(ctx, imageBytes)
}

func (s *Service) StageBytes(ctx context.Context, imageBytes []byte) (dto.MediaDraft, error) {
	if err := ctx.Err(); err != nil {
		return dto.MediaDraft{}, err
	}

	if len(imageBytes) > MaxBytes {
		return dto.MediaDraft{}, dto.Error("VALIDATION", "Images must not exceed 20 MiB")
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(imageBytes))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 40_000_000 {
		return dto.MediaDraft{}, dto.Error("UNSUPPORTED_FORMAT", "Choose a valid PNG, JPEG or WebP image with no more than 40 million pixels")
	}

	if format != "png" && format != "jpeg" && format != "webp" {
		return dto.MediaDraft{}, dto.Error("UNSUPPORTED_FORMAT", "Only PNG, JPEG and WebP are supported")
	}

	if _, _, err = image.Decode(bytes.NewReader(imageBytes)); err != nil {
		return dto.MediaDraft{}, dto.Error("UNSUPPORTED_FORMAT", "Image is damaged and cannot be decoded")
	}

	token := uuid.NewString()
	draft := dto.MediaDraft{
		Token:      token,
		PreviewURL: "/media/staging/" + token,
		Width:      config.Width,
		Height:     config.Height,
	}
	contentType := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "webp": "image/webp"}[format]

	s.mu.Lock()
	defer s.mu.Unlock()

	for stagedToken, stagedCover := range s.items {
		if time.Since(stagedCover.created) > 24*time.Hour {
			delete(s.items, stagedToken)
		}
	}

	s.items[token] = staged{
		draft:   draft,
		bytes:   append([]byte(nil), imageBytes...),
		mime:    contentType,
		created: time.Now(),
	}

	return draft, nil
}

func (s *Service) Discard(ctx context.Context, token string) error {
	s.mu.Lock()
	delete(s.items, token)
	s.mu.Unlock()

	return nil
}

// PreparedCover 持有已落盘的图片文件；InsertInTx 将其元数据与所属素材原子提交。
// 未被引用的文件可在之后安全清理。
type PreparedCover struct {
	ID        string
	Hash      string
	Original  string
	Thumbnail string
	MIME      string
	Width     int
	Height    int
}

func (p PreparedCover) InsertInTx(ctx context.Context, tx *sql.Tx) (string, error) {
	var id string

	err := tx.QueryRowContext(ctx, "SELECT id FROM media WHERE sha256=?", p.Hash).Scan(&id)
	if err == nil {
		return id, nil
	}

	if err != sql.ErrNoRows {
		return "", err
	}

	_, err = tx.ExecContext(
		ctx,
		"INSERT INTO media(id,sha256,original_path,thumbnail_path,mime,width,height,created_at) VALUES(?,?,?,?,?,?,?,?)",
		p.ID,
		p.Hash,
		p.Original,
		p.Thumbnail,
		p.MIME,
		p.Width,
		p.Height,
		time.Now().UTC().Format(time.RFC3339Nano),
	)

	return p.ID, err
}

func (s *Service) CommitCover(ctx context.Context, token string) (string, error) {
	preparedCover, err := s.PrepareCover(ctx, token)
	if err != nil {
		return "", err
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}

	defer tx.Rollback()

	id, err := preparedCover.InsertInTx(ctx, tx)
	if err != nil {
		return "", err
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}

	return id, nil
}

func (s *Service) PrepareCover(ctx context.Context, token string) (PreparedCover, error) {
	if err := ctx.Err(); err != nil {
		return PreparedCover{}, err
	}

	s.mu.Lock()
	stagedCover, ok := s.items[token]
	s.mu.Unlock()
	if !ok {
		return PreparedCover{}, dto.Error("NOT_FOUND", "Image preview expired. Choose it again.")
	}

	sum := sha256.Sum256(stagedCover.bytes)
	hash := hex.EncodeToString(sum[:])
	originalImage, _, err := image.Decode(bytes.NewReader(stagedCover.bytes))
	if err != nil {
		return PreparedCover{}, err
	}

	// 保持宽高比，将较长边缩到 480 像素；小图不放大。
	width, height := stagedCover.draft.Width, stagedCover.draft.Height
	if width > 480 || height > 480 {
		if width >= height {
			height = max(1, height*480/width)
			width = 480
		} else {
			width = max(1, width*480/height)
			height = 480
		}
	}

	thumbnailImage := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(thumbnailImage, thumbnailImage.Bounds(), originalImage, originalImage.Bounds(), draw.Over, nil)

	// 原图与缩略图使用内容哈希命名，数据库事务只发布引用。
	ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}[stagedCover.mime]
	original := filepath.Join("media", hash+ext)
	thumbnail := filepath.Join("media", hash+"-thumb.png")
	if err = os.MkdirAll(filepath.Join(s.Root, "media"), 0700); err != nil {
		return PreparedCover{}, err
	}

	if err = atomicWrite(filepath.Join(s.Root, original), stagedCover.bytes); err != nil {
		return PreparedCover{}, err
	}

	var thumbnailBytes bytes.Buffer
	if err = png.Encode(&thumbnailBytes, thumbnailImage); err != nil {
		return PreparedCover{}, err
	}

	if err = atomicWrite(filepath.Join(s.Root, thumbnail), thumbnailBytes.Bytes()); err != nil {
		return PreparedCover{}, err
	}

	return PreparedCover{
		ID:        uuid.NewString(),
		Hash:      hash,
		Original:  original,
		Thumbnail: thumbnail,
		MIME:      stagedCover.mime,
		Width:     stagedCover.draft.Width,
		Height:    stagedCover.draft.Height,
	}, nil
}

func atomicWrite(filename string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(filename), "image-*")
	if err != nil {
		return err
	}

	name := tmp.Name()
	defer os.Remove(name)

	if _, err = tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}

	if err = tmp.Close(); err != nil {
		return err
	}

	if _, err = os.Stat(filename); err == nil {
		return nil
	}

	return os.Rename(name, filename)
}

func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/media/"), "/")
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("X-Content-Type-Options", "nosniff")
		if parts[0] == "staging" {
			s.mu.Lock()
			stagedCover, ok := s.items[parts[1]]
			s.mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}

			w.Header().Set("Content-Type", stagedCover.mime)
			w.Write(stagedCover.bytes)

			return
		}

		column := "original_path"
		if parts[1] == "thumbnail" {
			column = "thumbnail_path"
		} else if parts[1] != "original" {
			http.NotFound(w, r)
			return
		}

		var relativePath, contentType string
		if err := s.DB.QueryRowContext(r.Context(), fmt.Sprintf("SELECT %s,mime FROM media WHERE id=?", column), parts[0]).Scan(&relativePath, &contentType); err != nil {
			http.NotFound(w, r)
			return
		}

		absolute := filepath.Join(s.Root, relativePath)
		check, err := filepath.Rel(filepath.Join(s.Root, "media"), absolute)
		if err != nil || strings.HasPrefix(check, "..") || filepath.IsAbs(check) {
			http.NotFound(w, r)
			return
		}

		if column == "thumbnail_path" {
			contentType = "image/png"
		}

		w.Header().Set("Content-Type", contentType)
		http.ServeFile(w, r, absolute)
	})
}

// 为下载的封面图片保留 JPEG 解码器注册。
var _ = jpeg.DefaultQuality
