import { describe, expect, it } from 'vitest';
import { createDraft } from '../editor/draft';
import {
  resolveMerge,
  renameImportTag,
  importTarget,
  invalidateChangedMergeChoices,
} from './merge';

describe('import field conflict decisions', () => {
  it('requires an explicit choice for different nonempty scalar fields and preserves tag unions', () => {
    const current = {
      ...createDraft(),
      id: 'existing',
      revision: 3,
      name: 'My name',
      notes: 'My notes',
      avatarIds: ['a'],
    };
    const incoming = {
      ...createDraft(),
      name: 'Excel name',
      notes: 'Excel notes',
      folderPath: 'D:\\Assets',
      avatarIds: ['b'],
    };
    const unresolved = resolveMerge(current, incoming, {});

    expect(unresolved.conflicts).toEqual(['name', 'notes']);

    const resolved = resolveMerge(current, incoming, { name: 'existing', notes: 'incoming' });

    expect(resolved.conflicts).toEqual([]);
    expect(resolved.draft).toMatchObject({
      id: 'existing',
      revision: 3,
      name: 'My name',
      notes: 'Excel notes',
      folderPath: 'D:\\Assets',
      avatarIds: ['a', 'b'],
    });
  });

  it('keeps a shared imported tag key consistent when its display name is corrected', () => {
    const makeRow = (key: string) => ({
      key,
      sheet: 'Cloth',
      row: 2,
      draft: {
        ...createDraft(),
        newTags: [{ key: 'new:lumina', kind: 'avatar' as const, name: 'Lumnia', aliases: [] }],
      },
      existingAssetId: '',
      existingRevision: 0,
      issues: [],
      imageOptions: [],
    });
    const rows = [makeRow('a'), makeRow('b')];
    renameImportTag(rows, 'new:lumina', 'Lumina');

    expect(rows.map((r) => r.draft.newTags[0].name)).toEqual(['Lumina', 'Lumina']);
  });

  it('uses an existing database asset before a same-file duplicate target', () => {
    const row = {
      key: 'b',
      sheet: 'Cloth',
      row: 3,
      draft: createDraft(),
      existingAssetId: 'database-id',
      existingRowKey: 'a',
      existingRevision: 2,
      issues: [],
      imageOptions: [],
    };

    expect(importTarget(row)).toEqual({ kind: 'asset', id: 'database-id' });
  });

  it('requires reconfirmation when an earlier merge changes the value displayed as existing', () => {
    const database = { ...createDraft(), name: 'Same', notes: 'A' };
    const first = { ...createDraft(), name: 'Same', notes: 'B' };
    const cumulative = resolveMerge(database, first, { notes: 'incoming' }).draft;
    const checked = invalidateChangedMergeChoices(database, cumulative, {
      notes: 'existing',
      name: 'existing',
    });

    expect(checked.invalidated).toEqual(['notes']);
    expect(checked.choices).toEqual({ name: 'existing' });

    const second = { ...createDraft(), name: 'Same', notes: 'C' };

    expect(resolveMerge(cumulative, second, checked.choices).conflicts).toEqual(['notes']);
    expect(resolveMerge(cumulative, second, { notes: 'existing' }).draft.notes).toBe('B');
  });
});
