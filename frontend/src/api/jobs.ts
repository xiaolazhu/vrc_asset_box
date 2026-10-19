import { t } from '../i18n';
import { api } from './client';
import type { JobSnapshot } from './types';

export async function waitJob<T>(
  id: string,
  onProgress?: (s: JobSnapshot) => void,
  signal?: AbortSignal,
): Promise<T> {
  while (true) {
    signal?.throwIfAborted();
    const snapshot = await api.job(id);
    signal?.throwIfAborted();
    onProgress?.(snapshot);
    signal?.throwIfAborted();
    if (snapshot.status === 'succeeded') {
      return JSON.parse(snapshot.resultJson) as T;
    }

    if (snapshot.status === 'failed') {
      throw new Error(snapshot.errorJson);
    }

    if (snapshot.status === 'cancelled') {
      throw new Error(t('Operation canceled'));
    }

    // 等待期间也响应取消，避免组件卸载后还要等下一个轮询周期。
    await new Promise<void>((resolve, reject) => {
      const abort = () => {
        clearTimeout(timer);
        reject(signal?.reason || new DOMException('Polling aborted', 'AbortError'));
      };
      const timer = setTimeout(() => {
        signal?.removeEventListener('abort', abort);
        resolve();
      }, 400);
      signal?.addEventListener('abort', abort, { once: true });
    });
  }
}
