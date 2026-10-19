import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { NSelect, NCheckbox, NCard, NModal } from 'naive-ui';
import ImportWizard from './ImportWizard.vue';
import { api } from '../../api/client';
import { createDraft, createQuery } from '../editor/draft';
import type { ImportPreview, ImportRow } from '../../api/types';

const mocks = vi.hoisted(() => ({ warning: vi.fn(), success: vi.fn(), error: vi.fn() }));

vi.mock('naive-ui', async (original) => ({
  ...(await original<typeof import('naive-ui')>()),
  useDialog: () => ({ warning: mocks.warning }),
  useMessage: () => mocks,
}));

const data = {
  categories: [],
  avatarTags: [],
  freeTags: [],
  preferences: {
    theme: 'light' as const,
    view: 'cards' as const,
    cardSize: 'medium' as const,
    query: createQuery(),
  },
};
const stubs = {
  Modal: { props: ['show'], template: '<div v-if="show"><slot/></div>' },
  Card: { template: '<section><slot/><slot name="footer"/></section>' },
};
let wrapper: ReturnType<typeof mount> | undefined;

afterEach(() => {
  wrapper?.unmount();
  vi.restoreAllMocks();
  mocks.warning.mockClear();
});

describe('cumulative import confirmation', () => {
  it.each(['button', 'mask', 'escape'] as const)('编辑弹窗通过 %s 关闭后释放滚动锁并可再次打开', async (method) => {
    const preview: ImportPreview = {
      previewId: 'preview', sourceSHA256: 'hash',
      sheets: [{ sheet: 'Items', included: true, categoryId: '', newCategoryName: '',
        nameColumn: 'A', boothColumn: '', imageColumn: '', avatarColumn: '', notesColumn: '',
        freeTagsColumn: '', folderColumn: '' }],
      rows: [{ key: 'row', sheet: 'Items', row: 2, draft: { ...createDraft(), name: 'Asset' },
        existingAssetId: '', existingRevision: 0, issues: [], imageOptions: [] }],
    };
    vi.spyOn(api, 'selectWorkbook').mockResolvedValue('D:/source.xlsx');
    vi.spyOn(api, 'previewImport').mockResolvedValue({ id: 'preview-job' });
    vi.spyOn(api, 'discardImport').mockResolvedValue(undefined);
    vi.spyOn(api, 'job').mockResolvedValue({ id: 'preview-job', kind: 'preview', status: 'succeeded',
      stage: 'Complete', completed: 1, total: 1, cancellable: false,
      resultJson: JSON.stringify(preview), errorJson: '' });
    wrapper = mount(ImportWizard, { props: { data }, attachTo: document.body,
      global: { stubs: { transition: false } } });
    const button = (label: string) => wrapper!.findAll('button').find(b => b.text() === label)!;
    await button('选择 Excel 工作簿').trigger('click');
    await flushPromises();
    await button('生成预览').trigger('click');
    await flushPromises();
    await button('编辑').trigger('click');
    await flushPromises();
    expect(document.querySelector('.import-edit-modal')).not.toBeNull();

    if (method === 'button') {
      wrapper.findAllComponents(NCard).find(card => card.classes().includes('import-edit-modal'))!.vm.$emit('close');
    } else if (method === 'mask') {
      const mask = document.querySelector('.n-modal-mask')!;
      mask.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      mask.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
      mask.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    } else {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', code: 'Escape', bubbles: true }));
    }
    await flushPromises();
    // 关闭动画结束前，内容必须保留，否则 Modal 无法完成滚动锁清理。
    expect(wrapper.findComponent(NModal).props('show')).toBe(false);
    await vi.waitFor(() => {
      expect(document.querySelector('.n-modal-body-wrapper')).toBeNull();
      expect(document.documentElement.style.overflow).not.toBe('hidden');
    });
    await button('编辑').trigger('click');
    await flushPromises();
    expect(document.querySelector('.import-edit-modal')).not.toBeNull();
  });

  it('shows changed existing values and blocks commit until a stale choice is confirmed again', async () => {
    const makeRow = (key: string, notes: string): ImportRow => ({
      key,
      sheet: 'Cloth',
      row: key === 'first' ? 2 : 3,
      draft: { ...createDraft(), name: 'Same', boothId: '123', notes },
      existingAssetId: 'db',
      existingRevision: 1,
      issues: [],
      imageOptions: [],
    });
    const preview: ImportPreview = {
      previewId: 'preview',
      sourceSHA256: 'hash',
      sheets: [
        {
          sheet: 'Cloth',
          included: true,
          categoryId: '',
          newCategoryName: '',
          nameColumn: 'B',
          boothColumn: 'A',
          imageColumn: '',
          avatarColumn: '',
          notesColumn: 'C',
          freeTagsColumn: '',
          folderColumn: '',
        },
      ],
      rows: [makeRow('first', 'B'), makeRow('second', 'C')],
    };
    vi.spyOn(api, 'selectWorkbook').mockResolvedValue('D:\\source.xlsx');
    vi.spyOn(api, 'previewImport').mockResolvedValue({ id: 'preview-job' });
    vi.spyOn(api, 'job').mockResolvedValue({
      id: 'preview-job',
      kind: 'preview',
      status: 'succeeded',
      stage: 'done',
      completed: 1,
      total: 1,
      cancellable: false,
      resultJson: JSON.stringify(preview),
      errorJson: '',
    });
    vi.spyOn(api, 'get').mockResolvedValue({
      ...createDraft(),
      id: 'db',
      revision: 1,
      name: 'Same',
      boothId: '123',
      notes: 'A',
      avatars: [],
      freeTags: [],
      coverUrl: '',
      thumbnailUrl: '',
      createdAt: '',
      updatedAt: '',
    });
    vi.spyOn(api, 'discardImport').mockResolvedValue();
    const commit = vi.spyOn(api, 'commitImport');
    wrapper = mount(ImportWizard, { props: { data }, global: { stubs } });
    const button = (label: string) => wrapper!.findAll('button').find((b) => b.text() === label)!;
    await button('选择 Excel 工作簿').trigger('click');
    await flushPromises();
    await button('生成预览').trigger('click');
    await flushPromises();
    const tableRows = wrapper.findAll('tbody tr');
    tableRows[0].findComponent(NSelect).vm.$emit('update:value', 'merge');
    await flushPromises();
    wrapper
      .findAllComponents(NCheckbox)
      .find((c) => c.text() === '使用导入值')!
      .vm.$emit('update:checked', true);
    await button('完成检查').trigger('click');
    tableRows[1].findComponent(NSelect).vm.$emit('update:value', 'merge');
    await flushPromises();
    wrapper
      .findAllComponents(NCheckbox)
      .find((c) => c.text() === '保留已有')!
      .vm.$emit('update:checked', true);
    await button('完成检查').trigger('click');
    await button('确认导入…').trigger('click');
    await flushPromises();

    expect(wrapper.text()).toContain('前序合并已更改目标字段（备注）');
    expect(commit).not.toHaveBeenCalled();
    expect(mocks.warning).not.toHaveBeenCalled();

    const existingChoice = wrapper
      .findAllComponents(NCheckbox)
      .find((c) => c.text() === '保留已有')!;

    expect(existingChoice.props('checked')).toBe(false);
    expect(existingChoice.element.parentElement?.textContent).toContain('B');

    existingChoice.vm.$emit('update:checked', true);
    await button('完成检查').trigger('click');
    await button('确认导入…').trigger('click');
    await flushPromises();

    expect(mocks.warning).toHaveBeenCalledWith(
      expect.objectContaining({ title: '导入所选素材？' }),
    );
    expect(commit).not.toHaveBeenCalled();

    mocks.warning.mockClear();
    tableRows[0].findComponent(NSelect).vm.$emit('update:value', 'skip');
    await flushPromises();
    await button('确认导入…').trigger('click');
    await flushPromises();

    expect(mocks.warning).not.toHaveBeenCalled();

    const revertedChoice = wrapper
      .findAllComponents(NCheckbox)
      .find((c) => c.text() === '保留已有')!;

    expect(revertedChoice.props('checked')).toBe(false);
    expect(revertedChoice.element.parentElement?.textContent).toContain('A');
  });
});
