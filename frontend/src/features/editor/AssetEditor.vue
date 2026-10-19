<script setup lang="ts">
import { localize, t } from '../../i18n';
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import {
  NModal,
  NCard,
  NButton,
  NInput,
  NSelect,
  NCheckbox,
  NForm,
  NFormItem,
  NAlert,
  NImage,
  NDivider,
  NSpace,
  useDialog,
  useMessage,
} from 'naive-ui';
import type {
  Asset,
  AssetDraft,
  Bootstrap,
  BoothDraft,
  JobSnapshot,
  MediaDraft,
} from '../../api/types';
import { api, sourceErrorMessage } from '../../api/client';
import { waitJob } from '../../api/jobs';
import { createDraft, validateDraft, applyBoothResult, addDraftTag } from './draft';
import JobProgress from '../../shared/JobProgress.vue';
import CoverPlaceholderIcon from '../../shared/CoverPlaceholderIcon.vue';

const props = withDefaults(defineProps<{ asset: Asset | null; data: Bootstrap; show?: boolean }>(), { show: true });
const emit = defineEmits<{ close: []; afterLeave: []; saved: [asset: Asset]; openExisting: [id: string] }>();
// 首次挂载后再显示，确保入场与后续关闭都走组件库过渡。
const visible = ref(false);
watch(() => props.show, (show) => {
  visible.value = show;
});
const message = useMessage();
const dialog = useDialog();

// 编辑器持有独立草稿，只有保存成功才发布到资料库。
const draft = ref<AssetDraft>(
  props.asset
    ? {
        ...JSON.parse(JSON.stringify(props.asset)),
        avatarIds: props.asset.avatarIds || [],
        freeTagIds: props.asset.freeTagIds || [],
        newTags: props.asset.newTags || [],
      }
    : createDraft(),
);
let boothPolling = new AbortController();
let disposed = false;
const initial = JSON.stringify(draft.value);
const validationStarted = ref(false);
const errors = computed(() => validationStarted.value ? validateDraft(draft.value) : {});
const editorFields = ref<HTMLElement | null>(null);
const saving = ref(false);
const error = ref('');

// 暂存封面在关闭时清理，已经提交的 token 会从集合中移除。
const cover = ref(props.asset?.coverUrl || '');
const staging = ref(false);
const stagedTokens = new Set<string>();

// Booth 获取与回填选项相互独立，获取成功不会直接覆盖草稿。
const boothOpen = ref(false);
const boothInput = ref('');
const boothBusy = ref(false);
const boothResult = ref<BoothDraft | null>(null);
const boothError = ref('');
const boothJob = ref<JobSnapshot | null>(null);
const replaceID = ref(false);
const replaceName = ref(false);
const replaceCover = ref(false);
const requestEpoch = ref(0);
const duplicateId = ref('');

const newTagName = ref('');
const newTagKind = ref<'avatar' | 'free'>('avatar');
const newAliases = ref('');

const categoryOptions = computed(() => [
  { label: t('Uncategorized'), value: '' },
  ...props.data.categories.map((tag) => ({ label: tag.name, value: tag.id })),
]);

function tagOptions(kind: 'avatar' | 'free') {
  return [
    ...(kind === 'avatar' ? props.data.avatarTags : props.data.freeTags).map((tag) => ({
      label: tag.name,
      value: tag.id,
    })),
    ...draft.value.newTags
      .filter((tag) => tag.kind === kind)
      .map((tag) => ({ label: t('{p0} · New', { p0: tag.name }), value: tag.key })),
  ];
}

function close() {
  if (saving.value || boothOpen.value) {
    return;
  }

  if (JSON.stringify(draft.value) !== initial) {
    dialog.warning({
      title: t('Discard unsaved changes?'),
      content: t('Changes have not been saved.'),
      positiveText: t('Discard changes'),
      negativeText: t('Continue editing'),
      onPositiveClick: () => emit('close'),
    });
  } else {
    emit('close');
  }
}

function addTag() {
  try {
    draft.value = addDraftTag(
      draft.value,
      [...props.data.avatarTags, ...props.data.freeTags],
      newTagKind.value,
      newTagName.value,
      newAliases.value.split(/[,，;；\n]/),
    );
    newTagName.value = '';
    newAliases.value = '';
  } catch (e) {
    error.value = sourceErrorMessage(e);
  }
}

function removeDraftTag(key: string) {
  draft.value.newTags = draft.value.newTags.filter((tag) => tag.key !== key);
  draft.value.avatarIds = draft.value.avatarIds.filter((id) => id !== key);
  draft.value.freeTagIds = draft.value.freeTagIds.filter((id) => id !== key);
}

async function acceptMedia(media: MediaDraft) {
  if (!media?.token) {
    return;
  }

  if (disposed) {
    void api.discardCover(media.token).catch(() => {});
    return;
  }

  stagedTokens.add(media.token);
  draft.value.coverToken = media.token;
  draft.value.coverId = '';
  cover.value = media.previewUrl;
}

async function chooseCover() {
  try {
    staging.value = true;
    await acceptMedia(await api.selectCover());
  } catch (e) {
    error.value = sourceErrorMessage(e);
  } finally {
    staging.value = false;
  }
}

async function stageFile(file: File) {
  if (!file.type.startsWith('image/')) {
    message.warning(t('Select an image file'));
    return;
  }

  if (file.size > 20 * 1024 * 1024) {
    message.warning(t('Images must not exceed 20 MiB'));
    return;
  }

  staging.value = true;

  try {
    const bytes = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result));
      reader.onerror = () => reject(new Error(t('Failed to read image')));
      reader.readAsDataURL(file);
    });
    await acceptMedia(await api.stageBytes(bytes));
  } catch (e) {
    error.value = sourceErrorMessage(e);
  } finally {
    staging.value = false;
  }
}

function onDrop(event: DragEvent) {
  event.preventDefault();
  const file = event.dataTransfer?.files[0];
  if (file) {
    void stageFile(file);
  }
}

function paste(event: ClipboardEvent) {
  if (!props.show || boothOpen.value) {
    return;
  }

  const file = Array.from(event.clipboardData?.items || [])
    .find((x) => x.type.startsWith('image/'))
    ?.getAsFile();
  if (file) {
    event.preventDefault();
    void stageFile(file);
  }
}

async function folder() {
  try {
    const path = await api.selectFolder();
    if (path) {
      draft.value.folderPath = path;
    }
  } catch (e) {
    error.value = sourceErrorMessage(e);
  }
}

async function save() {
  validationStarted.value = true;
  if (Object.keys(errors.value).length) {
    await nextTick();
    const firstInvalid = editorFields.value?.querySelector<HTMLInputElement>('input[aria-invalid="true"]');
    firstInvalid?.scrollIntoView({ block: 'center', behavior: 'auto' });
    firstInvalid?.focus({ preventScroll: true });
    return;
  }

  saving.value = true;
  error.value = '';

  try {
    const saved = await api.save({
      ...draft.value,
      newTags: draft.value.newTags.filter((tag) =>
        [...draft.value.avatarIds, ...draft.value.freeTagIds].includes(tag.key),
      ),
    });
    stagedTokens.delete(draft.value.coverToken);
    message.success(t('Asset saved'));
    emit('saved', saved);
  } catch (e) {
    error.value = sourceErrorMessage(e);

    try {
      const parsed = JSON.parse(e instanceof Error ? e.message : String(e));
      duplicateId.value = parsed.code === 'DUPLICATE_BOOTH' ? parsed.existingAssetId || '' : '';
    } catch {}
  } finally {
    saving.value = false;
  }
}

function openBooth() {
  boothInput.value = draft.value.boothId;
  boothResult.value = null;
  boothError.value = '';
  boothOpen.value = true;
}

async function recognize() {
  if (boothBusy.value || boothResult.value) {
    return;
  }

  const input = boothInput.value.trim();
  if (
    !/^\d+$/.test(input) &&
    !/^https:\/\/(?:[\w-]+\.)?booth\.pm\/(?:[\w-]+\/)?items\/\d+(?:[/?#].*)?$/.test(input)
  ) {
    boothError.value = 'Enter a numeric ID or a valid Booth product URL';
    return;
  }

  const epoch = ++requestEpoch.value;
  boothBusy.value = true;
  boothError.value = '';
  boothResult.value = null;
  boothJob.value = null;
  boothPolling.abort();
  boothPolling = new AbortController();

  try {
    const ref = await api.startBooth(input, draft.value.id);
    if (epoch !== requestEpoch.value) {
      await api.cancel(ref.id);
      return;
    }

    const result = await waitJob<BoothDraft>(
      ref.id,
      (s) => {
        if (epoch === requestEpoch.value) {
          boothJob.value = s;
        }
      },
      boothPolling.signal,
    );
    if (epoch !== requestEpoch.value) {
      if (result.cover) {
        void api.discardCover(result.cover.token).catch(() => {});
      }

      return;
    }

    boothResult.value = result;
    if (result.cover) {
      stagedTokens.add(result.cover.token);
    }

    replaceID.value = !draft.value.boothId || draft.value.boothId === result.boothId;
    replaceName.value = !draft.value.name || draft.value.name === result.name;
    replaceCover.value = !draft.value.coverId && !draft.value.coverToken;
  } catch (e) {
    if (epoch === requestEpoch.value) {
      boothError.value = sourceErrorMessage(e);

      try {
        const p = JSON.parse(e instanceof Error ? e.message : String(e));
        duplicateId.value = p.existingAssetId || p.details?.existingAssetId || '';
      } catch {}
    }
  } finally {
    if (epoch === requestEpoch.value) {
      boothBusy.value = false;
    }
  }
}

function cancelBooth() {
  requestEpoch.value++;
  boothPolling.abort();
  if (boothJob.value?.status === 'running') {
    void api.cancel(boothJob.value.id).catch(() => {});
  }

  boothBusy.value = false;
  boothOpen.value = false;
}

function fillBooth() {
  const r = boothResult.value;
  if (!r) {
    return;
  }

  draft.value = applyBoothResult(draft.value, r, {
    replaceID: replaceID.value,
    replaceName: replaceName.value,
    replaceCover: replaceCover.value,
    avatarIds: [],
  });
  if (replaceCover.value && r.cover) {
    cover.value = r.cover.previewUrl;
  }

  boothOpen.value = false;
  message.success(t('Details applied. Review and save.'));
}

function keydown(event: KeyboardEvent) {
  if (!props.show) {
    return;
  }

  if (event.key === 'Escape' && !event.defaultPrevented) {
    event.preventDefault();
    if (boothOpen.value) {
      cancelBooth();
    } else {
      close();
    }
  }
}

onMounted(() => {
  visible.value = props.show;
  document.addEventListener('paste', paste);
  document.addEventListener('keydown', keydown);
});

onBeforeUnmount(() => {
  disposed = true;
  document.removeEventListener('paste', paste);
  document.removeEventListener('keydown', keydown);
  requestEpoch.value++;
  boothPolling.abort();
  if (boothJob.value?.status === 'running') {
    void api.cancel(boothJob.value.id).catch(() => {});
  }

  for (const token of stagedTokens) {
    void api.discardCover(token).catch(() => {});
  }
});
</script>

<template>
  <n-modal
    :show="visible"
    :mask-closable="!saving && !boothOpen"
    :close-on-esc="false"
    @update:show="(show) => { if (!show) close(); }"
    @after-leave="emit('afterLeave')"
  >
    <n-card class="editor-modal" :title="asset ? t('Edit asset') : t('Add asset')" closable @close="close">
      <n-alert v-if="error" type="error" closable @close="error = ''">
        {{ localize(error) }}
        <n-button v-if="duplicateId" text @click="emit('openExisting', duplicateId)">{{ t('Open existing record') }}</n-button>
      </n-alert>
      <div ref="editorFields" class="editor-grid">
        <div>
          <div class="cover-drop" @dragover.prevent @drop="onDrop">
            <n-image v-if="cover" :src="cover" object-fit="contain" width="100%" />
            <div v-else class="cover-placeholder">
              <CoverPlaceholderIcon />
              {{ t('Add cover') }}
            </div>
            <p>{{ t('Drop an image here, or paste with Ctrl+V') }}</p>
            <n-space justify="center">
              <n-button :loading="staging" @click="chooseCover">{{ t('Choose image') }}</n-button>
              <n-button
                v-if="cover"
                quaternary
                @click="draft.coverId = '';
                  draft.coverToken = '';
                  cover = '';
                "
              >{{ t('Remove') }}</n-button>
            </n-space>
          </div>
          <p class="subtle" style="text-align: center">{{ t('JPG, PNG or WebP, up to 20 MiB.') }}</p>
        </div>
        <n-form label-placement="top">
          <n-form-item
            :label="t('Asset name *')"
            :feedback="errors.name"
            :validation-status="errors.name ? 'error' : undefined"
          >
            <n-input
              v-model:value="draft.name"
              :status="errors.name ? 'error' : undefined"
              :input-props="{ 'aria-invalid': !!errors.name, 'aria-required': true }"
              :placeholder="t('Enter asset name')"
            />
          </n-form-item>
          <div class="form-two">
            <n-form-item :label="t('Category')">
              <n-select v-model:value="draft.categoryId" :options="categoryOptions" />
            </n-form-item>
            <n-form-item
              :label="t('Booth ID')"
              :feedback="errors.boothId"
              :validation-status="errors.boothId ? 'error' : undefined"
            >
              <n-input
                v-model:value="draft.boothId"
                :status="errors.boothId ? 'error' : undefined"
                :input-props="{ 'aria-invalid': !!errors.boothId }"
                :placeholder="t('Numeric ID; optional')"
              />
            </n-form-item>
          </div>
          <n-button secondary type="primary" @click="openBooth">{{ t('✦ Booth lookup') }}</n-button>
          <n-divider />
          <n-form-item :label="t('Compatible avatars')">
            <n-select
              v-model:value="draft.avatarIds"
              multiple
              filterable
              :options="tagOptions('avatar')"
              :placeholder="t('Search or select avatars')"
            />
          </n-form-item>
          <n-checkbox v-model:checked="draft.universal">{{ t('Compatible with all avatars') }}</n-checkbox>
          <n-form-item :label="t('Custom tags')" style="margin-top: 18px">
            <n-select
              v-model:value="draft.freeTagIds"
              multiple
              filterable
              :options="tagOptions('free')"
              :placeholder="t('For example: Purchased, To organize, Winter clothing')"
            />
          </n-form-item>
          <details class="new-tag">
            <summary>{{ t('＋ New tag') }}</summary>
            <div class="form-two">
              <n-select
                v-model:value="newTagKind"
                :options="[
                  { label: t('Avatar tags'), value: 'avatar' },
                  { label: t('Custom tags'), value: 'free' },
                ]"
              />
              <n-input
                v-model:value="newTagName"
                :placeholder="t('New tag name')"
                @keydown.enter.prevent="addTag"
              />
            </div>
            <n-input
              v-if="newTagKind === 'avatar'"
              v-model:value="newAliases"
              :placeholder="t('Aliases (comma-separated, optional)')"
            />
            <n-button size="small" @click="addTag">{{ t('Add tag to draft') }}</n-button>
            <small>{{ t('New tags are created when the asset is saved.') }}</small>
            <div v-if="draft.newTags.length" class="draft-tag-list">
              <div v-for="tag in draft.newTags" :key="tag.key" class="draft-tag-row">
                <span>{{ tag.kind === 'avatar' ? t('Avatar') : t('Tags') }}</span>
                <n-input v-model:value="tag.name" :placeholder="t('New tag name')" size="small" />
                <n-input
                  v-if="tag.kind === 'avatar'"
                  :value="tag.aliases.join(', ')"
                  :placeholder="t('Aliases (comma-separated)')"
                  size="small"
                  @update:value="(v) =>
                      (tag.aliases = v
                        .split(/[,，;；]/)
                        .map((s) => s.trim())
                        .filter(Boolean))
                  "
                />
                <n-button size="small" quaternary type="error" @click="removeDraftTag(tag.key)">{{ t('Remove') }}</n-button>
              </div>
            </div>
          </details>
          <n-form-item :label="t('Notes')">
            <n-input
              v-model:value="draft.notes"
              type="textarea"
              :autosize="{ minRows: 3, maxRows: 6 }"
              :placeholder="t('Enter notes')"
            />
          </n-form-item>
          <n-form-item :label="t('Link asset folder')">
            <n-input v-model:value="draft.folderPath" clearable :placeholder="t('Select asset folder')" />
            <n-button style="margin-left: 8px" @click="folder">{{ t('Browse') }}</n-button>
          </n-form-item>
          <n-checkbox v-if="draft.folderPath" v-model:checked="draft.allowOfflineFolder">{{ t('Allow currently unavailable folder paths') }}</n-checkbox>
        </n-form>
      </div>
      <template #footer>
        <div class="modal-actions">
          <n-space style="margin-left: auto">
            <n-button @click="close">{{ t('Cancel') }}</n-button>
            <n-button
              type="primary"
              :loading="saving"
              :disabled="staging || boothBusy"
              @click="save"
            >{{ t('Save asset') }}</n-button>
          </n-space>
        </div>
      </template>
    </n-card>
  </n-modal>
  <n-modal
    :show="boothOpen"
    :mask-closable="true"
    :close-on-esc="false"
    @update:show="(show) => { if (!show) cancelBooth(); }"
  >
    <n-card class="booth-modal" :title="t('Booth lookup')" closable @close="cancelBooth">
      <p class="subtle">{{ t('Fetch the product name and cover, then review before applying.') }}</p>
      <n-input
        v-model:value="boothInput"
        :placeholder="t('Booth ID or product URL')"
        :disabled="boothBusy || !!boothResult"
        @keydown.enter="recognize"
      />
      <n-alert v-if="boothError" type="error" style="margin-top: 14px">
        {{ localize(boothError) }}
        <n-button v-if="duplicateId" text @click="emit('openExisting', duplicateId)">{{ t('Open existing record') }}</n-button>
      </n-alert>
      <JobProgress v-if="boothBusy" :job="boothJob" @error="boothError = $event" />
      <div v-if="boothResult" class="booth-result">
        <div class="booth-result-heading">
          <n-image v-if="boothResult.cover" :src="boothResult.cover.previewUrl" width="110" />
          <div>
            <h3>{{ boothResult.name }}</h3>
            <p>#{{ boothResult.boothId }}</p>
          </div>
        </div>
        <n-space vertical>
          <n-checkbox v-model:checked="replaceID">
            {{ t(
                '{p0} Booth ID: {p1}',
                {
                  p0: draft.boothId && draft.boothId !== boothResult.boothId ? t('Replace current') : t('Fill in'),
                  p1: boothResult.boothId,
                },
              ) }}
          </n-checkbox>
          <n-checkbox v-model:checked="replaceName">
            {{ draft.name && draft.name !== boothResult.name ? t('Replace existing name') : t('Use product name')
            }}{{ draft.name && draft.name !== boothResult.name ? `「${draft.name}」` : '' }}
          </n-checkbox>
          <n-checkbox v-if="boothResult.cover" v-model:checked="replaceCover">
            {{ cover ? t('Replace current cover') : t('Use product cover') }}
          </n-checkbox>
        </n-space>
      </div>
      <template #footer>
        <n-space justify="end">
          <n-button @click="cancelBooth">{{ boothBusy ? t('Cancel lookup') : t('Back to editor') }}</n-button>
          <n-button v-if="!boothResult" type="primary" :loading="boothBusy" @click="recognize">{{ t('Fetch details') }}</n-button>
          <n-button v-else type="primary" @click="fillBooth">{{ t('Apply') }}</n-button>
        </n-space>
      </template>
    </n-card>
  </n-modal>
</template>
