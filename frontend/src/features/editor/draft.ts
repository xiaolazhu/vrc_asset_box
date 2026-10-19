import { t } from '../../i18n';
import type { AssetDraft, AssetQuery, BoothDraft, Tag } from '../../api/types';

export function createDraft(): AssetDraft {
  return {
    id: '',
    revision: 0,
    name: '',
    boothId: '',
    categoryId: '',
    coverId: '',
    coverToken: '',
    universal: false,
    notes: '',
    folderPath: '',
    allowOfflineFolder: false,
    avatarIds: [],
    freeTagIds: [],
    newTags: [],
  };
}

export function createQuery(): AssetQuery {
  return {
    keyword: '',
    categoryIds: [],
    uncategorized: false,
    avatarIds: [],
    avatarMode: 'selected',
    avatarMatch: 'any',
    freeTagIds: [],
    freeMode: 'include',
    freeMatch: 'any',
    folder: 'all',
    sort: 'createdAt',
    descending: true,
    offset: 0,
    limit: 60,
  };
}

export function validateDraft(draft: AssetDraft): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!draft.name.trim()) {
    errors.name = t('Enter an asset name');
  }

  if (draft.boothId.trim() && !/^\d+$/.test(draft.boothId.trim())) {
    errors.boothId = t('Booth ID must contain only digits or be left blank');
  }

  return errors;
}

export function applyBoothResult(
  draft: AssetDraft,
  result: BoothDraft,
  selection: {
    replaceID: boolean;
    replaceName: boolean;
    replaceCover: boolean;
    avatarIds: string[];
  },
): AssetDraft {
  return {
    ...draft,
    boothId: selection.replaceID ? result.boothId : draft.boothId,
    name: selection.replaceName ? result.name : draft.name,
    coverToken: selection.replaceCover && result.cover ? result.cover.token : draft.coverToken,
    coverId: selection.replaceCover && result.cover ? '' : draft.coverId,
    avatarIds: [...new Set([...draft.avatarIds, ...selection.avatarIds])],
  };
}

export function addDraftTag(
  draft: AssetDraft,
  catalog: Tag[],
  kind: 'avatar' | 'free',
  name: string,
  aliases: string[],
): AssetDraft {
  const normalized = (value: string) => value.trim().toLowerCase();
  const display = name.trim();
  if (!display) {
    throw new Error(t('Tag name cannot be empty'));
  }

  const tags = [
    ...catalog.filter((tag) => tag.kind === kind).map((tag) => ({ ...tag, key: tag.id })),
    ...draft.newTags.filter((tag) => tag.kind === kind),
  ];
  const known = tags.find((tag) =>
    [tag.name, ...tag.aliases].some((n) => normalized(n) === normalized(display)),
  );
  const cleanAliases =
    kind === 'avatar'
      ? [...new Map(aliases.map((a) => [normalized(a), a.trim()])).entries()]
          .filter(([k]) => k && k !== normalized(display))
          .map(([, v]) => v)
      : [];
  if (
    !known &&
    cleanAliases.some((alias) =>
      tags.some((tag) => [tag.name, ...tag.aliases].some((n) => normalized(n) === normalized(alias))),
    )
  ) {
    throw new Error(t('Name or alias already in use. Select an existing tag or change the name or alias.'));
  }

  const key = known?.key || `draft-${crypto.randomUUID()}`;
  const field = kind === 'avatar' ? 'avatarIds' : 'freeTagIds';

  return {
    ...draft,
    newTags: known
      ? draft.newTags
      : [...draft.newTags, { key, kind, name: display, aliases: cleanAliases }],
    [field]: [...new Set([...draft[field], key])],
  };
}
