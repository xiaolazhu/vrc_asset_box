import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import SettingsPage from './SettingsPage.vue';
import { api } from '../../api/client';
import { createQuery } from '../editor/draft';
import { NButton, NSelect } from 'naive-ui';
import { setLanguage, t } from '../../i18n';

const mocks = vi.hoisted(() => ({ warning: vi.fn(), success: vi.fn(), error: vi.fn() }));

vi.mock('naive-ui', async (original) => ({
  ...(await original<typeof import('naive-ui')>()),
  useDialog: () => ({ warning: mocks.warning }),
  useMessage: () => mocks,
}));

const preferences = {
  theme: 'light' as const,
  view: 'cards' as const,
  cardSize: 'medium' as const,
  query: createQuery(),
};
let wrapper: ReturnType<typeof mount> | undefined;

beforeEach(() => {
  mocks.warning.mockClear();
  vi.spyOn(api, 'info').mockResolvedValue({
    dataPath: 'D:\\Data',
    schemaVersion: 1,
    assetCount: 7,
  });
});

afterEach(() => {
  wrapper?.unmount();
  vi.restoreAllMocks();
});

describe('backup restore confirmation', () => {
  it('取消导出备份不查询任务或提示成功', async () => {
    mocks.success.mockClear();
    vi.spyOn(api, 'backup').mockResolvedValue({ id: '' });
    const job = vi.spyOn(api, 'job');
    wrapper = mount(SettingsPage, { props: { preferences } });
    await flushPromises();
    await wrapper.findAll('button').find(button => button.text() === t('Export backup'))!.trigger('click');
    await flushPromises();

    expect(job).not.toHaveBeenCalled();
    expect(mocks.success).not.toHaveBeenCalled();
    expect(wrapper.find('.job-progress').exists()).toBe(false);
  });

  it('选择备份仅自身显示加载，取消后不显示预览或触发恢复', async () => {
    let finish!: (value: Awaited<ReturnType<typeof api.inspectBackup>>) => void;
    vi.spyOn(api, 'inspectBackup').mockImplementation(() => new Promise(resolve => { finish = resolve; }));
    const restore = vi.spyOn(api, 'restore');
    const job = vi.spyOn(api, 'job');
    wrapper = mount(SettingsPage, { props: { preferences } });
    await flushPromises();
    const buttons = wrapper.findAllComponents(NButton);
    const select = buttons.find(button => button.text() === t('Select backup'))!;
    const backup = buttons.find(button => button.text() === t('Export backup'))!;
    await select.trigger('click');

    expect(select.props('loading')).toBe(true);
    expect(backup.props('loading')).toBe(false);
    expect(backup.props('disabled')).toBe(true);

    finish({ inspectionId: '', assetCount: 0, createdAt: '', formatVersion: '', schemaVersion: 0, totalBytes: 0 });
    await flushPromises();
    expect(select.props('loading')).toBe(false);
    expect(wrapper.find('.backup-inspection').exists()).toBe(false);
    expect(restore).not.toHaveBeenCalled();
    expect(job).not.toHaveBeenCalled();
    expect(mocks.warning).not.toHaveBeenCalled();
  });

  it('导出 Excel 完成后显示成功提示并解除维护状态', async () => {
    const path = 'D:/素材/all.xlsx';
    vi.spyOn(api, 'exportExcel').mockResolvedValue({ id: 'excel-export' });
    vi.spyOn(api, 'job').mockResolvedValue({
      id: 'excel-export', kind: 'excelExport', status: 'succeeded', stage: '导出 Excel',
      completed: 7, total: 7, cancellable: false, resultJson: JSON.stringify(path), errorJson: '',
    });
    wrapper = mount(SettingsPage, { props: { preferences } });
    await flushPromises();
    await wrapper.findAll('button').find(button => button.text() === '导出 Excel')!.trigger('click');
    await flushPromises();

    expect(mocks.success).toHaveBeenCalledWith(t('Excel exported: {p0}', { p0: path }));
    expect(wrapper.emitted('maintenance')).toEqual([[true], [false]]);
    expect(wrapper.text()).toContain('7 / 7');
  });

  it('取消保存对话框不启动任务或显示导出成功', async () => {
    mocks.success.mockClear();
    vi.spyOn(api, 'exportExcel').mockResolvedValue({ id: '' });
    const job = vi.spyOn(api, 'job');
    wrapper = mount(SettingsPage, { props: { preferences } });
    await flushPromises();
    await wrapper.findAll('button').find(button => button.text() === '导出 Excel')!.trigger('click');
    await flushPromises();

    expect(job).not.toHaveBeenCalled();
    expect(mocks.success).not.toHaveBeenCalled();
    expect(wrapper.emitted('maintenance')).not.toContainEqual([true]);
  });

  it('导出失败后显示错误并恢复操作', async () => {
    vi.spyOn(api, 'exportExcel').mockResolvedValue({ id: 'failed-export' });
    vi.spyOn(api, 'job').mockResolvedValue({
      id: 'failed-export', kind: 'excelExport', status: 'failed', stage: '导出 Excel',
      completed: 0, total: 7, cancellable: false, resultJson: '', errorJson: '磁盘空间不足',
    });
    wrapper = mount(SettingsPage, { props: { preferences } });
    await flushPromises();
    const button = wrapper.findAll('button').find(button => button.text() === '导出 Excel')!;
    await button.trigger('click');
    await flushPromises();

    expect(wrapper.text()).toContain('磁盘空间不足');
    expect(wrapper.emitted('maintenance')).toEqual([[true], [false]]);
    expect(button.attributes('disabled')).toBeUndefined();
  });

  it('设置中提供 Excel 导入入口', async () => {
    wrapper = mount(SettingsPage, { props: { preferences } });
    await flushPromises();
    expect(wrapper.find('.excel-card .n-card-header__main').text()).toBe('Excel 导入与导出');
    await wrapper.findAll('button').find(button => button.text() === '从 Excel 导入')!.trigger('click');
    expect(wrapper.emitted('import')).toHaveLength(1);
    expect(wrapper.find('.eyebrow').exists()).toBe(false);
  });

  it('错误显示后切换语言，现有提示也立即更新', async () => {
    vi.spyOn(api, 'info').mockRejectedValue(new Error('Asset not found'));
    wrapper = mount(SettingsPage, { props: { preferences } });
    await flushPromises();
    expect(wrapper.text()).toContain('素材不存在');

    setLanguage('en');
    await flushPromises();
    expect(wrapper.text()).toContain(t('Asset not found'));
    expect(wrapper.text()).not.toContain('素材不存在');

    setLanguage('zh-TW');
    await flushPromises();
    expect(wrapper.text()).toContain(t('Asset not found'));
  });

  it('提供默认跟随系统及英、简中、繁中、日、韩选项', async () => {
    wrapper = mount(SettingsPage, { props: { preferences } });
    await flushPromises();
    const selector = wrapper.findAllComponents(NSelect)[0];

    expect(selector.props('value')).toBe('system');
    expect(selector.props('options')).toEqual([
      { label: '跟随系统', value: 'system' },
      { label: 'English', value: 'en' },
      { label: '简体中文', value: 'zh-CN' },
      { label: '繁體中文', value: 'zh-TW' },
      { label: '日本語', value: 'ja' },
      { label: '한국어', value: 'ko' },
    ]);
    selector.vm.$emit('update:value', 'zh-TW');
    expect(wrapper.emitted('preferences')).toEqual([[{ ...preferences, language: 'zh-TW' }]]);
  });

  it('waits for pending preference writes to drain before starting restoration', async () => {
    vi.spyOn(api, 'inspectBackup').mockResolvedValue({
      inspectionId: 'checked',
      assetCount: 12,
      createdAt: '2026-10-15',
      formatVersion: '1',
      schemaVersion: 1,
      totalBytes: 100,
    });
    const restore = vi.spyOn(api, 'restore').mockResolvedValue({ id: 'job' });
    vi.spyOn(api, 'job').mockResolvedValue({
      id: 'job',
      kind: 'restore',
      status: 'succeeded',
      stage: 'done',
      completed: 1,
      total: 1,
      cancellable: false,
      resultJson: '{}',
      errorJson: '',
    });
    let drain!: () => void;
    const beforeRestore = vi.fn(() => new Promise<void>((resolve) => (drain = resolve)));
    wrapper = mount(SettingsPage, { props: { preferences, beforeRestore } });
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '选择备份')!
      .trigger('click');
    await flushPromises();
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '恢复…')!
      .trigger('click');
    const pending = mocks.warning.mock.calls[0][0].onPositiveClick();
    await flushPromises();

    expect(beforeRestore).toHaveBeenCalledOnce();
    expect(restore).not.toHaveBeenCalled();

    drain();
    await pending;

    expect(restore).toHaveBeenCalledWith('checked');
  });

  it('never offers overwrite confirmation for a damaged backup', async () => {
    vi.spyOn(api, 'inspectBackup').mockRejectedValue(new Error('校验失败'));
    const restore = vi.spyOn(api, 'restore');
    wrapper = mount(SettingsPage, { props: { preferences } });
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '选择备份')!
      .trigger('click');
    await flushPromises();

    expect(wrapper.text()).toContain('校验失败');
    expect(wrapper.text()).not.toContain('恢复…');
    expect(mocks.warning).not.toHaveBeenCalled();
    expect(restore).not.toHaveBeenCalled();
  });

  it('requires inspection and final overwrite approval and holds navigation through restoration', async () => {
    vi.spyOn(api, 'inspectBackup').mockResolvedValue({
      inspectionId: 'checked',
      assetCount: 12,
      createdAt: '2026-10-15',
      formatVersion: '1',
      schemaVersion: 1,
      totalBytes: 100,
    });
    const restore = vi.spyOn(api, 'restore').mockResolvedValue({ id: 'job' });
    vi.spyOn(api, 'job').mockResolvedValue({
      id: 'job',
      kind: 'restore',
      status: 'succeeded',
      stage: 'done',
      completed: 1,
      total: 1,
      cancellable: false,
      resultJson: '{}',
      errorJson: '',
    });
    wrapper = mount(SettingsPage, { props: { preferences } });
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '选择备份')!
      .trigger('click');
    await flushPromises();
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '恢复…')!
      .trigger('click');

    expect(restore).not.toHaveBeenCalled();

    const confirmation = mocks.warning.mock.calls[0][0];

    expect(confirmation.content).toContain('12 件素材将替换当前的 7 件素材');

    await confirmation.onPositiveClick();
    await flushPromises();

    expect(restore).toHaveBeenCalledWith('checked');
    expect(wrapper.emitted('maintenance')).toEqual([[true], [false]]);
    expect(wrapper.emitted('restored')).toHaveLength(1);
  });
});
