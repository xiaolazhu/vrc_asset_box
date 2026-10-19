import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import AssetDetail from './AssetDetail.vue';
import { createDraft, createQuery } from '../editor/draft';
import { api } from '../../api/client';
import { t } from '../../i18n';

const messages = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('naive-ui', async (original) => ({
  ...(await original<typeof import('naive-ui')>()),
  useMessage: () => messages,
  useDialog: () => ({ warning: vi.fn() }),
}));

let wrapper: ReturnType<typeof mount> | undefined;
let trigger: HTMLButtonElement | undefined;

afterEach(() => {
  wrapper?.unmount();
  trigger?.remove();
  vi.restoreAllMocks();
  messages.success.mockClear();
  messages.error.mockClear();
});

function mountDetail(assetOverrides: Record<string, unknown> = {}) {
  return mount(AssetDetail, {
    attachTo: document.body,
    global: { stubs: { transition: false } },
    props: {
      asset: {
        ...createDraft(), id: 'asset-1', name: '测试素材', avatars: [], freeTags: [],
        coverUrl: '', thumbnailUrl: '', createdAt: '', updatedAt: '',
        ...assetOverrides,
      },
      data: {
        categories: [], avatarTags: [], freeTags: [],
        preferences: { theme: 'light', view: 'cards', cardSize: 'medium', query: createQuery() },
      },
    },
  });
}

describe('详情字段复制', () => {
  it('点击 ID 和文件夹路径文字即可复制并显示提示，不显示单独复制按钮', async () => {
    const copy = vi.spyOn(api, 'copy').mockResolvedValue();
    wrapper = mountDetail({ boothId: '123456', folderPath: 'D:/Assets/Hair' });
    await flushPromises();

    const copyableValues = document.querySelectorAll<HTMLButtonElement>('button.copyable-value');
    expect(copyableValues).toHaveLength(2);
    expect(document.body.textContent).not.toContain(t('Copy ID'));
    expect(document.body.textContent).not.toContain(t('Copy path'));

    copyableValues[0].click();
    copyableValues[1].click();
    await flushPromises();

    expect(copy).toHaveBeenNthCalledWith(1, '123456');
    expect(copy).toHaveBeenNthCalledWith(2, 'D:/Assets/Hair');
    expect(messages.success).toHaveBeenNthCalledWith(1, t('ID copied'));
    expect(messages.success).toHaveBeenNthCalledWith(2, t('Path copied'));
  });
});

describe('详情初始焦点', () => {
  it('打开聚焦内容而非关闭按钮，保留关闭按钮聚焦、Esc 和焦点恢复', async () => {
    trigger = document.createElement('button');
    document.body.append(trigger);
    trigger.focus();
    wrapper = mountDetail();
    await flushPromises();
    const content = document.querySelector<HTMLElement>('.detail-content');
    const close = document.querySelector<HTMLButtonElement>('.n-drawer-header__close')!;
    expect(content).not.toBeNull();
    expect(document.activeElement).toBe(content);
    expect(document.activeElement).not.toBe(close);
    expect(close.tabIndex).toBe(0);
    close.focus();
    expect(document.activeElement).toBe(close);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', code: 'Escape', bubbles: true }));
    await flushPromises();
    expect(wrapper.emitted('close')).toHaveLength(1);
    await wrapper.setProps({ show: false });
    await flushPromises();
    expect(document.activeElement).toBe(trigger);
  });
});
