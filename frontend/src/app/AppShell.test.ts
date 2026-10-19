import { afterEach, describe, expect, it, vi } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import AppShell from './AppShell.vue';
import AssetLibrary from '../features/library/AssetLibrary.vue';
import SettingsPage from '../features/settings/SettingsPage.vue';
import ImportWizard from '../features/import/ImportWizard.vue';
import AssetEditor from '../features/editor/AssetEditor.vue';
import AssetDetail from '../features/library/AssetDetail.vue';
import { api } from '../api/client';
import { createQuery } from '../features/editor/draft';
import { locale, setLanguage, t } from '../i18n';

vi.mock('naive-ui', async (original) => ({
  ...(await original<typeof import('naive-ui')>()),
  useMessage: () => ({ warning: vi.fn(), error: vi.fn() }),
}));

let wrapper: ReturnType<typeof shallowMount> | undefined;

afterEach(() => {
  wrapper?.unmount();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe('preferences during library restore', () => {
  it('编辑器和详情关闭后保留到退场结束，再卸载组件', async () => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: () => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() }),
    });
    vi.spyOn(api, 'bootstrap').mockResolvedValue({
      categories: [], avatarTags: [], freeTags: [],
      preferences: {
        language: 'zh-CN', theme: 'system', view: 'cards', cardSize: 'medium', query: createQuery(),
      },
    });
    const asset = { id: 'asset-1', name: '测试素材' } as Awaited<ReturnType<typeof api.get>>;
    vi.spyOn(api, 'get').mockResolvedValue(asset);
    wrapper = shallowMount(AppShell);
    await flushPromises();

    wrapper.findComponent(AssetLibrary).vm.$emit('create');
    await flushPromises();
    const editor = wrapper.findComponent(AssetEditor);
    editor.vm.$emit('close');
    await flushPromises();
    expect(wrapper.findComponent(AssetEditor).exists()).toBe(true);
    expect(editor.props('show')).toBe(false);
    editor.vm.$emit('afterLeave');
    await flushPromises();
    expect(wrapper.findComponent(AssetEditor).exists()).toBe(false);

    wrapper.findComponent(AssetLibrary).vm.$emit('open', asset.id);
    await flushPromises();
    const detail = wrapper.findComponent(AssetDetail);
    detail.vm.$emit('close');
    await flushPromises();
    expect(wrapper.findComponent(AssetDetail).exists()).toBe(true);
    expect(detail.props('show')).toBe(false);
    detail.vm.$emit('afterLeave');
    await flushPromises();
    expect(wrapper.findComponent(AssetDetail).exists()).toBe(false);
  });

  it('未分类在首位，普通分类默认五个并可展开收起', async () => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: () => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() }),
    });
    vi.spyOn(api, 'bootstrap').mockResolvedValue({
      categories: Array.from({ length: 8 }, (_, i) => ({
        id: `category-${i}`, name: `分类 ${i + 1}`, position: i, count: 0,
      })),
      avatarTags: [], freeTags: [],
      preferences: {
        language: 'zh-CN', theme: 'system', view: 'cards', cardSize: 'medium', query: createQuery(),
      },
    });
    wrapper = shallowMount(AppShell);
    await flushPromises();

    expect(wrapper.findAll('.category-nav')).toHaveLength(6);
    expect(wrapper.findAll('.category-nav')[0].text()).toContain('未分类');
    expect(wrapper.find('.category-toggle').text()).toContain('3');
    expect(wrapper.find('.category-toggle').attributes('aria-expanded')).toBe('false');
    await wrapper.find('.category-toggle').trigger('click');
    expect(wrapper.findAll('.category-nav')).toHaveLength(9);
    expect(wrapper.find('.category-toggle').attributes('aria-expanded')).toBe('true');
    await wrapper.find('.category-toggle').trigger('click');
    expect(wrapper.findAll('.category-nav')).toHaveLength(6);
    expect(wrapper.findAll('.category-nav')[0].text()).toContain('未分类');
    expect(wrapper.text()).toContain('素材标签');
  });

  it('Excel 只从设置进入，返回时回到设置并保持导航选中', async () => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: () => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() }),
    });
    vi.spyOn(api, 'bootstrap').mockResolvedValue({
      categories: [], avatarTags: [], freeTags: [],
      preferences: {
        language: 'zh-CN', theme: 'system', view: 'cards', cardSize: 'medium', query: createQuery(),
      },
    });
    wrapper = shallowMount(AppShell);
    await flushPromises();
    expect(wrapper.findAll('.sidebar button').some(button => button.text().includes('Excel'))).toBe(false);
    await wrapper.findAll('button').find(button => button.text().includes('设置'))!.trigger('click');
    wrapper.findComponent(SettingsPage).vm.$emit('import');
    await flushPromises();

    expect(wrapper.findComponent(SettingsPage).exists()).toBe(false);
    expect(wrapper.findComponent(ImportWizard).isVisible()).toBe(true);
    expect(wrapper.find('.sidebar-bottom .nav-item.active').text()).toContain('设置');

    wrapper.findComponent(ImportWizard).vm.$emit('back');
    await flushPromises();
    expect(wrapper.findComponent(SettingsPage).exists()).toBe(true);
    expect(wrapper.findComponent(ImportWizard).element.parentElement?.style.display).toBe('none');
  });

  it('立即切换简繁语言，通知桌面对话框并保存偏好，恢复时重新应用语言', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: () => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() }),
    });
    const preferences = {
      language: 'zh-CN' as const,
      theme: 'light' as const,
      view: 'cards' as const,
      cardSize: 'medium' as const,
      query: createQuery(),
    };
    const bootstrap = vi.spyOn(api, 'bootstrap').mockResolvedValue({
      categories: [{ id: 'clothes', name: '衣服', position: 0, count: 3 }],
      avatarTags: [],
      freeTags: [],
      systemLocale: 'ja-JP',
      preferences,
    });
    const nativeLanguage = vi.spyOn(api, 'setLanguage').mockResolvedValue();
    const save = vi.spyOn(api, 'preferences')
      .mockRejectedValueOnce(new Error('Asset not found'))
      .mockResolvedValue();
    wrapper = shallowMount(AppShell, { global: { renderStubDefaultSlot: true } });
    await flushPromises();
    await wrapper.findAll('button').find((button) => button.text().includes('设置'))!
      .trigger('click');

    const settings = wrapper.findComponent(SettingsPage);
    settings.vm.$emit('preferences', { ...preferences, language: 'zh-TW' });
    await flushPromises();

    expect(locale.value).toBe('zh-TW');
    expect(wrapper.text()).toContain('設定');
    expect(wrapper.text()).toContain('衣服');
    expect(nativeLanguage).toHaveBeenCalledWith('zh-TW');
    expect(save).not.toHaveBeenCalled();

    vi.advanceTimersByTime(350);
    await flushPromises();
    expect(save).toHaveBeenCalledWith(expect.objectContaining({ language: 'zh-TW' }));
    expect(wrapper.text()).toContain(t('Preferences were not saved: {p0}', { p0: t('Asset not found') }));

    settings.vm.$emit('preferences', { ...preferences, language: 'system' });
    await flushPromises();
    expect(locale.value).toBe('ja');
    expect(wrapper.text()).toContain(t('Preferences were not saved: {p0}', { p0: t('Asset not found') }));

    const refresh = vi.spyOn(api, 'refreshSystemLocale').mockResolvedValue('ko-KR');
    window.dispatchEvent(new Event('languagechange'));
    await flushPromises();
    expect(refresh).toHaveBeenCalledOnce();
    expect(locale.value).toBe('ko');

    bootstrap.mockResolvedValue({ categories: [], avatarTags: [], freeTags: [], preferences });
    settings.vm.$emit('restored');
    await flushPromises();
    expect(locale.value).toBe('zh-CN');
    window.dispatchEvent(new Event('languagechange'));
    await flushPromises();
    expect(refresh).toHaveBeenCalledOnce();
    setLanguage('zh-CN');
  });

  it('drains the active save and discards queued and debounced saves before restoring', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn(() => ({
        matches: false,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    });
    const preferences = {
      language: 'zh-CN' as const,
      theme: 'light' as const,
      view: 'cards' as const,
      cardSize: 'medium' as const,
      query: createQuery(),
    };
    vi.spyOn(api, 'bootstrap').mockResolvedValue({
      categories: [],
      avatarTags: [],
      freeTags: [],
      preferences,
    });
    let completeSave!: () => void;
    const save = vi
      .spyOn(api, 'preferences')
      .mockImplementationOnce(() => new Promise<void>((resolve) => (completeSave = resolve)))
      .mockResolvedValue();
    wrapper = shallowMount(AppShell);
    await flushPromises();
    const library = wrapper.findComponent(AssetLibrary);
    library.vm.$emit('preferences', { ...preferences, cardSize: 'small' });
    vi.advanceTimersByTime(350);
    await flushPromises();

    expect(save).toHaveBeenCalledTimes(1);

    library.vm.$emit('preferences', { ...preferences, cardSize: 'large' });
    vi.advanceTimersByTime(350);
    await flushPromises();
    library.vm.$emit('preferences', { ...preferences, theme: 'dark' });
    await wrapper
      .findAll('button')
      .find((b) => b.text().includes('设置'))!
      .trigger('click');
    const settings = wrapper.findComponent(SettingsPage);
    settings.vm.$emit('maintenance', true);
    const drain = (settings.props('beforeRestore') as () => Promise<void>)();
    let ready = false;
    void drain.then(() => (ready = true));
    vi.advanceTimersByTime(350);
    await flushPromises();

    expect(ready).toBe(false);

    completeSave();
    await drain;
    await flushPromises();

    expect(save).toHaveBeenCalledTimes(1);

    settings.vm.$emit('maintenance', false);
    settings.vm.$emit('preferences', { ...preferences, theme: 'dark' });
    vi.advanceTimersByTime(350);
    await flushPromises();

    expect(save).toHaveBeenCalledTimes(2);
  });
});
