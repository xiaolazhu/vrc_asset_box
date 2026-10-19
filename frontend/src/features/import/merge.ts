import type { AssetDraft, ImportRow } from '../../api/types';

export function renameImportTag(rows: ImportRow[], key: string, name: string): void {
  for (const row of rows) {
    for (const tag of row.draft.newTags) {
      if (tag.key === key) {
        tag.name = name;
      }
    }
  }
}

export function importTarget(row: ImportRow): { kind: 'asset' | 'row'; id: string } | null {
  return row.existingAssetId
    ? { kind: 'asset', id: row.existingAssetId }
    : row.existingRowKey
      ? { kind: 'row', id: row.existingRowKey }
      : null;
}

export const mergeFields = ['name', 'categoryId', 'notes', 'folderPath', 'coverId'] as const;

export type MergeField = (typeof mergeFields)[number];

export type MergeChoices = Partial<Record<MergeField, 'existing' | 'incoming'>>;

export function invalidateChangedMergeChoices(
  displayed: AssetDraft | undefined,
  current: AssetDraft,
  previousChoices: MergeChoices,
): { choices: MergeChoices; invalidated: MergeField[] } {
  const choices = { ...previousChoices };
  const invalidated: MergeField[] = [];
  for (const field of mergeFields) {
    const before =
      displayed &&
      (field === 'coverId' ? displayed.coverToken || displayed.coverId : displayed[field]);
    const now = field === 'coverId' ? current.coverToken || current.coverId : current[field];
    if (choices[field] && before !== now) {
      delete choices[field];
      invalidated.push(field);
    }
  }

  return { choices, invalidated };
}

export function resolveMerge(
  existing: AssetDraft,
  incoming: AssetDraft,
  choices: Partial<Record<MergeField, 'existing' | 'incoming'>>,
): { draft: AssetDraft; conflicts: MergeField[] } {
  const draft: AssetDraft = {
    ...existing,
    avatarIds: [...new Set([...existing.avatarIds, ...incoming.avatarIds])],
    freeTagIds: [...new Set([...existing.freeTagIds, ...incoming.freeTagIds])],
    newTags: [
      ...new Map([...existing.newTags, ...incoming.newTags].map((tag) => [tag.key, tag])).values(),
    ],
    universal: existing.universal || incoming.universal,
  };
  const conflicts: MergeField[] = [];
  for (const field of mergeFields) {
    const existingValue =
      field === 'coverId' ? existing.coverToken || existing.coverId : existing[field];
    const incomingValue =
      field === 'coverId' ? incoming.coverToken || incoming.coverId : incoming[field];
    if (existingValue && incomingValue && existingValue !== incomingValue && !choices[field]) {
      conflicts.push(field);
    }

    if ((!existingValue && incomingValue) || choices[field] === 'incoming') {
      draft[field] = incoming[field];
      if (field === 'coverId') {
        draft.coverToken = incoming.coverToken;
      }
    }
  }

  return { draft, conflicts };
}
