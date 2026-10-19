package localization

import "strings"

func ValidPreference(language string) bool {
	switch language {
	case "system", "en", "zh-CN", "zh-TW", "ja", "ko":
		return true
	}

	return false
}

// Resolve 统一系统区域格式，无法识别的语言回退到英语。
func Resolve(preference, systemLocale string) string {
	if preference != "" && preference != "system" && ValidPreference(preference) {
		return preference
	}

	language := strings.ToLower(strings.ReplaceAll(systemLocale, "_", "-"))
	language = strings.SplitN(language, ".", 2)[0]
	language = strings.SplitN(language, "@", 2)[0]
	switch {
	case language == "zh" || strings.HasPrefix(language, "zh-"):
		if strings.Contains(language, "hant") ||
			strings.HasSuffix(language, "-tw") ||
			strings.HasSuffix(language, "-hk") ||
			strings.HasSuffix(language, "-mo") {
			return "zh-TW"
		}

		return "zh-CN"
	case language == "ja" || strings.HasPrefix(language, "ja-"):
		return "ja"
	case language == "ko" || strings.HasPrefix(language, "ko-"):
		return "ko"
	default:
		return "en"
	}
}

func Text(language, key string) string {
	if value := messages[language][key]; value != "" {
		return value
	}

	if value := messages["en"][key]; value != "" {
		return value
	}

	return key
}

var messages = map[string]map[string]string{
	"en": {
		"Clothing":       "Clothing",
		"Hair":           "Hair",
		"Items":          "Items",
		"Tools":          "Tools",
		"exportExcel":    "Export Excel workbook",
		"selectFolder":   "Select asset folder",
		"selectCover":    "Select cover",
		"images":         "Images (PNG / JPEG / WebP)",
		"selectWorkbook": "Select Excel file",
		"workbooks":      "Excel workbooks",
		"saveReport":     "Save import report",
		"reportFilename": "Asset import report.csv",
		"csvReports":     "CSV reports",
		"exportBackup":   "Export library backup",
		"backups":        "Library backups",
		"selectBackup":   "Select backup to restore",
	},
	"zh-CN": {
		"Clothing":       "衣服",
		"Hair":           "头发",
		"Items":          "物品",
		"Tools":          "工具",
		"exportExcel":    "导出 Excel 工作簿",
		"selectFolder":   "选择素材文件夹",
		"selectCover":    "选择封面",
		"images":         "图片（PNG / JPEG / WebP）",
		"selectWorkbook": "选择 Excel 文件",
		"workbooks":      "Excel 工作簿",
		"saveReport":     "保存导入报告",
		"reportFilename": "素材导入报告.csv",
		"csvReports":     "CSV 报告",
		"exportBackup":   "导出素材库备份",
		"backups":        "素材库备份",
		"selectBackup":   "选择待恢复的备份",
	},
	"zh-TW": {
		"Clothing":       "衣服",
		"Hair":           "頭髮",
		"Items":          "物品",
		"Tools":          "工具",
		"exportExcel":    "匯出 Excel 活頁簿",
		"selectFolder":   "選擇素材資料夾",
		"selectCover":    "選擇封面",
		"images":         "圖片（PNG / JPEG / WebP）",
		"selectWorkbook": "選擇 Excel 檔案",
		"workbooks":      "Excel 活頁簿",
		"saveReport":     "儲存匯入報告",
		"reportFilename": "素材匯入報告.csv",
		"csvReports":     "CSV 報告",
		"exportBackup":   "匯出素材庫備份",
		"backups":        "素材庫備份",
		"selectBackup":   "選擇要還原的備份",
	},
	"ja": {
		"Clothing":       "衣装",
		"Hair":           "髪型",
		"Items":          "アイテム",
		"Tools":          "ツール",
		"exportExcel":    "Excel ブックをエクスポート",
		"selectFolder":   "素材フォルダーを選択",
		"selectCover":    "カバー画像を選択",
		"images":         "画像（PNG / JPEG / WebP）",
		"selectWorkbook": "Excel ファイルを選択",
		"workbooks":      "Excel ブック",
		"saveReport":     "インポートレポートを保存",
		"reportFilename": "素材インポートレポート.csv",
		"csvReports":     "CSV レポート",
		"exportBackup":   "ライブラリのバックアップをエクスポート",
		"backups":        "ライブラリのバックアップ",
		"selectBackup":   "復元するバックアップを選択",
	},
	"ko": {
		"Clothing":       "의상",
		"Hair":           "헤어",
		"Items":          "아이템",
		"Tools":          "도구",
		"exportExcel":    "Excel 통합 문서 내보내기",
		"selectFolder":   "에셋 폴더 선택",
		"selectCover":    "표지 선택",
		"images":         "이미지(PNG / JPEG / WebP)",
		"selectWorkbook": "Excel 파일 선택",
		"workbooks":      "Excel 통합 문서",
		"saveReport":     "가져오기 보고서 저장",
		"reportFilename": "에셋_가져오기_보고서.csv",
		"csvReports":     "CSV 보고서",
		"exportBackup":   "라이브러리 백업 내보내기",
		"backups":        "라이브러리 백업",
		"selectBackup":   "복원할 백업 선택",
	},
}
