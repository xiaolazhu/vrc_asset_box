import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import AssetLibrary from './AssetLibrary.vue';
import { api } from '../../api/client';
import { createQuery } from '../editor/draft';
import { t } from '../../i18n';

const mocks = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn() }));
vi.mock('naive-ui', async (original) => ({
  ...(await original<typeof import('naive-ui')>()),
  useMessage: () => mocks,
  useDialog: () => ({ warning: mocks.warning }),
}));

let wrapper: ReturnType<typeof mount> | undefined;

afterEach(() => {
  wrapper?.unmount();
  vi.restoreAllMocks();
});

describe('素材筛选', () => {
  it('不显示无自由标签筛选', async () => {
    const list = vi.spyOn(api, 'list').mockResolvedValue({ items: [], total: 0 });
    wrapper = mount(AssetLibrary, {
      props: {
        data: {
          categories: [],
          avatarTags: [],
          freeTags: [],
          preferences: {
            theme: 'system',
            view: 'cards',
            cardSize: 'medium',
            query: createQuery(),
          },
        },
        preferences: {
          theme: 'system',
          view: 'cards',
          cardSize: 'medium',
          query: createQuery(),
        },
        version: 0,
      },
      global: { stubs: { transition: false } },
    });
    await flushPromises();

    expect(wrapper.text()).not.toContain(t('No custom tags'));
    expect(list).toHaveBeenCalledOnce();
  });
});
