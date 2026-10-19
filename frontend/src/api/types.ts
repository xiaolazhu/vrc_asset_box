export interface Tag {
  id: string;
  kind: 'avatar' | 'free';
  name: string;
  aliases: string[];
  count: number;
}

export interface Category {
  id: string;
  name: string;
  position: number;
  count: number;
}

export interface NewTag {
  key: string;
  kind: 'avatar' | 'free';
  name: string;
  aliases: string[];
}

export interface AssetDraft {
  id: string;
  revision: number;

  name: string;
  boothId: string;
  categoryId: string;
  coverId: string;
  coverToken: string;

  universal: boolean;
  notes: string;

  folderPath: string;
  allowOfflineFolder: boolean;

  avatarIds: string[];
  freeTagIds: string[];
  newTags: NewTag[];
}

export interface Asset extends AssetDraft {
  avatars: Tag[];
  freeTags: Tag[];
  createdAt: string;
  updatedAt: string;
  coverUrl: string;
  thumbnailUrl: string;
}

export interface AssetQuery {
  keyword: string;

  categoryIds: string[];
  uncategorized: boolean;

  avatarIds: string[];
  avatarMode: 'selected' | 'universal' | 'unknown';
  avatarMatch: 'any' | 'all';

  freeTagIds: string[];
  freeMode: 'include' | 'exclude';
  freeMatch: 'any' | 'all';

  folder: 'all' | 'linked' | 'unlinked';
  sort: 'createdAt' | 'updatedAt' | 'name' | 'boothId';
  descending: boolean;

  offset: number;
  limit: number;
}

export interface AssetPage {
  items: Asset[];
  total: number;
}

export interface MediaDraft {
  token: string;
  previewUrl: string;
  width: number;
  height: number;
}

export interface AvatarCandidate {
  avatarId: string;
  matchedText: string;
  context: string;
}

export interface BoothDraft {
  boothId: string;
  name: string;
  cover: MediaDraft | null;
  candidates: AvatarCandidate[];
}

export interface Preferences {
  language?: import('../i18n').Language;
  theme: 'system' | 'light' | 'dark';
  view: 'cards' | 'table';
  cardSize: 'small' | 'medium' | 'large';
  query: AssetQuery;
}

export interface Bootstrap {
  systemLocale?: string;
  categories: Category[];
  avatarTags: Tag[];
  freeTags: Tag[];
  preferences: Preferences;
}

export interface JobSnapshot {
  id: string;
  kind: string;
  status: 'running' | 'succeeded' | 'failed' | 'cancelled';
  stage: string;
  completed: number;
  total: number;
  cancellable: boolean;
  resultJson: string;
  errorJson: string;
}

export interface ImportRow {
  key: string;
  sheet: string;
  row: number;
  draft: AssetDraft;
  existingAssetId: string;
  existingRowKey?: string;
  existingRevision: number;
  issues: string[];
  imageOptions: MediaDraft[];
}

export interface SheetMapping {
  sheet: string;
  included: boolean;
  categoryId: string;
  newCategoryName: string;
  boothColumn: string;
  nameColumn: string;
  imageColumn: string;
  avatarColumn: string;
  notesColumn: string;
  freeTagsColumn: string;
  folderColumn: string;
}

export interface ImportPreview {
  previewId: string;
  sourceSHA256: string;
  sheets: SheetMapping[];
  rows: ImportRow[];
}

export interface ImportResult {
  created: number;
  merged: number;
  skipped: number;
  failed: number;
}

export interface BackupInfo {
  inspectionId: string;
  createdAt: string;
  formatVersion: string;
  schemaVersion: number;
  assetCount: number;
  totalBytes: number;
}

export interface WorkspaceInfo {
  dataPath: string;
  schemaVersion: number;
  assetCount: number;
}
