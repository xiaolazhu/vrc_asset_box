import type { DialogApi, DialogOptions } from 'naive-ui';

// 二次确认默认允许点遮罩返回；提交期间锁定关闭，等待操作完成。
export function confirmAction(dialog: DialogApi, options: DialogOptions) {
  let submitting = false;
  const confirmation = dialog.warning({
    ...options,
    onPositiveClick: async (event) => {
      if (submitting) {
        return false;
      }

      submitting = true;
      const idle = {
        maskClosable: options.maskClosable ?? true,
        closeOnEsc: options.closeOnEsc ?? true,
        closable: options.closable ?? true,
        loading: options.loading ?? false,
        negativeButtonProps: options.negativeButtonProps,
      };

      Object.assign(confirmation || {}, {
        maskClosable: false,
        closeOnEsc: false,
        closable: false,
        loading: true,
        negativeButtonProps: { ...options.negativeButtonProps, disabled: true },
      });

      try {
        return await options.onPositiveClick?.(event);
      } finally {
        submitting = false;
        Object.assign(confirmation || {}, idle);
      }
    },
  });

  return confirmation;
}
