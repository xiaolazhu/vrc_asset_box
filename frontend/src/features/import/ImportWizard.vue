<script setup lang="ts">
import { confirmAction } from '../../shared/confirm-action';
import { localize, t } from '../../i18n';
import { computed, onBeforeUnmount, ref } from 'vue';
import {
  NSteps,
  NStep,
  NButton,
  NAlert,
  NInput,
  NSelect,
  NCheckbox,
  NTag,
  NImage,
  NSpace,
  NModal,
  NCard,
  NPagination,
  NForm,
  NFormItem,
  useDialog,
  useMessage,
} from 'naive-ui';
import type {
  AssetDraft,
  Asset,
  ImportPreview,
  ImportRow,
  ImportResult,
  Bootstrap,
  JobSnapshot,
  SheetMapping,
} from '../../api/types';
import { api, sourceErrorMessage } from '../../api/client';
import { waitJob } from '../../api/jobs';
import {
  resolveMerge,
  renameImportTag,
  importTarget,
  invalidateChangedMergeChoices,
  type MergeField,
} from './merge';
import JobProgress from '../../shared/JobProgress.vue';

const props = defineProps<{ data: Bootstrap }>();
const emit = defineEmits<{ changed: []; back: [] }>();
const message = useMessage();
const dialog = useDialog();

// 工作簿预览、逐行决定与提交结果属于同一次导入会话。
const step = ref(1);
const path = ref('');
const busy = ref(false);
const error = ref('');
const job = ref<JobSnapshot | null>(null);
const preview = ref<ImportPreview | null>(null);
const actions = ref<Record<string, string>>({});
const result = ref<ImportResult | null>(null);
const page = ref(1);
const onlyIssues = ref(false);
const editRow = ref<ImportRow | null>(null);
const editVisible = ref(false);
const existing = ref<Record<string, AssetDraft>>({});
const choices = ref<Record<string, Partial<Record<MergeField, 'existing' | 'incoming'>>>>({});
const loadingExisting = ref(false);

let polling = new AbortController();
let disposed = false;

// 合并目标缓存与字段显示配置。
const databaseTargets: Record<string, AssetDraft> = {};
const fieldNames = computed<Record<MergeField, string>>(() => ({
  name: t('Name'),
  categoryId: t('Category'),
  notes: t('Notes'),
  folderPath: t('Linked folder'),
  coverId: t('Cover'),
}));
const columnFields = computed<{ key: keyof SheetMapping; label: string }[]>(() => [
  { key: 'nameColumn', label: t('Name *') },
  { key: 'boothColumn', label: 'Booth ID' },
  { key: 'imageColumn', label: t('Image') },
  { key: 'avatarColumn', label: t('Avatar') },
  { key: 'notesColumn', label: t('Notes') },
  { key: 'freeTagsColumn', label: t('Custom tags') },
  { key: 'folderColumn', label: t('Folder') },
]);

const categoryOptions = computed(() => [
  { label: t('Uncategorized'), value: '' },
  ...props.data.categories.map((c) => ({ label: c.name, value: c.id })),
]);
const filteredRows = computed(() =>
  (preview.value?.rows || []).filter(
    (r) => !onlyIssues.value || r.issues?.length || r.existingAssetId || r.existingRowKey,
  ),
);
const pageRows = computed(() => filteredRows.value.slice((page.value - 1) * 20, page.value * 20));
const summary = computed(() => {
  const counts = { create: 0, merge: 0, skip: 0 };
  for (const r of preview.value?.rows || []) {
    counts[(actions.value[r.key] || 'skip') as keyof typeof counts]++;
  }

  return counts;
});
const currentConflicts = computed(() => {
  const r = editRow.value;
  const old = r ? existing.value[r.key] : null;
  if (!r || !old) {
    return [];
  }

  return resolveMerge(old, r.draft, choices.value[r.key] || {}).conflicts;
});

function normalizeRow(row: ImportRow): ImportRow {
  return {
    ...row,
    issues: row.issues || [],
    imageOptions: row.imageOptions || [],
    draft: {
      ...row.draft,
      avatarIds: row.draft.avatarIds || [],
      freeTagIds: row.draft.freeTagIds || [],
      newTags: row.draft.newTags || [],
    },
  };
}

async function chooseFile() {
  try {
    const selected = await api.selectWorkbook();
    if (selected) {
      path.value = selected;
      await readPreview([]);
    }
  } catch (e) {
    error.value = sourceErrorMessage(e);
  }
}

async function readPreview(mappings: SheetMapping[]) {
  busy.value = true;
  error.value = '';
  result.value = null;
  polling.abort();
  polling = new AbortController();
  const oldId = preview.value?.previewId;

  try {
    const ref = await api.previewImport(
      path.value,
      mappings.map((s) => ({ ...s, categoryId: s.newCategoryName.trim() ? '' : s.categoryId })),
    );
    if (disposed) {
      await api.cancel(ref.id);
      return;
    }

    const data = await waitJob<ImportPreview>(ref.id, (s) => (job.value = s), polling.signal);
    if (disposed) {
      await api.discardImport(data.previewId);
      return;
    }

    preview.value = { ...data, rows: (data.rows || []).map(normalizeRow) };
    if (oldId) {
      void api.discardImport(oldId).catch(() => {});
    }

    actions.value = {};
    choices.value = {};
    existing.value = {};
    for (const key of Object.keys(databaseTargets)) {
      delete databaseTargets[key];
    }

    page.value = 1;
    for (const row of preview.value.rows) {
      actions.value[row.key] =
        row.existingAssetId || row.existingRowKey || row.issues?.length ? 'skip' : 'create';
    }

    step.value = mappings.length ? 3 : 2;
  } catch (e) {
    if (!disposed) {
      error.value = sourceErrorMessage(e);
    }
  } finally {
    if (!disposed) {
      busy.value = false;
    }
  }
}

function actionOptions(r: ImportRow) {
  return r.existingAssetId || r.existingRowKey
    ? [
        { label: t('Skip'), value: 'skip' },
        { label: t('Merge into existing record'), value: 'merge' },
      ]
    : [
        { label: t('Add'), value: 'create' },
        { label: t('Skip'), value: 'skip' },
      ];
}

async function target(r: ImportRow): Promise<AssetDraft> {
  const destination = importTarget(r);
  if (destination?.kind === 'row') {
    const row = preview.value?.rows.find((x) => x.key === destination.id);
    if (!row || actions.value[row.key] === 'skip') {
      throw new Error(t('Set the target row in this file to Add or Merge first'));
    }

    return row.draft;
  }

  return (databaseTargets[r.existingAssetId] ||= await api.get(r.existingAssetId));
}

async function edit(r: ImportRow) {
  editRow.value = r;
  editVisible.value = true;
  error.value = '';
  if ((r.existingAssetId || r.existingRowKey) && !existing.value[r.key]) {
    loadingExisting.value = true;

    try {
      existing.value[r.key] = JSON.parse(JSON.stringify(await target(r)));
    } catch (e) {
      error.value = `Failed to read merge target: ${sourceErrorMessage(e)}`;
    } finally {
      loadingExisting.value = false;
    }
  }
}

function closeEdit() {
  if (!busy.value) {
    editVisible.value = false;
  }
}

function clearClosedEdit() {
  // 保留内容直到退场动画结束，确保 Modal 正常解除滚动锁。
  if (!editVisible.value) {
    editRow.value = null;
  }
}

function acceptNewTags() {
  for (const row of preview.value?.rows || []) {
    if (
      !row.existingAssetId &&
      !row.existingRowKey &&
      row.draft.name.trim() &&
      row.issues.every((s) => s.startsWith('New tag needs confirmation: '))
    ) {
      actions.value[row.key] = 'create';
    }
  }

  message.success(t('New tags confirmed. Other issues and duplicates retain their previous choices.'));
}

function renameTag(row: ImportRow, key: string, name: string) {
  renameImportTag(preview.value?.rows || [row], key, name);
}

function tagOptions(row: ImportRow, kind: 'avatar' | 'free') {
  return [
    ...(kind === 'avatar' ? props.data.avatarTags : props.data.freeTags).map((tag) => ({
      label: tag.name,
      value: tag.id,
    })),
    ...(row.draft.newTags || [])
      .filter((tag) => tag.kind === kind)
      .map((tag) => ({ label: t('{p0} · Import new tag', { p0: tag.name }), value: tag.key })),
  ];
}

function fieldValue(d: AssetDraft, field: MergeField) {
  if (field === 'categoryId') {
    return props.data.categories.find((c) => c.id === d.categoryId)?.name || t('Uncategorized');
  }

  if (field === 'coverId') {
    return d.coverId || d.coverToken ? t('Existing image') : t('No image');
  }

  return d[field] || t('(empty)');
}

function selectCover(row: ImportRow, index: number) {
  row.draft.coverId = '';
  row.draft.coverToken = row.imageOptions[index].token;
}

function conflictFields(row: ImportRow) {
  const old = existing.value[row.key];
  if (!old) {
    return [];
  }

  return resolveMerge(old, row.draft, {}).conflicts;
}

async function confirmImport() {
  if (!preview.value) {
    return;
  }

  error.value = '';
  busy.value = true;
  const resolved: ImportRow[] = [];
  const resolvedTargets = new Map<string, AssetDraft>();

  try {
    for (const row of preview.value.rows) {
      const action = actions.value[row.key];
      if (action === 'skip') {
        resolved.push(row);
        continue;
      }

      if (!row.draft.name.trim()) {
        throw new Error(t('{p0}!{p1}: Name cannot be empty. Edit or skip this row.', { p0: row.sheet, p1: row.row }));
      }

      const draft = {
        ...row.draft,
        newTags: row.draft.newTags.filter((tag) =>
          [...row.draft.avatarIds, ...row.draft.freeTagIds].includes(tag.key),
        ),
      };
      if (action === 'merge') {
        const destination = importTarget(row);
        const old =
          destination?.kind === 'row'
            ? resolvedTargets.get(destination.id)
            : resolvedTargets.get('asset:' + row.existingAssetId) || (await target(row));
        if (!old) {
          throw new Error(t('{p0}!{p1}: Enable the target row being merged into in this file first', { p0: row.sheet, p1: row.row }));
        }

        const checked = invalidateChangedMergeChoices(
          existing.value[row.key],
          old,
          choices.value[row.key] || {},
        );
        existing.value[row.key] = JSON.parse(JSON.stringify(old));
        choices.value[row.key] = checked.choices;
        if (checked.invalidated.length) {
          editRow.value = row;
          editVisible.value = true;
          throw new Error(
            t(
              '{p0}!{p1}: An earlier merge changed the target fields ({p2}). Confirm again using the values now shown.',
              {
                p0: row.sheet,
                p1: row.row,
                p2: checked.invalidated.map((f) => fieldNames.value[f]).join('、'),
              },
            ),
          );
        }

        const merged = resolveMerge(old, draft, choices.value[row.key] || {});
        if (merged.conflicts.length) {
          throw new Error(
            t(
              '{p0}!{p1}: Review merge conflicts first ({p2})',
              {
                p0: row.sheet,
                p1: row.row,
                p2: merged.conflicts.map((f) => fieldNames.value[f]).join('、'),
              },
            ),
          );
        }

        resolved.push({ ...row, draft: merged.draft });
        resolvedTargets.set(row.key, merged.draft);
        if (destination?.kind === 'row') {
          resolvedTargets.set(destination.id, merged.draft);
        } else {
          resolvedTargets.set('asset:' + row.existingAssetId, merged.draft);
        }
      } else {
        resolved.push({ ...row, draft });
        resolvedTargets.set(row.key, draft);
      }
    }

    busy.value = false;
    confirmAction(dialog, {
      title: t('Import selected assets?'),
      content: t(
        'Add: {p0}; merge: {p1}; skip: {p2}. The library will be backed up before import. The source Excel file will not be modified.',
        {
          p0: summary.value.create,
          p1: summary.value.merge,
          p2: summary.value.skip,
        },
      ),
      positiveText: t('Confirm import'),
      negativeText: t('Back to preview'),
      onPositiveClick: async () => {
        if (!preview.value) {
          return;
        }

        busy.value = true;
        error.value = '';
        polling.abort();
        polling = new AbortController();

        try {
          const ref = await api.commitImport(preview.value.previewId, resolved, actions.value);
          if (disposed) {
            return;
          }

          result.value = await waitJob<ImportResult>(
            ref.id,
            (s) => (job.value = s),
            polling.signal,
          );
          if (disposed) {
            return;
          }

          step.value = 4;
          emit('changed');
          message.success(t('Excel import complete'));
        } catch (e) {
          if (!disposed) {
            error.value = sourceErrorMessage(e);
          }
        } finally {
          if (!disposed) {
            busy.value = false;
          }
        }
      },
    });
  } catch (e) {
    error.value = sourceErrorMessage(e);
    busy.value = false;
  }
}

async function report() {
  if (!preview.value) {
    return;
  }

  try {
    const saved = await api.report(preview.value.previewId);
    if (saved) {
      message.success(t('Report saved: {p0}', { p0: saved }));
    }
  } catch (e) {
    error.value = sourceErrorMessage(e);
  }
}

function restart() {
  if (preview.value) {
    void api.discardImport(preview.value.previewId).catch(() => {});
  }

  step.value = 1;
  preview.value = null;
  result.value = null;
  job.value = null;
  error.value = '';
}

onBeforeUnmount(() => {
  disposed = true;
  polling.abort();
  if (job.value?.status === 'running' && job.value.cancellable) {
    void api.cancel(job.value.id).catch(() => {});
  }

  if (preview.value) {
    void api.discardImport(preview.value.previewId).catch(() => {});
  }
});
</script>

<template>
  <div class="import-page">
    <header class="page-heading">
      <div>
        <h1>{{ t('Import from Excel') }}</h1>
        <p class="subtle">{{ t('Map fields and review before importing. The source file will not be modified.') }}</p>
      </div>
      <n-button @click="emit('back')">{{ t('Back to settings') }}</n-button>
    </header>
    <n-steps :current="step" size="small">
      <n-step :title="t('Choose workbook')" />
      <n-step :title="t('Confirm field mapping')" />
      <n-step :title="t('Preview & review')" />
      <n-step :title="t('Import results')" />
    </n-steps>
    <n-alert v-if="error" type="error" closable style="margin-top: 24px" @close="error = ''">
      {{ localize(error) }}
    </n-alert>
    <JobProgress v-if="busy" :job="job" @error="error = $event" />
    <n-card v-if="step === 1" :bordered="false" class="workbook-picker">
      <div class="workbook-symbol">
        <svg
          viewBox="0 0 48 48"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
          focusable="false"
        >
          <path d="M28 5H12a3 3 0 0 0-3 3v32a3 3 0 0 0 3 3h24a3 3 0 0 0 3-3V16Z" />
          <path d="M28 5v8a3 3 0 0 0 3 3h8" />
          <rect x="16" y="23" width="16" height="12" rx="1.5" />
          <path d="M16 27h16M16 31h16M22 23v12" />
        </svg>
      </div>
      <h2>{{ t('Select Excel file') }}</h2>
      <p>{{ t('Supports .xlsx files and embedded images that can be read.') }}</p>
      <p class="subtle">{{ t('Map sheets to categories. Review duplicates and issues in the preview.') }}</p>
      <n-button type="primary" size="large" :loading="busy" @click="chooseFile">{{ t('Choose Excel workbook') }}</n-button>
      <p v-if="path" class="path-text">{{ path }}</p>
    </n-card>
    <template v-if="preview && step === 2">
      <div class="section-line" style="margin: 28px 0 12px">
        <h3>{{ t('Sheets & field mapping') }}</h3>
        <n-button :disabled="busy" @click="chooseFile">{{ t('Choose another file') }}</n-button>
      </div>
      <p class="subtle path-text">{{ path }}</p>
      <n-alert type="info" :bordered="false">{{ t('Use column letters (A, B, C). Leave blank to skip a field. Omitting the avatar column does not imply universal compatibility.') }}</n-alert>
      <div v-for="sheet in preview.sheets" :key="sheet.sheet" class="sheet-mapping">
        <div class="sheet-heading">
          <n-checkbox v-model:checked="sheet.included">
            <strong>{{ sheet.sheet }}</strong>
          </n-checkbox>
          <div class="sheet-category">
            <span>{{ t('Category') }}</span>
            <n-select
              v-model:value="sheet.categoryId"
              :options="categoryOptions"
              :placeholder="t('Select existing category')"
            />
            <n-input v-model:value="sheet.newCategoryName" :placeholder="t('Or create a category (takes priority)')" />
          </div>
        </div>
        <div class="mapping-columns">
          <label v-for="field in columnFields" :key="field.key">
            <span>{{ field.label }}</span>
            <n-input
              :value="String(sheet[field.key] || '')"
              :disabled="!sheet.included"
              :placeholder="t('Column')"
              @update:value="(v) => ((sheet as unknown as Record<string, unknown>)[field.key] = v.toUpperCase())
              "
            />
          </label>
        </div>
      </div>
      <div class="wizard-actions">
        <n-button :disabled="busy" @click="restart">{{ t('Back') }}</n-button>
        <n-button
          type="primary"
          :loading="busy"
          :disabled="!preview.sheets.some((s) => s.included)"
          @click="readPreview(preview.sheets)"
        >{{ t('Generate preview') }}</n-button>
      </div>
    </template>
    <template v-if="preview && step === 3">
      <div class="preview-summary">
        <div>
          <strong>{{ preview.rows.length }}</strong>
          <span>{{ t('Source records') }}</span>
        </div>
        <div>
          <strong>{{ summary.create }}</strong>
          <span>{{ t('To add') }}</span>
        </div>
        <div>
          <strong>{{ summary.merge }}</strong>
          <span>{{ t('To merge') }}</span>
        </div>
        <div>
          <strong>{{ summary.skip }}</strong>
          <span>{{ t('Skip') }}</span>
        </div>
      </div>
      <div class="section-line">
        <n-space>
          <n-checkbox v-model:checked="onlyIssues" @update:checked="page = 1">{{ t('Show only issues and duplicates') }}</n-checkbox>
          <n-button size="small" secondary @click="acceptNewTags">{{ t('Confirm tags and add valid records') }}</n-button>
        </n-space>
        <n-button size="small" :disabled="busy" @click="step = 2">{{ t('Adjust field mapping') }}</n-button>
      </div>
      <div class="import-table-wrap">
        <table class="import-table">
          <thead>
            <tr>
              <th>{{ t('Source') }}</th>
              <th>{{ t('Cover') }}</th>
              <th>{{ t('Asset name / Booth ID') }}</th>
              <th>{{ t('Avatar / Issues') }}</th>
              <th>{{ t('Action') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in pageRows" :key="row.key">
              <td>
                {{ row.sheet }}
                <small>{{ t('Row {p0}', { p0: row.row }) }}</small>
              </td>
              <td>
                <img
                  v-if="row.imageOptions.length"
                  :src="row.imageOptions.find((i) => i.token === row.draft.coverToken)?.previewUrl ||
                    row.imageOptions[0].previewUrl
                  "
                  loading="lazy"
                  :alt="t('Imported cover')"
                />
                <span v-else class="subtle">{{ t('No image') }}</span>
              </td>
              <td>
                <strong>{{ row.draft.name || t('(missing name)') }}</strong>
                <small>{{ row.draft.boothId ? `#${row.draft.boothId}` : t('No Booth ID') }}</small>
              </td>
              <td>
                <div class="import-row-tags">
                  <n-tag v-if="row.draft.universal" size="small" type="success">{{ t('Universal compatibility') }}</n-tag>
                  <span>
                    {{
                      tagOptions(row, 'avatar')
                        .filter((tag) => row.draft.avatarIds.includes(tag.value))
                        .map((tag) => tag.label.replace(t(' · Import new tag'), ''))
                        .join(' / ') || t('No avatar compatibility set')
                    }}
                  </span>
                </div>
                <small v-for="issue in row.issues" :key="issue" class="issue-text">
                  {{ localize(issue) }}
                </small>
                <small v-if="row.existingAssetId || row.existingRowKey" class="issue-text">{{ t('Record already exists; skipped by default. Confirm conflicts before merging.') }}</small>
              </td>
              <td>
                <n-select
                  v-model:value="actions[row.key]"
                  :options="actionOptions(row)"
                  size="small"
                  style="width: 150px"
                  @update:value="(v) => {
                      if (v === 'merge') edit(row);
                    }
                  "
                />
              </td>
              <td>
                <n-button size="small" text type="primary" @click="edit(row)">{{ t('Edit') }}</n-button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="pagination-footer">
        <span class="subtle">{{ t('{p0} preview records', { p0: filteredRows.length }) }}</span>
        <n-pagination v-model:page="page" :item-count="filteredRows.length" :page-size="20" />
      </div>
      <n-alert type="warning" :bordered="false">{{ t('Rows with issues are skipped by default. Review tags and covers before selecting Add. Merging keeps both sets of tags; review each conflicting non-empty field.') }}</n-alert>
      <div class="wizard-actions">
        <n-button :disabled="busy" @click="report">{{ t('Save issue report') }}</n-button>
        <n-button type="primary" :loading="busy" @click="confirmImport">{{ t('Import…') }}</n-button>
      </div>
    </template>
    <n-card v-if="step === 4 && result" :bordered="false" class="import-result">
      <div class="success-symbol">✓</div>
      <h2>{{ t('Import complete') }}</h2>
      <p>{{ t(
          'Added {p0} · Merged {p1} · Skipped {p2} · Failed {p3}',
          {
            p0: result.created,
            p1: result.merged,
            p2: result.skipped,
            p3: result.failed,
          },
        ) }}
      </p>
      <p class="subtle">{{ t(
          '{p0} source records. The source file was not modified.',
          {
            p0: result.created + result.merged + result.skipped + result.failed,
          },
        ) }}</p>
      <n-space justify="center">
        <n-button @click="report">{{ t('Save CSV report') }}</n-button>
        <n-button type="primary" @click="restart">{{ t('Import another workbook') }}</n-button>
      </n-space>
    </n-card>
    <n-modal
      v-if="editRow"
      :show="editVisible"
      :mask-closable="!busy"
      :close-on-esc="!busy"
      @update:show="(show) => { if (!show) closeEdit(); }"
      @after-leave="clearClosedEdit"
    >
      <n-card
        v-if="editRow"
        :title="t('Review imported record')"
        class="import-edit-modal"
        :closable="!busy"
        @close="closeEdit"
      >
        <p class="subtle">{{ t('{p0} · Row {p1}', { p0: editRow.sheet, p1: editRow.row }) }}</p>
        <n-alert
          v-for="issue in editRow.issues"
          :key="issue"
          type="warning"
          style="margin-bottom: 8px"
        >
          {{ localize(issue) }}
        </n-alert>
        <n-form label-placement="top">
          <div class="form-two">
            <n-form-item :label="t('Name *')"><n-input v-model:value="editRow.draft.name" /></n-form-item>
            <n-form-item :label="t('Booth ID')">
              <n-input v-model:value="editRow.draft.boothId" />
            </n-form-item>
          </div>
          <n-form-item :label="t('Category')">
            <n-select v-model:value="editRow.draft.categoryId" :options="categoryOptions" />
          </n-form-item>
          <div v-if="editRow.draft.newTags.length" class="import-new-tags">
            <h4>{{ t('Confirm names for new imported tags') }}</h4>
            <p class="subtle">{{ t('Edit the name, or select an existing tag and remove this new one. Tags are not merged automatically.') }}</p>
            <label
              v-for="tag in editRow.draft.newTags"
              :key="tag.key"
              class="section-line"
              style="margin-bottom: 8px"
            >
              <span>{{ tag.kind === 'avatar' ? t('Avatar') : t('Custom tags') }}</span>
              <n-input
                :value="tag.name"
                @update:value="(name) => renameTag(editRow!, tag.key, name)"
              />
            </label>
          </div>
          <n-form-item :label="t('Compatible avatars')">
            <n-select
              v-model:value="editRow.draft.avatarIds"
              multiple
              filterable
              :options="tagOptions(editRow, 'avatar')"
            />
          </n-form-item>
          <n-checkbox v-model:checked="editRow.draft.universal">{{ t('Universal compatibility') }}</n-checkbox>
          <n-form-item :label="t('Custom tags')" style="margin-top: 16px">
            <n-select
              v-model:value="editRow.draft.freeTagIds"
              multiple
              filterable
              :options="tagOptions(editRow, 'free')"
            />
          </n-form-item>
          <n-form-item :label="t('Notes')">
            <n-input v-model:value="editRow.draft.notes" type="textarea" />
          </n-form-item>
          <n-form-item :label="t('Linked folder')">
            <n-input v-model:value="editRow.draft.folderPath" />
          </n-form-item>
        </n-form>
        <div v-if="editRow.imageOptions.length">
          <h4>{{ t('Choose imported cover{p0}', { p0: editRow.imageOptions.length > 1 ? t('(multiple images; choose one)') : '' }) }}
          </h4>
          <div class="import-cover-options">
            <button
              v-for="(image, index) in editRow.imageOptions"
              :key="image.token"
              :class="{ active: editRow.draft.coverToken === image.token }"
              @click="selectCover(editRow, index)"
            >
              <img :src="image.previewUrl" :alt="t('Suggested cover')" />
              <span>{{ editRow.draft.coverToken === image.token ? t('✓ Selected') : t('Use this cover') }}</span>
            </button>
            <n-button
              size="small"
              @click="editRow.draft.coverToken = '';
                editRow.draft.coverId = '';
              "
            >{{ t('Import without cover') }}</n-button>
          </div>
        </div>
        <section v-if="editRow.existingAssetId || editRow.existingRowKey" class="merge-conflicts">
          <h3>{{ t('Merge into existing record') }}</h3>
          <p v-if="loadingExisting">{{ t('Reading target record…') }}</p>
          <template v-if="existing[editRow.key]">
            <p class="subtle">
              {{ t('{p0}: Tags and universal compatibility will be combined. Choose which values to keep below.', { p0: existing[editRow.key].name }) }}</p>
            <div v-for="field in conflictFields(editRow)" :key="field" class="conflict-field">
              <strong>{{ fieldNames[field] }}</strong>
              <div class="conflict-values">
                <label>
                  <n-checkbox
                    :checked="choices[editRow.key]?.[field] === 'existing'"
                    @update:checked="() => {
                        choices[editRow!.key] ||= {};
                        choices[editRow!.key][field] = 'existing';
                      }
                    "
                  >{{ t('Keep existing') }}</n-checkbox>
                  <p>{{ fieldValue(existing[editRow.key], field) }}</p>
                </label>
                <label>
                  <n-checkbox
                    :checked="choices[editRow.key]?.[field] === 'incoming'"
                    @update:checked="() => {
                        choices[editRow!.key] ||= {};
                        choices[editRow!.key][field] = 'incoming';
                      }
                    "
                  >{{ t('Use imported value') }}</n-checkbox>
                  <p>{{ fieldValue(editRow.draft, field) }}</p>
                </label>
              </div>
            </div>
            <n-tag v-if="currentConflicts.length" type="warning">{{ t('{p0} conflicts still need confirmation', { p0: currentConflicts.length }) }}</n-tag>
            <n-tag v-else type="success">{{ t('Field conflicts confirmed') }}</n-tag>
          </template>
        </section>
        <n-select
          v-model:value="actions[editRow.key]"
          :options="actionOptions(editRow)"
          style="margin-top: 20px"
        />
        <template #footer>
          <n-space justify="end">
            <n-button type="primary" :disabled="busy" @click="closeEdit">{{ t('Finish review') }}</n-button>
          </n-space>
        </template>
      </n-card>
    </n-modal>
  </div>
</template>
