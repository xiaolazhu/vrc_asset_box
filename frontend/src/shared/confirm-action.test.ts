import { describe, expect, it, vi } from 'vitest';
import type { DialogApi, DialogOptions, DialogReactive } from 'naive-ui';
import { confirmAction } from './confirm-action';

describe('提交确认框的关闭保护', () => {
  it('等待异步提交期间锁定关闭，失败后恢复并保留确认框', async () => {
    let options!: DialogOptions;
    const instance = {} as DialogReactive;
    const dialog = { warning: vi.fn((value: DialogOptions) => {
      options = value;
      return instance;
    }) } as unknown as DialogApi;
    let fail!: (reason: Error) => void;
    confirmAction(dialog, {
      onPositiveClick: () => new Promise((_, reject) => { fail = reject; }),
    });
    const pending = options.onPositiveClick!(new MouseEvent('click'));
    expect(instance.maskClosable).toBe(false);
    expect(instance.closeOnEsc).toBe(false);
    expect(instance.closable).toBe(false);
    fail(new Error('提交失败'));
    await expect(pending).rejects.toThrow('提交失败');
    expect(instance.maskClosable).toBe(true);
    expect(instance.closeOnEsc).toBe(true);
    expect(instance.closable).toBe(true);
  });
});
