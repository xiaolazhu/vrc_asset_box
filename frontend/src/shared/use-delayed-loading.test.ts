import { afterEach, describe, expect, it, vi } from 'vitest';
import { effectScope, ref } from 'vue';
import { useDelayedLoading } from './use-delayed-loading';

afterEach(() => vi.useRealTimers());

describe('延迟加载提示', () => {
  it('快速完成时不显示，慢请求显示，完成后立即隐藏', () => {
    vi.useFakeTimers();
    const scope = effectScope();
    const loading = ref(true);
    const visible = scope.run(() => useDelayedLoading(loading))!;

    expect(visible.value).toBe(false);
    vi.advanceTimersByTime(100);
    loading.value = false;
    vi.advanceTimersByTime(200);
    expect(visible.value).toBe(false);

    loading.value = true;
    vi.advanceTimersByTime(199);
    expect(visible.value).toBe(false);
    vi.advanceTimersByTime(1);
    expect(visible.value).toBe(true);
    loading.value = false;
    expect(visible.value).toBe(false);
    scope.stop();
  });

  it('组件卸载后取消待显示的提示', () => {
    vi.useFakeTimers();
    const scope = effectScope();
    const visible = scope.run(() => useDelayedLoading(ref(true)))!;

    scope.stop();
    vi.advanceTimersByTime(300);
    expect(visible.value).toBe(false);
    expect(vi.getTimerCount()).toBe(0);
  });
});
