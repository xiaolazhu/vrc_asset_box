import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import TaxonomyPage from './TaxonomyPage.vue';
import { api } from '../../api/client';
import { createQuery } from '../editor/draft';

const messages = vi.hoisted(() => ({ error: vi.fn(), success: vi.fn() }));

vi.mock('naive-ui', async (original) => ({
  ...(await original<typeof import('naive-ui')>()),
  useMessage: () => messages,
  useDialog: () => ({ warning: vi.fn() }),
}));

let wrapper: ReturnType<typeof mount> | undefined;

afterEach(() => {
  wrapper?.unmount();
  vi.restoreAllMocks();
});

const data = {
  categories: ['衣服', '头发', '工具'].map((name, position) => ({
    id: String(position), name, position, count: 0,
  })),
  avatarTags: [],
  freeTags: [],
  preferences: { theme: 'system' as const, view: 'cards' as const, cardSize: 'medium' as const, query: createQuery() },
};

describe('分类拖拽排序', () => {
  it('从名称前手柄拖动后提交完整顺序，不再显示上下移按钮', async () => {
    const save = vi.spyOn(api, 'reorderCategories').mockResolvedValue();
    wrapper = mount(TaxonomyPage, { props: { kind: 'category', data } });
    await flushPromises();
    expect(wrapper.text()).not.toContain('上移');
    expect(wrapper.text()).not.toContain('下移');

    await wrapper.findAll('.category-drag-handle')[2].trigger('dragstart');
    await wrapper.find('[data-category-id="0"]').trigger('drop', { clientY: 0 });
    await flushPromises();

    expect(save).toHaveBeenCalledWith(['2', '0', '1']);
    expect(wrapper.emitted('changed')).toHaveLength(1);
  });

  it('保存失败时保留原顺序并提示错误', async () => {
    vi.spyOn(api, 'reorderCategories').mockRejectedValue(new Error('无法保存'));
    wrapper = mount(TaxonomyPage, { props: { kind: 'category', data } });
    await flushPromises();
    await wrapper.findAll('.category-drag-handle')[2].trigger('dragstart');
    await wrapper.find('[data-category-id="0"]').trigger('drop', { clientY: 0 });
    await flushPromises();

    expect(wrapper.emitted('changed')).toBeUndefined();
    expect(messages.error).toHaveBeenCalledWith('无法保存');
    expect(wrapper.findAll('[data-category-id]').map(row => row.attributes('data-category-id')))
      .toEqual(['0', '1', '2']);
  });
});
