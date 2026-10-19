import { describe, expect, it } from 'vitest';
import { createDraft, validateDraft, applyBoothResult, addDraftTag } from './draft';

describe('manual editing', () => {
  it('allows multiple manually entered assets without a Booth ID', () => {
    for (const name of ['其他渠道素材', 'ID 未知的商品']) {
      const draft = createDraft();
      draft.name = name;

      expect(draft.boothId).toBe('');
      expect(validateDraft(draft)).toEqual({});
    }
  });

  it('requires name and rejects a nonnumeric nonempty Booth ID', () => {
    const draft = createDraft();

    expect(validateDraft(draft).name).toBeTruthy();

    draft.name = '名称';
    draft.boothId = '无';

    expect(validateDraft(draft).boothId).toBeTruthy();
  });

  it('does not replace existing fields or confirm avatar candidates implicitly', () => {
    const draft = createDraft();
    draft.name = '手动名称';
    draft.notes = '备注';
    draft.avatarIds = ['a'];
    const filled = applyBoothResult(
      draft,
      {
        boothId: '123',
        name: '获取名称',
        cover: null,
        candidates: [{ avatarId: 'b', matchedText: 'B', context: 'B' }],
      },
      { replaceID: false, replaceName: false, replaceCover: false, avatarIds: [] },
    );

    expect(filled.name).toBe('手动名称');
    expect(filled.notes).toBe('备注');
    expect(filled.avatarIds).toEqual(['a']);
    expect(filled.boothId).toBe('');
  });

  it('selects an existing draft tag through its name or alias without creating duplicate keys', () => {
    let draft = addDraftTag(createDraft(), [], 'avatar', 'Lumina', ['ルミナ']);
    draft = addDraftTag(draft, [], 'avatar', ' lumina ', []);
    draft = addDraftTag(draft, [], 'avatar', 'ルミナ', []);

    expect(draft.newTags).toHaveLength(1);
    expect(draft.avatarIds).toEqual([draft.newTags[0].key]);
  });

  it('rejects a new alias that belongs to a different pending tag before saving', () => {
    const draft = addDraftTag(createDraft(), [], 'avatar', 'Lumina', ['ルミナ']);

    expect(() => addDraftTag(draft, [], 'avatar', 'Shinano', ['ルミナ'])).toThrow('已被使用');
    expect(draft.newTags).toHaveLength(1);
  });
});
