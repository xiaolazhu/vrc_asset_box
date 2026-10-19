import { describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';
import { NProgress } from 'naive-ui';
import JobProgress from './JobProgress.vue';
import type { JobSnapshot } from '../api/types';

function snapshot(status: JobSnapshot['status']): JobSnapshot {
  return {
    id: 'backup', kind: 'backup', status, stage: 'Complete',
    completed: 1, total: 1, cancellable: false, resultJson: '', errorJson: '',
  };
}

describe('任务进度状态', () => {
  it('成功显示完整进度和成功色，停止加载动画', () => {
    const wrapper = mount(JobProgress, { props: { job: snapshot('succeeded') } });
    const progress = wrapper.findComponent(NProgress);
    expect(progress.props('percentage')).toBe(100);
    expect(progress.props('status')).toBe('success');
    expect(progress.props('processing')).toBe(false);
    expect(wrapper.find('small').text()).toBe('1 / 1');
    wrapper.unmount();
  });

  it.each(['failed', 'cancelled'] as const)('%s 保留实际进度并停止动画', (status) => {
    const wrapper = mount(JobProgress, {
      props: { job: { ...snapshot(status), completed: 1, total: 2 } },
    });
    const progress = wrapper.findComponent(NProgress);
    expect(progress.props('percentage')).toBe(50);
    expect(progress.props('processing')).toBe(false);
    expect(progress.props('status')).toBe(status === 'failed' ? 'error' : 'warning');
    expect(wrapper.find('small').text()).toBe('1 / 2');
    wrapper.unmount();
  });
});
