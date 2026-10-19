package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func testApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("VRC_ASSET_BOX_DATA_DIR", filepath.Join(t.TempDir(), "资料库"))

	a := NewApp()
	a.dailyOnce.Do(func() {})
	a.startup(context.Background())
	if a.startupErr != nil {
		t.Fatal(a.startupErr)
	}

	t.Cleanup(func() {
		a.shutdown(context.Background())
	})

	return a
}

func TestAppManualPersistenceAndPreferences(t *testing.T) {
	a := testApp(t)
	b, err := a.GetBootstrap()
	if err != nil || len(b.Categories) != 4 {
		t.Fatalf("bootstrap %+v %v", b, err)
	}

	for _, name := range []string{"手工素材 A", "非 Booth 素材 B"} {
		saved, err := a.SaveAsset(dto.AssetDraft{Name: name, AvatarIDs: []string{}, FreeTagIDs: []string{}})
		if err != nil || saved.BoothID != "" {
			t.Fatalf("manual save %+v %v", saved, err)
		}
	}

	prefs := defaultPreferences()
	prefs.Theme = "dark"
	prefs.View = "table"
	prefs.Query.Keyword = "素材"
	if err = a.SavePreferences(prefs); err != nil {
		t.Fatal(err)
	}

	if err = a.db.Close(); err != nil {
		t.Fatal(err)
	}

	a.startup(context.Background())

	page, err := a.ListAssets(defaultPreferences().Query)
	if err != nil || page.Total != 2 {
		t.Fatalf("persisted page %+v %v", page, err)
	}

	got, err := a.GetPreferences()
	if err != nil || got.Theme != "dark" || got.View != "table" || got.Query.Keyword != "素材" {
		t.Fatalf("preferences %+v %v", got, err)
	}
}

func TestAppBusyProtectsActiveGeneration(t *testing.T) {
	a := testApp(t)
	a.gate.Lock()

	_, err := a.GetBootstrap()
	_, saveErr := a.SaveAsset(dto.AssetDraft{Name: "blocked"})
	a.gate.Unlock()
	if err == nil || saveErr == nil {
		t.Fatal("maintenance accepted business call")
	}

	if _, err = a.GetBootstrap(); err != nil {
		t.Fatal(err)
	}
}

func TestOfflineFolderNeedsExplicitConsent(t *testing.T) {
	a := testApp(t)
	if _, err := a.SaveAsset(dto.AssetDraft{Name: "invalid relative", FolderPath: "relative", AllowOfflineFolder: true}); err == nil {
		t.Fatal("offline consent bypassed absolute path rule")
	}

	path := filepath.Join(t.TempDir(), "missing")
	if _, err := a.SaveAsset(dto.AssetDraft{Name: "离线素材", FolderPath: path}); err == nil {
		t.Fatal("offline path saved without consent")
	}

	if _, err := a.SaveAsset(dto.AssetDraft{Name: "离线素材", FolderPath: path, AllowOfflineFolder: true}); err != nil {
		t.Fatal(err)
	}
}

func TestWebViewDataLivesUnderApplicationDataDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "VRCAssetBox")
	t.Setenv("VRC_ASSET_BOX_DATA_DIR", root)

	got, err := webviewDataPath()
	if err != nil {
		t.Fatal(err)
	}

	if want := root; got != want {
		t.Fatalf("WebView data path = %q, want %q", got, want)
	}
}

func TestBoothLookupFailureDoesNotStartNetworkJob(t *testing.T) {
	a := testApp(t)
	if _, err := a.db.Exec("DROP TABLE assets"); err != nil {
		t.Fatal(err)
	}

	if job, err := a.StartBooth("12345", ""); err == nil || job.ID != "" {
		t.Fatalf("storage failure started Booth job: %+v %v", job, err)
	}
}

func TestRestorePreflightFailurePreservesDraftServices(t *testing.T) {
	a := testApp(t)
	path := filepath.Join(t.TempDir(), "invalid.zip")
	if err := os.WriteFile(path, []byte("invalid archive"), 0600); err != nil {
		t.Fatal(err)
	}

	hash, err := hashFile(path)
	if err != nil {
		t.Fatal(err)
	}

	a.inspections["test"] = backupInspection{path: path, hash: hash}
	beforeDB, beforeMedia, beforeMigration := a.db, a.media, a.migration
	job, err := a.StartRestore("test")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		state, err := a.GetJob(job.ID)
		if err != nil {
			t.Fatal(err)
		}

		if state.Status != "running" {
			if state.Status != "failed" {
				t.Fatalf("unexpected restore result: %+v", state)
			}

			break
		}

		if time.Now().After(deadline) {
			t.Fatal("restore did not finish")
		}

		time.Sleep(5 * time.Millisecond)
	}

	if a.db != beforeDB || a.media != beforeMedia || a.migration != beforeMigration {
		t.Fatal("preflight failure discarded active draft/preview services")
	}

	if _, err := a.GetBootstrap(); err != nil {
		t.Fatal(err)
	}
}
