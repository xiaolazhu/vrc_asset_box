<script setup lang="ts">
import { confirmAction } from '../../shared/confirm-action';
import { formatDate, t } from '../../i18n';
import { nextTick, onMounted, ref, watch } from 'vue';
import {
  NDrawer,
  NDrawerContent,
  NImage,
  NTag,
  NButton,
  NSpace,
  NDescriptions,
  NDescriptionsItem,
  useMessage,
  useDialog,
} from 'naive-ui';
import type { Asset, Bootstrap } from '../../api/types';
import { api, errorMessage } from '../../api/client';
import CoverPlaceholderIcon from '../../shared/CoverPlaceholderIcon.vue';

const props = withDefaults(defineProps<{ asset: Asset; data: Bootstrap; show?: boolean }>(), { show: true });
const emit = defineEmits<{ close: []; afterLeave: []; edit: [asset: Asset]; deleted: [] }>();
const visible = ref(false);
const detailContent = ref<HTMLElement | null>(null);

// 浏览详情时聚焦内容，不默认突出关闭按钮；焦点约束和恢复仍由抽屉管理。
watch(visible, async (show) => {
  if (!show) {
    return;
  }

  await nextTick();
  if (visible.value) {
    detailContent.value?.focus({ preventScroll: true });
  }
}, { flush: 'post' });

// 延后一轮显示，让首次打开也触发抽屉自带的滑入动画。
watch(() => props.show, (show) => {
  visible.value = show;
});

onMounted(() => {
  visible.value = props.show;
});
const message = useMessage();
const dialog = useDialog();

async function action(fn: () => Promise<unknown>, successMessage?: string) {
  try {
    await fn();
    if (successMessage) {
      message.success(t(successMessage));
    }
  } catch (e) {
    message.error(errorMessage(e));
  }
}

function remove() {
  confirmAction(dialog, {
    title: t('Delete asset?'),
    content: t('Delete the record and cover for “{p0}”. Files in the linked folder are preserved. Deleted data can only be restored from a full backup.', { p0: props.asset.name }),
    positiveText: t('Delete asset'),
    negativeText: t('Keep'),
    onPositiveClick: async () => {
      try {
        await api.batch([props.asset.id], 'delete');
        message.success(t('Asset deleted'));
        emit('deleted');
      } catch (e) {
        message.error(errorMessage(e));
      }
    },
  });
}
</script>

<template>
  <n-drawer
    :show="visible"
    @after-leave="emit('afterLeave')"
    :width="580"
    :mask-closable="true"
    :auto-focus="false"
    @update:show="(v) => {
        if (!v) emit('close');
      }
    "
  >
    <n-drawer-content :title="t('Asset details')" closable>
      <div ref="detailContent" class="detail-content" tabindex="-1" :aria-label="t('Asset details')">
        <div class="detail-cover">
          <n-image v-if="asset.coverUrl" :src="asset.coverUrl" object-fit="contain" width="100%" />
          <div v-else class="image-placeholder">
            <CoverPlaceholderIcon />
            <small>{{ t('No cover') }}</small>
          </div>
        </div>
        <div class="detail-heading">
          <n-tag :bordered="false" size="small">
            {{ data.categories.find((c) => c.id === asset.categoryId)?.name || t('Uncategorized') }}
          </n-tag>
          <h2>{{ asset.name }}</h2>
        </div>
        <n-space>
          <n-button type="primary" @click="emit('edit', asset)">{{ t('Edit asset') }}</n-button>
          <n-button v-if="asset.folderPath" @click="action(() => api.openFolder(asset.folderPath))">{{ t('Open folder') }}</n-button>
          <n-button v-if="asset.boothId" @click="action(() => api.openBooth(asset.boothId))">{{ t('Booth ↗') }}</n-button>
        </n-space>
        <section class="detail-section">
          <h4>{{ t('Compatible avatars') }}</h4>
          <n-space>
            <n-tag v-if="asset.universal" type="success">{{ t('Universal compatibility') }}</n-tag>
            <n-tag v-for="tag in asset.avatars" :key="tag.id">{{ tag.name }}</n-tag>
            <span v-if="!asset.universal && !asset.avatars?.length" class="subtle">{{ t('No avatar compatibility set') }}</span>
          </n-space>
          <h4>{{ t('Custom tags') }}</h4>
          <n-space>
            <n-tag v-for="tag in asset.freeTags" :key="tag.id" :bordered="false">
              {{ tag.name }}
            </n-tag>
            <span v-if="!asset.freeTags?.length" class="subtle">{{ t('No custom tags') }}</span>
          </n-space>
          <h4>{{ t('Notes') }}</h4>
          <p class="notes-text">{{ asset.notes || t('No notes') }}</p>
        </section>
        <n-descriptions :column="1" label-placement="left" bordered>
          <n-descriptions-item :label="t('Booth ID')">
            <div class="section-line">
              <button
                v-if="asset.boothId"
                type="button"
                class="copyable-value"
                @click="action(() => api.copy(asset.boothId), 'ID copied')"
              >{{ asset.boothId }}</button>
              <span v-else>{{ t('Not recorded') }}</span>
            </div>
          </n-descriptions-item>
          <n-descriptions-item :label="t('Folder')">
            <button
              v-if="asset.folderPath"
              type="button"
              class="copyable-value path-text"
              @click="action(() => api.copy(asset.folderPath), 'Path copied')"
            >{{ asset.folderPath }}</button>
            <div v-else>{{ t('Not linked') }}</div>
          </n-descriptions-item>
          <n-descriptions-item :label="t('Created')">
            {{ formatDate(asset.createdAt) }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('Updated')">
            {{ formatDate(asset.updatedAt) }}
          </n-descriptions-item>
        </n-descriptions>
        <n-space style="margin-top: 20px">
          <n-button
            v-if="asset.boothId"
            size="small"
            @click="action(() => api.copy(`https://booth.pm/zh-cn/items/${asset.boothId}`), 'Link copied')"
          >{{ t('Copy product URL') }}</n-button>
          <n-button type="error" quaternary size="small" @click="remove">{{ t('Delete asset') }}</n-button>
        </n-space>
      </div>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
.detail-content:focus {
  outline: none;
}

.copyable-value {
  max-width: 100%;
  padding: 0;
  border: 0;
  color: inherit;
  background: transparent;
  font: inherit;
  text-align: left;
  overflow-wrap: anywhere;
  cursor: pointer;
}

.copyable-value:hover {
  color: var(--primary);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.copyable-value:focus-visible {
  border-radius: 2px;
  outline: 2px solid var(--primary);
  outline-offset: 3px;
}
</style>
