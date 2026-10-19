<script setup lang="ts">
import { localize, t } from '../i18n';
import { computed } from 'vue';
import { NProgress, NButton, NAlert } from 'naive-ui';
import type { JobSnapshot } from '../api/types';
import { api, errorMessage } from '../api/client';

const props = defineProps<{ job: JobSnapshot | null }>();
const emit = defineEmits<{ error: [message: string] }>();
const percentage = computed(() =>
  props.job?.total ? Math.round((props.job.completed / props.job.total) * 100) : 0,
);
const progressStatus = computed(() => {
  switch (props.job?.status) {
    case 'succeeded':
      return 'success';
    case 'failed':
      return 'error';
    case 'cancelled':
      return 'warning';
    default:
      return 'default';
  }
});

async function cancel() {
  try {
    if (props.job) {
      await api.cancel(props.job.id);
    }
  } catch (e) {
    emit('error', errorMessage(e));
  }
}
</script>

<template>
  <div v-if="job" class="job-progress">
    <div class="section-line">
      <span>{{ localize(job.stage) || t('Processing') }}</span>
      <n-button v-if="job.status === 'running' && job.cancellable" size="small" @click="cancel">{{ t('Cancel task') }}</n-button>
    </div>
    <n-progress
      type="line"
      :percentage="percentage"
      :status="progressStatus"
      :processing="job.status === 'running'"
      :show-indicator="job.total > 0"
    />
    <small>{{ job.completed }} / {{ job.total || '—' }}</small>
    <n-alert v-if="job.status === 'failed'" type="error">{{ errorMessage(job.errorJson) }}</n-alert>
  </div>
</template>
