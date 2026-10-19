package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/desktop"
	"github.com/xiaolazhu/vrc_asset_box/internal/localization"
)

func TestNewLibraryCategoriesFollowEffectiveLanguage(t *testing.T) {
	for _, test := range []struct {
		preference, system, expected string
	}{
		{"system", "zh-CN", "衣服"},
		{"system", "zh-Hant-TW", "衣服"},
		{"system", "ja-JP", "衣装"},
		{"system", "ko-KR", "의상"},
		{"system", "fr-FR", "Clothing"},
		{"ja", "zh-CN", "衣装"},
	} {
		t.Run(test.preference+"-"+test.system, func(t *testing.T) {
			t.Setenv("VRC_ASSET_BOX_DATA_DIR", filepath.Join(t.TempDir(), "library"))
			a := NewApp()
			a.language.Store(test.preference)
			a.systemLocale.Store(test.system)
			a.dailyOnce.Do(func() {})
			a.startup(context.Background())
			defer a.shutdown(context.Background())
			if a.startupErr != nil {
				t.Fatal(a.startupErr)
			}
			var name string
			if err := a.db.QueryRow(`SELECT name FROM categories WHERE id='cloth'`).Scan(&name); err != nil || name != test.expected {
				t.Fatalf("initial category %q, want %q; %v", name, test.expected, err)
			}
			if err := a.SetLanguage("en"); err != nil {
				t.Fatal(err)
			}
			if err := a.db.QueryRow(`SELECT name FROM categories WHERE id='cloth'`).Scan(&name); err != nil || name != test.expected {
				t.Fatalf("language change renamed category: %q %v", name, err)
			}
		})
	}
}

func TestLegacyPreferencesDefaultToSystemLanguage(t *testing.T) {
	a := testApp(t)
	if _, err := a.db.Exec("INSERT INTO settings(key,value_json) VALUES('preferences',?)", `{"theme":"dark","view":"cards","cardSize":"medium"}`); err != nil {
		t.Fatal(err)
	}
	p, err := a.GetPreferences()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(p)
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	if got["language"] != "system" {
		t.Fatalf("legacy language = %v, want system", got["language"])
	}
}

func TestRefreshSystemLocaleKeepsPreferenceAndNativeDialogsInSync(t *testing.T) {
	a := testApp(t)
	a.systemLocale.Store("stale-language")
	if err := a.SavePreferences(defaultPreferences()); err != nil {
		t.Fatal(err)
	}

	detected := a.RefreshSystemLocale()
	if detected != desktop.SystemLocale() || a.systemLocale.Load() != detected {
		t.Fatalf("refreshed system locale = %q", detected)
	}
	resolved := localization.Resolve("system", detected)
	if got := a.dialogText("selectCover"); got != localization.Text(resolved, "selectCover") {
		t.Fatalf("native dialog did not follow refreshed locale: %q", got)
	}

	p, err := a.GetPreferences()
	if err != nil || p.Language != "system" {
		t.Fatalf("refresh overwrote preference: %q, %v", p.Language, err)
	}
}

func TestLanguagePreferencePersistenceAndValidation(t *testing.T) {
	a := testApp(t)
	for _, language := range []string{"system", "en", "zh-CN", "zh-TW", "ja", "ko", "invalid"} {
		p := defaultPreferences()
		b, _ := json.Marshal(map[string]string{"language": language})
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
		err := a.SavePreferences(p)
		if language == "invalid" {
			if err == nil {
				t.Fatal("invalid language accepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		var raw string
		if err = a.db.QueryRow("SELECT value_json FROM settings WHERE key='preferences'").Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		_ = json.Unmarshal([]byte(raw), &got)
		if got["language"] != language {
			t.Fatalf("saved language = %v, want %s", got["language"], language)
		}
	}
}

func TestNativeDialogLanguageUpdatesDuringMaintenance(t *testing.T) {
	a := testApp(t)
	a.systemLocale.Store("zh-Hant-HK")
	a.gate.Lock()
	defer a.gate.Unlock()
	if err := a.SetLanguage("system"); err != nil {
		t.Fatal(err)
	}
	if got := a.dialogText("selectCover"); got != "選擇封面" {
		t.Fatalf("system dialog title = %q", got)
	}
	if err := a.SetLanguage("ja"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetLanguage("invalid"); err == nil {
		t.Fatal("invalid immediate language accepted")
	}
	if got := a.dialogText("selectCover"); got != "カバー画像を選択" {
		t.Fatalf("immediate dialog title = %q", got)
	}
}

func TestBootstrapAndNativeLanguageAfterRestart(t *testing.T) {
	a := testApp(t)
	p := defaultPreferences()
	p.Language = "ko"
	if err := a.SavePreferences(p); err != nil {
		t.Fatal(err)
	}
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	a.startup(a.ctx)
	if a.startupErr != nil {
		t.Fatal(a.startupErr)
	}
	b, err := a.GetBootstrap()
	if err != nil || b.SystemLocale != a.systemLocale.Load() || b.Preferences.Language != "ko" {
		t.Fatalf("bootstrap %+v, %v", b, err)
	}
	if got := a.dialogText("selectCover"); got != "표지 선택" {
		t.Fatalf("persisted native dialog title = %q", got)
	}
}
