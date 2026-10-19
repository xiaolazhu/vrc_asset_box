import { localize, t } from '../i18n';
import * as bindings from '../../wailsjs/go/main/App';
import type {
  Asset,
  AssetDraft,
  AssetPage,
  AssetQuery,
  Bootstrap,
  Category,
  Tag,
  MediaDraft,
  Preferences,
  JobSnapshot,
  BackupInfo,
  WorkspaceInfo,
  ImportRow,
  SheetMapping,
} from './types';

type Bridge = Record<string, (...args: unknown[]) => Promise<unknown>>;

// 持久显示的错误保留原文，渲染时再翻译，避免切换语言后残留旧文案。
export function sourceErrorMessage(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err);

  try {
    const parsed = JSON.parse(raw);
    return parsed.message || raw;
  } catch {
    return raw;
  }
}

export function errorMessage(err: unknown): string {
  return localize(sourceErrorMessage(err));
}

export async function call<T>(method: string, ...args: unknown[]): Promise<T> {
  const bridge = (window as unknown as { go?: { main?: { App?: Bridge } } }).go?.main?.App;
  if (!bridge?.[method]) {
    throw new Error(t('Cannot connect to the desktop service. Restart the app.'));
  }

  return (await (bindings as unknown as Bridge)[method](...args)) as T;
}

export const api = {
  refreshSystemLocale: () => call<string>('RefreshSystemLocale'),

  setLanguage: (language: string) => call<void>('SetLanguage', language),

  // 素材浏览与编辑。
  bootstrap: () => call<Bootstrap>('GetBootstrap'),

  list: (query: AssetQuery) => call<AssetPage>('ListAssets', query),

  get: (id: string) => call<Asset>('GetAsset', id),

  save: (draft: AssetDraft) => call<Asset>('SaveAsset', draft),

  batch: (ids: string[], operation: string, categoryId = '', tagIds: string[] = []) =>
    call<void>('BatchAssets', { ids, operation, categoryId, tagIds }),

  // 分类、Avatar 标签与自由标签。
  saveCategory: (draft: Partial<Category>) =>
    call<Category>('SaveCategory', {
      id: '',
      name: '',
      position: 0,
      ...draft,
    }),

  deleteCategory: (id: string, target = '') => call<void>('DeleteCategory', id, target),

  reorderCategories: (ids: string[]) => call<void>('ReorderCategories', ids),

  saveTag: (draft: { id: string; kind: string; name: string; aliases: string[] }) =>
    call<Tag>('SaveTag', draft),

  deleteTag: (id: string) => call<void>('DeleteTag', id),

  mergeTags: (sourceId: string, targetId: string) => call<void>('MergeTags', sourceId, targetId),

  // 桌面对话框、本地文件与封面暂存。
  selectFolder: () => call<string>('SelectFolder'),

  openFolder: (path: string) => call<void>('OpenFolder', path),

  openBooth: (id: string) => call<void>('OpenBooth', id),

  copy: (text: string) => call<void>('CopyText', text),

  selectCover: () => call<MediaDraft>('SelectCover'),

  stagePath: (path: string) => call<MediaDraft>('StageCoverPath', path),

  stageBytes: (data: string) => call<MediaDraft>('StageCoverBytes', data),

  discardCover: (token: string) => call<void>('DiscardCover', token),

  // 后台任务的启动、查询与取消。
  startBooth: (input: string, id: string) => call<{ id: string }>('StartBooth', input, id),

  job: (id: string) => call<JobSnapshot>('GetJob', id),

  cancel: (id: string) => call<void>('CancelJob', id),

  // Excel 预览与提交；生成提交请求时保留用户逐行确认的决定。
  selectWorkbook: () => call<string>('SelectWorkbook'),

  exportExcel: () => call<{ id: string }>('StartExcelExport'),

  previewImport: (path: string, mappings: SheetMapping[]) =>
    call<{ id: string }>('StartImportPreview', { path, mappings }),

  commitImport: (previewId: string, rows: ImportRow[], actions: Record<string, string>) =>
    call<{ id: string }>('StartImportCommit', {
      previewId,
      decisions: rows.map((row) => ({
        rowKey: row.key,
        action: actions[row.key] || (row.existingAssetId || row.existingRowKey ? 'skip' : 'create'),
        targetAssetId:
          row.existingAssetId || (row.existingRowKey ? 'row:' + row.existingRowKey : ''),
        targetRevision: row.existingRevision || 0,
        resolvedDraft: row.draft,
      })),
    }),

  discardImport: (id: string) => call<void>('DiscardImportPreview', id),

  report: (id: string) => call<string>('SaveImportReport', id),

  // 偏好设置、资料库与备份恢复。
  preferences: (preferences: Preferences) => call<void>('SavePreferences', preferences),

  info: () => call<WorkspaceInfo>('GetWorkspaceInfo'),

  openData: () => call<void>('OpenDataFolder'),

  backup: () => call<{ id: string }>('StartBackup'),

  inspectBackup: () => call<BackupInfo>('InspectBackup'),

  restore: (id: string) => call<{ id: string }>('StartRestore', id),
};
