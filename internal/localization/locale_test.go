package localization

import "testing"

func TestResolveSystemLocales(t *testing.T) {
	for input, want := range map[string]string{
		"": "en", "fr-FR": "en", "en-US": "en", "zh_CN.UTF-8": "zh-CN", "zh-SG": "zh-CN",
		"zh-TW": "zh-TW", "zh-Hant": "zh-TW", "zh-HK": "zh-TW", "zh-MO": "zh-TW", "ja-JP": "ja", "ko-KR": "ko",
	} {
		if got := Resolve("system", input); got != want {
			t.Errorf("Resolve(system, %q) = %q, want %q", input, got, want)
		}
		if got := Resolve("ja", input); got != "ja" {
			t.Errorf("explicit preference lost to %q: %q", input, got)
		}
	}
}

func TestAllNativeDialogTranslationsPresent(t *testing.T) {
	for _, language := range []string{"en", "zh-CN", "zh-TW", "ja", "ko"} {
		for key := range messages["en"] {
			if messages[language][key] == "" {
				t.Errorf("missing %s translation for %s", language, key)
			}
		}
	}
	if got := Text("unknown", "selectCover"); got != "Select cover" {
		t.Fatalf("unknown locale fallback = %q", got)
	}
}
