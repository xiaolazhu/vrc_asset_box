import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from './client';
import { waitJob } from './jobs';

afterEach(() => vi.restoreAllMocks());

describe('job polling lifecycle', () => {
  it('stops before making another request when its consumer is gone', async () => {
    const request = vi.spyOn(api, 'job').mockResolvedValue({
      id: 'a',
      kind: 'preview',
      status: 'running',
      stage: 'read',
      completed: 0,
      total: 10,
      cancellable: true,
      resultJson: '',
      errorJson: '',
    });
    const controller = new AbortController();
    const result = waitJob('a', () => controller.abort(), controller.signal);
    await expect(result).rejects.toMatchObject({ name: 'AbortError' });

    expect(request).toHaveBeenCalledTimes(1);
  });

  it('preserves structured failure details for duplicate-record navigation', async () => {
    const errorJson = JSON.stringify({
      code: 'DUPLICATE_BOOTH',
      message: '已有记录',
      existingAssetId: 'original',
    });
    vi.spyOn(api, 'job').mockResolvedValue({
      id: 'a',
      kind: 'booth',
      status: 'failed',
      stage: '',
      completed: 0,
      total: 0,
      cancellable: false,
      resultJson: '',
      errorJson,
    });
    await expect(waitJob('a')).rejects.toThrow(errorJson);
  });
});
