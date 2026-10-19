package dto

type Tag struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
	Count   int      `json:"count"`
}

type Category struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
	Count    int    `json:"count"`
}

type NewTag struct {
	Key     string   `json:"key"`
	Kind    string   `json:"kind"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

type AssetDraft struct {
	ID                 string   `json:"id"`
	Revision           int      `json:"revision"`
	Name               string   `json:"name"`
	BoothID            string   `json:"boothId"`
	CategoryID         string   `json:"categoryId"`
	CoverID            string   `json:"coverId"`
	CoverToken         string   `json:"coverToken"`
	Universal          bool     `json:"universal"`
	Notes              string   `json:"notes"`
	FolderPath         string   `json:"folderPath"`
	AllowOfflineFolder bool     `json:"allowOfflineFolder"`
	AvatarIDs          []string `json:"avatarIds"`
	FreeTagIDs         []string `json:"freeTagIds"`
	NewTags            []NewTag `json:"newTags"`
}

type Asset struct {
	AssetDraft
	Avatars      []Tag  `json:"avatars"`
	FreeTags     []Tag  `json:"freeTags"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	CoverURL     string `json:"coverUrl"`
	ThumbnailURL string `json:"thumbnailUrl"`
}

type AssetQuery struct {
	Keyword       string   `json:"keyword"`
	CategoryIDs   []string `json:"categoryIds"`
	Uncategorized bool     `json:"uncategorized"`
	AvatarIDs     []string `json:"avatarIds"`
	AvatarMode    string   `json:"avatarMode"`
	AvatarMatch   string   `json:"avatarMatch"`
	FreeTagIDs    []string `json:"freeTagIds"`
	FreeMode      string   `json:"freeMode"`
	FreeMatch     string   `json:"freeMatch"`
	Folder        string   `json:"folder"`
	Sort          string   `json:"sort"`
	Descending    bool     `json:"descending"`
	Offset        int      `json:"offset"`
	Limit         int      `json:"limit"`
}

type AssetPage struct {
	Items []Asset `json:"items"`
	Total int     `json:"total"`
}

type TagDraft struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

type CategoryDraft struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

type BatchRequest struct {
	IDs        []string `json:"ids"`
	Operation  string   `json:"operation"`
	CategoryID string   `json:"categoryId"`
	TagIDs     []string `json:"tagIds"`
}

type MediaDraft struct {
	Token      string `json:"token"`
	PreviewURL string `json:"previewUrl"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type AvatarCandidate struct {
	AvatarID    string `json:"avatarId"`
	MatchedText string `json:"matchedText"`
	Context     string `json:"context"`
}

type BoothDraft struct {
	BoothID    string            `json:"boothId"`
	Name       string            `json:"name"`
	Cover      *MediaDraft       `json:"cover"`
	Candidates []AvatarCandidate `json:"candidates"`
}

type Preferences struct {
	Language string     `json:"language"`
	Theme    string     `json:"theme"`
	View     string     `json:"view"`
	CardSize string     `json:"cardSize"`
	Query    AssetQuery `json:"query"`
}

type Bootstrap struct {
	SystemLocale string      `json:"systemLocale"`
	Categories   []Category  `json:"categories"`
	AvatarTags   []Tag       `json:"avatarTags"`
	FreeTags     []Tag       `json:"freeTags"`
	Preferences  Preferences `json:"preferences"`
}

type WorkspaceInfo struct {
	DataPath      string `json:"dataPath"`
	SchemaVersion int    `json:"schemaVersion"`
	AssetCount    int    `json:"assetCount"`
}

type JobRef struct {
	ID string `json:"id"`
}

type JobSnapshot struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	Stage       string `json:"stage"`
	Completed   int    `json:"completed"`
	Total       int    `json:"total"`
	Cancellable bool   `json:"cancellable"`
	ResultJSON  string `json:"resultJson"`
	ErrorJSON   string `json:"errorJson"`
}

type BackupInfo struct {
	InspectionID  string `json:"inspectionId"`
	CreatedAt     string `json:"createdAt"`
	FormatVersion string `json:"formatVersion"`
	SchemaVersion int    `json:"schemaVersion"`
	AssetCount    int    `json:"assetCount"`
	TotalBytes    int64  `json:"totalBytes"`
}
