package dto

type SheetMapping struct {
	Sheet           string `json:"sheet"`
	Included        bool   `json:"included"`
	CategoryID      string `json:"categoryId"`
	NewCategoryName string `json:"newCategoryName"`
	BoothColumn     string `json:"boothColumn"`
	NameColumn      string `json:"nameColumn"`
	ImageColumn     string `json:"imageColumn"`
	AvatarColumn    string `json:"avatarColumn"`
	NotesColumn     string `json:"notesColumn"`
	FreeTagsColumn  string `json:"freeTagsColumn"`
	FolderColumn    string `json:"folderColumn"`
}

type ImportPreviewRequest struct {
	Path     string         `json:"path"`
	Mappings []SheetMapping `json:"mappings"`
}

type ImportRow struct {
	Key              string       `json:"key"`
	Sheet            string       `json:"sheet"`
	Row              int          `json:"row"`
	Draft            AssetDraft   `json:"draft"`
	ExistingAssetID  string       `json:"existingAssetId"`
	ExistingRowKey   string       `json:"existingRowKey,omitempty"`
	ExistingRevision int          `json:"existingRevision"`
	Issues           []string     `json:"issues"`
	ImageOptions     []MediaDraft `json:"imageOptions"`
}

type ImportPreview struct {
	PreviewID    string         `json:"previewId"`
	SourceSHA256 string         `json:"sourceSHA256"`
	Sheets       []SheetMapping `json:"sheets"`
	Rows         []ImportRow    `json:"rows"`
}

type ImportDecision struct {
	RowKey         string     `json:"rowKey"`
	Action         string     `json:"action"`
	TargetAssetID  string     `json:"targetAssetId"`
	TargetRevision int        `json:"targetRevision"`
	ResolvedDraft  AssetDraft `json:"resolvedDraft"`
}

type ImportCommitRequest struct {
	PreviewID string           `json:"previewId"`
	Decisions []ImportDecision `json:"decisions"`
}

type ImportResult struct {
	Created int `json:"created"`
	Merged  int `json:"merged"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}
