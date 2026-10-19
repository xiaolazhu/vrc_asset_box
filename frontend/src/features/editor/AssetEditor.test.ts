import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import AssetEditor from './AssetEditor.vue';
import { api } from '../../api/client';
import { createDraft, createQuery } from './draft';
import type { JobSnapshot } from '../../api/types';

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
  Modal: { props: ['show', 'maskClosable'], template: '<div v-if="show"><slot/></div>' },
  Card: { template: '<section><slot/><slot name="footer"/></section>' },
};
let wrapper: ReturnType<typeof mount> | undefined;

beforeEach(() => {
  mocks.warning.mockClear();
  vi.spyOn(api, 'startBooth');
  vi.spyOn(api, 'discardCover').mockResolvedValue();
});

afterEach(() => {
  wrapper?.unmount();
  vi.restoreAllMocks();
});

describe('manual editor workflow', () => {
  it('遮罩关闭保留脏草稿确认，Booth 子窗口关闭不影响编辑器', async () => {
    wrapper = mount(AssetEditor, { props: { asset: null, data }, global: { stubs } });
    await flushPromises();
    const editor = wrapper.findAllComponents(stubs.Modal)[0];
    expect(editor.props('maskClosable')).toBe(true);
    editor.vm.$emit('update:show', false);
    expect(wrapper.emitted('close')).toHaveLength(1);

    await wrapper.get('input[placeholder="输入素材名称"]').setValue('未保存');
    editor.vm.$emit('update:show', false);
    expect(mocks.warning).toHaveBeenCalledOnce();
    expect(wrapper.emitted('close')).toHaveLength(1);
    await wrapper.findAll('button').find(button => button.text().includes('Booth 自动识别'))!.trigger('click');
    const booth = wrapper.findAllComponents(stubs.Modal)[1];
    expect(booth.props('maskClosable')).toBe(true);
    booth.vm.$emit('update:show', false);
    await flushPromises();
    expect(booth.props('show')).toBe(false);
    expect(editor.props('show')).toBe(true);
    expect(wrapper.get<HTMLInputElement>('input[placeholder="输入素材名称"]').element.value).toBe('未保存');
    expect(wrapper.emitted('close')).toHaveLength(1);
  });

  it('保存进行中禁止遮罩关闭编辑器', async () => {
    let finish!: (asset: Awaited<ReturnType<typeof api.save>>) => void;
    vi.spyOn(api, 'save').mockImplementation(() => new Promise(resolve => { finish = resolve; }));
    wrapper = mount(AssetEditor, { props: { asset: null, data }, global: { stubs } });
    await flushPromises();
    await wrapper.get('input[placeholder="输入素材名称"]').setValue('保存测试');
    await wrapper.findAll('button').find(button => button.text() === '保存素材')!.trigger('click');
    const editor = wrapper.findAllComponents(stubs.Modal)[0];
    expect(editor.props('maskClosable')).toBe(false);
    editor.vm.$emit('update:show', false);
    expect(wrapper.emitted('close')).toBeUndefined();
    finish({ ...createDraft(), name: '保存测试' } as Awaited<ReturnType<typeof api.save>>);
    await flushPromises();
  });

  it('挂载后触发显示，关闭时等待弹窗转发退场完成事件', async () => {
    wrapper = mount(AssetEditor, { props: { asset: null, data }, global: { stubs } });
    const modal = wrapper.findComponent(stubs.Modal);
    expect(modal.props('show')).toBe(false);
    await flushPromises();
    expect(modal.props('show')).toBe(true);
    await wrapper.setProps({ show: false });
    expect(modal.props('show')).toBe(false);
    expect(wrapper.emitted('afterLeave')).toBeUndefined();
    modal.vm.$emit('afterLeave');
    expect(wrapper.emitted('afterLeave')).toHaveLength(1);
  });

  it('保存时标红无效字段并聚焦首项，修正后即时清除错误', async () => {
    const save = vi.spyOn(api, 'save');
    const scroll = vi.fn();
    wrapper = mount(AssetEditor, { props: { asset: null, data }, attachTo: document.body, global: { stubs } });
    await flushPromises();
    const name = wrapper.get('input[placeholder="输入素材名称"]');
    Object.defineProperty(name.element, 'scrollIntoView', { configurable: true, value: scroll });
    const booth = wrapper.get('input[placeholder="输入数字 ID，可留空"]');
    await booth.setValue('abc');
    await wrapper.findAll('button').find(button => button.text() === '保存素材')!.trigger('click');
    await flushPromises();

    expect(save).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain('请输入素材名称');
    expect(name.attributes('aria-invalid')).toBe('true');
    expect(booth.attributes('aria-invalid')).toBe('true');
    expect(name.element.closest('.n-input')?.classList.contains('n-input--error-status')).toBe(true);
    expect(document.activeElement).toBe(name.element);
    expect(scroll).toHaveBeenCalled();

    await name.setValue('有效名称');
    expect(wrapper.text()).not.toContain('请输入素材名称');
    expect(name.attributes('aria-invalid')).toBe('false');
    expect(booth.attributes('aria-invalid')).toBe('true');
    await booth.setValue('');
    expect(booth.attributes('aria-invalid')).toBe('false');
    expect(wrapper.text()).not.toContain('Booth ID 应为输入数字 ID，可留空');
  });

  it('识别成功锁定弹窗输入，重新打开可编辑且失败后可重试', async () => {
    vi.spyOn(api, 'startBooth').mockResolvedValue({ id: 'lookup' });
    const job = vi.spyOn(api, 'job').mockResolvedValue({
      id: 'lookup', kind: 'booth', status: 'succeeded', stage: 'done',
      completed: 2, total: 2, cancellable: false, errorJson: '',
      resultJson: JSON.stringify({ boothId: '123', name: '识别名称', cover: null, candidates: [] }),
    });
    wrapper = mount(AssetEditor, { props: { asset: null, data }, global: { stubs } });
    await flushPromises();
    const open = () => wrapper!.findAll('button').find(button => button.text().includes('Booth 自动识别'))!.trigger('click');
    const fetch = () => wrapper!.findAll('button').find(button => button.text() === '获取资料')!.trigger('click');
    await open();
    await wrapper.get('input[placeholder="Booth ID 或商品链接"]').setValue('123');
    await fetch();
    await flushPromises();
    expect(wrapper.get('input[placeholder="Booth ID 或商品链接"]').attributes('disabled')).toBeDefined();

    await wrapper.findAll('button').find(button => button.text() === '返回编辑')!.trigger('click');
    await open();
    expect(wrapper.get('input[placeholder="Booth ID 或商品链接"]').attributes('disabled')).toBeUndefined();
    job.mockRejectedValueOnce(new Error('网络错误'));
    await wrapper.get('input[placeholder="Booth ID 或商品链接"]').setValue('456');
    await fetch();
    await flushPromises();
    expect(wrapper.get('input[placeholder="Booth ID 或商品链接"]').attributes('disabled')).toBeUndefined();
  });

  it('saves a name without requesting Booth information or requiring an ID', async () => {
    const saved = {
      ...createDraft(),
      name: '手动素材',
      avatars: [],
      freeTags: [],
      coverUrl: '',
      thumbnailUrl: '',
      createdAt: '',
      updatedAt: '',
    };
    const save = vi.spyOn(api, 'save').mockResolvedValue(saved);
    wrapper = mount(AssetEditor, { props: { asset: null, data }, global: { stubs } });
    await flushPromises();
    await wrapper.get('input[placeholder="输入素材名称"]').setValue('手动素材');
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '保存素材')!
      .trigger('click');
    await flushPromises();

    expect(save).toHaveBeenCalledWith(expect.objectContaining({ name: '手动素材', boothId: '' }));
    expect(api.startBooth).not.toHaveBeenCalled();
    expect(wrapper.emitted('saved')).toHaveLength(1);
  });

  it('asks before discarding a dirty draft with Escape', async () => {
    wrapper = mount(AssetEditor, { props: { asset: null, data }, global: { stubs } });
    await flushPromises();
    await wrapper.get('input[placeholder="输入素材名称"]').setValue('未保存内容');
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));

    expect(mocks.warning).toHaveBeenCalledWith(
      expect.objectContaining({ title: '放弃未保存的修改？' }),
    );
    expect(wrapper.emitted('close')).toBeUndefined();
  });

  it('ignores a successful Booth response that arrives after cancellation and keeps the manual draft', async () => {
    vi.spyOn(api, 'startBooth').mockResolvedValue({ id: 'lookup' });
    vi.spyOn(api, 'cancel').mockResolvedValue();
    let finish!: (job: JobSnapshot) => void;
    vi.spyOn(api, 'job').mockImplementation(() => new Promise((resolve) => (finish = resolve)));
    wrapper = mount(AssetEditor, { props: { asset: null, data }, global: { stubs } });
    await flushPromises();
    await wrapper.get('input[placeholder="输入素材名称"]').setValue('手动草稿');
    await wrapper
      .findAll('button')
      .find((b) => b.text().includes('Booth 自动识别'))!
      .trigger('click');
    await wrapper.get('input[placeholder="Booth ID 或商品链接"]').setValue('123');
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '获取资料')!
      .trigger('click');
    await flushPromises();
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '取消获取')!
      .trigger('click');
    finish({
      id: 'lookup',
      kind: 'booth',
      status: 'succeeded',
      stage: 'done',
      completed: 1,
      total: 1,
      cancellable: false,
      resultJson: JSON.stringify({
        boothId: '123',
        name: '迟到的网络名称',
        cover: null,
        candidates: [],
      }),
      errorJson: '',
    });
    await flushPromises();

    expect(
      (wrapper.get('input[placeholder="输入素材名称"]').element as HTMLInputElement).value,
    ).toBe('手动草稿');
    expect(
      (wrapper.get('input[placeholder="输入数字 ID，可留空"]').element as HTMLInputElement).value,
    ).toBe('');
    expect(wrapper.emitted('saved')).toBeUndefined();
  });
});
