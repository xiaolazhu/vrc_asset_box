import { ref, watch, type Ref } from 'vue';

// 短请求不显示提示；请求结束或组件卸载时取消计时，不延迟数据展示。
export function useDelayedLoading(loading: Ref<boolean>, delay = 200) {
  const visible = ref(false);

  watch(loading, (active, _previous, onCleanup) => {
    visible.value = false;
    if (!active) {
      return;
    }

    const timer = setTimeout(() => {
      visible.value = true;
    }, delay);

    onCleanup(() => clearTimeout(timer));
  }, { immediate: true, flush: 'sync' });

  return visible;
}
