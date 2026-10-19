<script setup lang="ts">
import { confirmAction } from '../../shared/confirm-action';
import { formatDate, formatNumber, localize, t } from '../../i18n';
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import {
  NCard,
  NSelect,
  NButton,
  NAlert,
  NDescriptions,
  NDescriptionsItem,
  NSpace,
  useDialog,
  useMessage,
} from 'naive-ui';
import type { Preferences, WorkspaceInfo, JobSnapshot, BackupInfo } from '../../api/types';
import { api, sourceErrorMessage } from '../../api/client';
import { waitJob } from '../../api/jobs';
import JobProgress from '../../shared/JobProgress.vue';

const props = defineProps<{ preferences: Preferences; beforeRestore?: () => Promise<void> }>();
const emit = defineEmits<{
  preferences: [p: Preferences];
  restored: [];
  import: [];
  maintenance: [busy: boolean];
}>();

// 资料库概况与备份任务状态。
const info = ref<WorkspaceInfo | null>(null);
const error = ref('');
const operation = ref<'backup' | 'inspect' | 'excelExport' | 'restore' | null>(null);
const busy = computed(() => operation.value !== null);
const job = ref<JobSnapshot | null>(null);
const inspection = ref<BackupInfo | null>(null);
const message = useMessage();
const dialog = useDialog();

// 页面卸载时终止轮询，防止后台结果继续更新当前页面。
let polling = new AbortController();
let disposed = false;

async function load() {
  try {
    info.value = await api.info();
  } catch (e) {
    error.value = sourceErrorMessage(e);
  }
}

function preference(key: keyof Preferences, value: unknown) {
  emit('preferences', { ...props.preferences, [key]: value });
}

async function backup() {
  operation.value = 'backup';
  error.value = '';

  try {
    polling.abort();
    polling = new AbortController();
    const ref = await api.backup();
    if (!ref.id) {
      return;
    }
    if (disposed) {
      await api.cancel(ref.id);
      return;
    }

    await waitJob(ref.id, (s) => (job.value = s), polling.signal);
    if (!disposed) {
      message.success(t('Full backup exported'));
    }
  } catch (e) {
    error.value = sourceErrorMessage(e);
  } finally {
    operation.value = null;
  }
}

async function inspect() {
  operation.value = 'inspect';
  error.value = '';
  inspection.value = null;

  try {
    const checked = await api.inspectBackup();
    if (!disposed && checked.inspectionId) {
      inspection.value = checked;
    }
  } catch (e) {
    error.value = sourceErrorMessage(e);
  } finally {
    operation.value = null;
  }
}

async function exportExcel() {
  operation.value = 'excelExport';
  error.value = '';
  job.value = null;

  try {
    polling.abort();
    polling = new AbortController();
    const task = await api.exportExcel();
    if (!task.id) {
      return;
    }
    if (disposed) {
      await api.cancel(task.id);
      return;
    }

    emit('maintenance', true);
    const path = await waitJob<string>(task.id, (snapshot) => (job.value = snapshot), polling.signal);
    if (!disposed) {
      message.success(t('Excel exported: {p0}', { p0: path }));
    }
  } catch (e) {
    if (!disposed) {
      error.value = sourceErrorMessage(e);
    }
  } finally {
    operation.value = null;
    emit('maintenance', false);
  }
}

function restore() {
  const checked = inspection.value;
  if (!checked) {
    return;
  }

  confirmAction(dialog, {
    title: t('Replace the entire current asset library?'),
    content: t(
      'Replace the current {p1} assets with {p0} assets from this backup, including all covers, tags, categories and settings. The current library will be backed up first. Linked files will not be moved or deleted.',
      {
        p0: checked.assetCount,
        p1: info.value?.assetCount ?? t('All'),
      },
    ),
    positiveText: t('Restore library'),
    negativeText: t('Cancel'),
    onPositiveClick: async () => {
      operation.value = 'restore';
      emit('maintenance', true);
      error.value = '';

      try {
        await props.beforeRestore?.();
        if (disposed) {
          return;
        }

        polling.abort();
        polling = new AbortController();
        const ref = await api.restore(checked.inspectionId);
        if (disposed) {
          return;
        }

        await waitJob(ref.id, (s) => (job.value = s), polling.signal);
        if (disposed) {
          return;
        }

        inspection.value = null;
        emit('restored');
        await load();
        message.success(t('Asset library restored'));
      } catch (e) {
        error.value = sourceErrorMessage(e);
      } finally {
        operation.value = null;
        emit('maintenance', false);
      }
    },
  });
}

async function openData() {
  try {
    await api.openData();
  } catch (e) {
    error.value = sourceErrorMessage(e);
  }
}

onMounted(load);

onBeforeUnmount(() => {
  disposed = true;
  polling.abort();
  if (job.value?.status === 'running' && job.value.cancellable) {
    void api.cancel(job.value.id).catch(() => {});
  }
});
</script>

<template>
  <div class="settings-page">
    <header class="page-heading">
      <div>
        <h1>{{ t('Settings') }}</h1>
      </div>
    </header>
    <n-alert v-if="error" type="error" closable style="margin-bottom: 20px" @close="error = ''">
      {{ localize(error) }}
    </n-alert>
    <div class="settings-grid">
      <n-card :title="t('Appearance & browsing')" :bordered="false">
        <div class="setting-row">
          <div>
            <strong>{{ t('Language') }}</strong>
            <p>{{ t('Changes apply immediately') }}</p>
          </div>
          <n-select
            :value="preferences.language || 'system'"
            :options="[
              { label: t('Follow system'), value: 'system' },
              { label: 'English', value: 'en' },
              { label: '简体中文', value: 'zh-CN' },
              { label: '繁體中文', value: 'zh-TW' },
              { label: '日本語', value: 'ja' },
              { label: '한국어', value: 'ko' },
            ]"
            @update:value="(value) => preference('language', value)"
          />
        </div>
        <div class="setting-row">
          <div>
            <strong>{{ t('Theme') }}</strong>
            <p>{{ t('Set the app appearance') }}</p>
          </div>
          <n-select
            :value="preferences.theme"
            :options="[
              { label: t('Follow system'), value: 'system' },
              { label: t('Light'), value: 'light' },
              { label: t('Dark'), value: 'dark' },
            ]"
            @update:value="(v) => preference('theme', v)"
          />
        </div>
        <div class="setting-row">
          <div>
            <strong>{{ t('Default view') }}</strong>
            <p>{{ t('Search and filters are saved automatically') }}</p>
          </div>
          <n-select
            :value="preferences.view"
            :options="[
              { label: t('Cards'), value: 'cards' },
              { label: t('Table'), value: 'table' },
            ]"
            @update:value="(v) => preference('view', v)"
          />
        </div>
        <div class="setting-row">
          <div>
            <strong>{{ t('Card size') }}</strong>
            <p>{{ t('Adjust card density') }}</p>
          </div>
          <n-select
            :value="preferences.cardSize"
            :options="[
              { label: t('Compact'), value: 'small' },
              { label: t('Standard'), value: 'medium' },
              { label: t('Spacious'), value: 'large' },
            ]"
            @update:value="(v) => preference('cardSize', v)"
          />
        </div>
      </n-card>
      <n-card :title="t('Local library')" :bordered="false">
        <n-descriptions :column="1" label-placement="top">
          <n-descriptions-item :label="t('Data location')">
            <code class="path-text">{{ info?.dataPath || t('Reading…') }}</code>
          </n-descriptions-item>
          <n-descriptions-item :label="t('Asset count')">
            {{ t(
                '{p0} assets · Data version {p1}',
                {
                  p0: info?.assetCount ?? '—',
                  p1: info?.schemaVersion ?? '—',
                },
              ) }}
          </n-descriptions-item>
        </n-descriptions>
        <n-button style="margin-top: 16px" @click="openData">{{ t('Open data folder') }}</n-button>
      </n-card>
      <n-card :title="t('Excel import and export')" :bordered="false" class="backup-card excel-card">
        <div class="backup-actions">
          <div>
            <h3>{{ t('Excel import') }}</h3>
            <p class="subtle">{{ t('Map fields and review before importing. The source file will not be modified.') }}</p>
          </div>
          <n-button :disabled="busy" @click="emit('import')">{{ t('Import from Excel') }}</n-button>
        </div>
        <div class="backup-actions">
          <div>
            <h3>{{ t('Excel export') }}</h3>
            <p class="subtle">{{ t('Export all asset records, tags, cover thumbnails and linked folder paths.') }}</p>
          </div>
          <n-button :loading="operation === 'excelExport'" :disabled="busy" @click="exportExcel">{{ t('Export Excel') }}</n-button>
        </div>
        <JobProgress v-if="job?.kind === 'excelExport'" :job="job" @error="error = $event" />
      </n-card>
      <n-card :title="t('Backup and restore')" :bordered="false" class="backup-card">
        <n-alert type="info" :bordered="false">{{ t('Backups include records, covers, categories, tags, aliases and settings. Back up linked asset files separately.') }}</n-alert>
        <div class="backup-actions">
          <div>
            <h3>{{ t('Export full backup') }}</h3>
            <p class="subtle">{{ t('Save a full backup to a location you choose.') }}</p>
          </div>
          <n-button :loading="operation === 'backup'" :disabled="busy" @click="backup">{{ t('Export backup') }}</n-button>
        </div>
        <div class="backup-actions">
          <div>
            <h3>{{ t('Restore from backup') }}</h3>
            <p class="subtle">{{ t('Validate a backup, then confirm replacement of the entire library.') }}</p>
          </div>
          <n-button :loading="operation === 'inspect'" :disabled="busy" @click="inspect">{{ t('Select backup') }}</n-button>
        </div>
        <div v-if="inspection" class="backup-inspection">
          <h4>{{ t('Backup passed validation') }}</h4>
          <n-descriptions :column="3">
            <n-descriptions-item :label="t('Backup date')">
              {{ formatDate(inspection.createdAt) }}
            </n-descriptions-item>
            <n-descriptions-item :label="t('Asset count')">{{ inspection.assetCount }}</n-descriptions-item>
            <n-descriptions-item :label="t('Size')">
              {{ t('{p0} MiB', { p0: formatNumber(inspection.totalBytes / 1024 / 1024, 1) }) }}</n-descriptions-item>
          </n-descriptions>
          <n-button type="error" :loading="operation === 'restore'" :disabled="busy" @click="restore">{{ t('Restore…') }}</n-button>
        </div>
        <JobProgress v-if="job?.kind !== 'excelExport'" :job="job" @error="error = $event" />
      </n-card>
    </div>
    <p class="settings-footnote">{{ t('Shortcuts: Ctrl+F to search · Ctrl+N to add an asset') }}</p>
  </div>
</template>
