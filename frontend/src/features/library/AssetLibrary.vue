<script setup lang="ts">
import { confirmAction } from '../../shared/confirm-action';
import { localize, t } from '../../i18n';
import { computed, h, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import {
  NInput,
  NButton,
  NSelect,
  NCheckbox,
  NTag,
  NEmpty,
  NSpace,
  NPagination,
  NSpin,
  NAlert,
  NDataTable,
  NModal,
  NCard,
  useDialog,
  useMessage,
} from 'naive-ui';
import type { DataTableColumns, InputInst, SelectOption } from 'naive-ui';
import type { Asset, AssetQuery, Bootstrap, Preferences } from '../../api/types';
import { api, errorMessage, sourceErrorMessage } from '../../api/client';
import { createQuery } from '../editor/draft';
import { useDelayedLoading } from '../../shared/use-delayed-loading';
import CoverPlaceholderIcon from '../../shared/CoverPlaceholderIcon.vue';
import ViewModeIcon from '../../shared/ViewModeIcon.vue';

const props = defineProps<{ data: Bootstrap; preferences: Preferences; version: number }>();
const emit = defineEmits<{
  open: [id: string];
  edit: [asset: Asset];
  create: [];
  preferences: [p: Preferences];
  changed: [];
}>();
const message = useMessage();
const dialog = useDialog();

// 查询条件与列表结果；loadEpoch 用于丢弃旧请求的返回结果。
const query = reactive<AssetQuery>({ ...createQuery(), ...props.preferences.query });
const items = ref<Asset[]>([]);
const total = ref(0);
const loading = ref(false);
const showLoading = useDelayedLoading(loading);
const hasLoaded = ref(false);
const error = ref('');
const selected = ref<string[]>([]);
const searchInput = ref<InputInst | null>(null);

const view = ref(props.preferences.view);
const page = computed({
  get: () => Math.floor(query.offset / query.limit) + 1,

  set: (n: number) => (query.offset = (n - 1) * query.limit),
});

// 批量操作在对话框中暂存，确认后才提交。
const batchOpen = ref(false);
const batchOperation = ref('category');
const batchCategory = ref('');
const batchTags = ref<string[]>([]);
const batchBusy = ref(false);
let loadEpoch = 0;
let timer: ReturnType<typeof setTimeout> | undefined;

const categories = computed(() =>
  props.data.categories.map((tag) => ({ label: `${tag.name} (${tag.count})`, value: tag.id })),
);
const avatars = computed(() => props.data.avatarTags.map((tag) => ({ label: tag.name, value: tag.id })));
const freeTags = computed(() => props.data.freeTags.map((tag) => ({ label: tag.name, value: tag.id })));

const avatarModeOptions = computed(() => [
  { label: t('Specific avatars'), value: 'selected' },
  { label: t('Universal compatibility only'), value: 'universal' },
  { label: t('No avatar compatibility set'), value: 'unknown' },
]);
const matchOptions = computed(() => [
  { label: t('Match any tag'), value: 'any' },
  { label: t('Match all tags'), value: 'all' },
]);
const freeModeOptions = computed(() => [
  { label: t('Include'), value: 'include' },
  { label: t('Exclude'), value: 'exclude' },
]);
const folderOptions = computed(() => [
  { label: t('All folder states'), value: 'all' },
  { label: t('Has linked folder'), value: 'linked' },
  { label: t('No linked folder'), value: 'unlinked' },
]);

// 选项允许换行；标题也为已选的长标签提供完整文本。
function renderFilterLabel(option: SelectOption) {
  const label = String(option.label ?? '');
  return h('span', { class: 'filter-option-label', title: label }, label);
}
const activeCount = computed(
  () =>
    Number(!!query.keyword) +
    query.categoryIds.length +
    Number(query.uncategorized) +
    query.avatarIds.length +
    Number(query.avatarMode !== 'selected') +
    query.freeTagIds.length +
    Number(query.folder !== 'all'),
);

function categoryName(id: string) {
  return props.data.categories.find((tag) => tag.id === id)?.name || t('Uncategorized');
}

function persist() {
  emit('preferences', {
    ...props.preferences,
    query: JSON.parse(JSON.stringify(query)),
    view: view.value,
  });
}

async function load() {
  const epoch = ++loadEpoch;
  loading.value = true;
  error.value = '';

  try {
    const result = await api.list({ ...query });
    if (epoch === loadEpoch) {
      items.value = result.items || [];
      total.value = result.total;
      if (query.offset >= result.total && query.offset > 0) {
        query.offset = 0;
      }
    }
  } catch (e) {
    if (epoch === loadEpoch) {
      error.value = sourceErrorMessage(e);
    }
  } finally {
    if (epoch === loadEpoch) {
      loading.value = false;
      hasLoaded.value = true;
    }
  }
}

// 条件变化先保存偏好，再防抖刷新列表。
watch(
  query,
  () => {
    persist();
    if (timer) {
      clearTimeout(timer);
    }

    timer = setTimeout(load, 220);
  },
  { deep: true },
);

watch(view, persist);

watch(() => props.version, load);

watch(
  () => [
    query.keyword,
    JSON.stringify(query.categoryIds),
    query.uncategorized,
    JSON.stringify(query.avatarIds),
    query.avatarMode,
    query.avatarMatch,
    JSON.stringify(query.freeTagIds),
    query.freeMode,
    query.freeMatch,
    query.folder,
    query.sort,
    query.descending,
  ],
  () => (query.offset = 0),
);

function clear() {
  Object.assign(query, createQuery(), { limit: query.limit });
}

function toggle(id: string, checked: boolean) {
  selected.value = checked
    ? [...new Set([...selected.value, id])]
    : selected.value.filter((x) => x !== id);
}

function chooseCategory(id: string | null) {
  query.categoryIds = id && id !== 'uncategorized' ? [id] : [];
  query.uncategorized = id === 'uncategorized';
  query.offset = 0;
}

function browseTag(kind: string, id: string) {
  clear();
  if (kind === 'avatar') {
    query.avatarIds = [id];
  } else {
    query.freeTagIds = [id];
  }
}

async function openFolder(asset: Asset) {
  try {
    await api.openFolder(asset.folderPath);
  } catch (e) {
    message.error(errorMessage(e));
  }
}

async function booth(asset: Asset) {
  try {
    await api.openBooth(asset.boothId);
  } catch (e) {
    message.error(errorMessage(e));
  }
}

function confirmBatch() {
  if (!selected.value.length) {
    return;
  }

  const operation = batchOperation.value;
  const label =
    operation === 'delete'
      ? t('Delete assets')
      : operation === 'category'
        ? t('Change category')
        : operation === 'addTags'
          ? t('Add tags')
          : t('Remove tags');
  confirmAction(dialog, {
    title: t('Apply “{p0}” to {p1} assets?', { p0: label, p1: selected.value.length }),
    content:
      operation === 'delete'
        ? t('Asset records and covers will be deleted. Linked files are preserved. Deleted data can only be restored from a full backup.')
        : t(
          'This action will apply to the {p0} selected assets. {p1}',
          {
            p0: selected.value.length,
            p1: operation === 'category' ? t('Target category: {p0}', { p0: (categoryName(batchCategory.value)) }) : '',
          },
        ),
    positiveText: t('Confirm action'),
    negativeText: t('Back to review'),
    onPositiveClick: async () => {
      batchBusy.value = true;

      try {
        await api.batch(selected.value, operation, batchCategory.value, batchTags.value);
        selected.value = [];
        batchOpen.value = false;
        message.success(t('Bulk action complete'));
        emit('changed');
        await load();
      } catch (e) {
        message.error(errorMessage(e));
      } finally {
        batchBusy.value = false;
      }
    },
  });
}

const columns = computed<DataTableColumns<Asset>>(() => [
  { type: 'selection' },
  {
    title: t('Asset'),
    key: 'name',
    minWidth: 220,
    render: (a) =>
      h('button', { class: 'table-asset', onClick: () => emit('open', a.id) }, [
        a.thumbnailUrl
          ? h('img', { src: a.thumbnailUrl, loading: 'lazy', alt: '' })
          : h('span', { class: 'mini-placeholder' }, [h(CoverPlaceholderIcon)]),
        h('span', {}, [h('strong', a.name), h('small', `# ${a.boothId || t('No ID recorded')}`)]),
      ]),
  },
  { title: t('Category'), key: 'categoryId', width: 90, render: (a) => categoryName(a.categoryId) },
  {
    title: () => t('Avatar'),
    key: 'avatars',
    minWidth: 180,
    ellipsis: { tooltip: true },
    render: (a) =>
      [a.universal ? t('Universal compatibility · ') : '', ...(a.avatars || []).map((tag) => tag.name)].join(' / ') ||
      t('No avatar compatibility set'),
  },
  {
    title: t('Custom tags'),
    key: 'freeTags',
    minWidth: 130,
    ellipsis: { tooltip: true },
    render: (a) => (a.freeTags || []).map((tag) => tag.name).join(' / ') || '—',
  },
  { title: t('Notes'), key: 'notes', minWidth: 150, ellipsis: { tooltip: true } },
  {
    title: t('Folder'),
    key: 'folderPath',
    width: 100,
    render: (a) =>
      a.folderPath
        ? h(
            NButton,
            { size: 'small', quaternary: true, onClick: () => openFolder(a) },
            { default: () => t('Open folder') },
          )
        : h('span', { class: 'subtle' }, t('Not linked')),
  },
  {
    title: t('Actions'),
    key: 'actions',
    width: 64,
    render: (a) =>
      h(
        NButton,
        { size: 'small', text: true, type: 'primary', onClick: () => emit('edit', a) },
        { default: () => t('Edit') },
      ),
  },
]);

defineExpose({
  chooseCategory,
  browseTag,
  focusSearch: () => searchInput.value?.focus(),
  refresh: load,
});

onMounted(load);

onBeforeUnmount(() => {
  if (timer) {
    clearTimeout(timer);
  }

  loadEpoch++;
});
</script>

<template>
  <div class="library-page">
    <header class="page-heading">
      <div>
        <h1>{{ t('Asset library') }}<small>{{ t('{p0} assets', { p0: total }) }}</small>
        </h1>
      </div>
      <n-space>
        <n-button type="primary" @click="emit('create')">{{ t('＋ Add asset') }}</n-button>
      </n-space>
    </header>
    <div class="search-bar">
      <n-input
        ref="searchInput"
        v-model:value="query.keyword"
        clearable
        size="large"
        :placeholder="t('Search names, Booth IDs, avatars, tags or notes…')"
      >
        <template #prefix>⌕</template>

      </n-input>
      <n-button
        :type="view === 'cards' ? 'primary' : 'default'"
        :secondary="view === 'cards'"
        :aria-pressed="view === 'cards'"
        @click="view = 'cards'"
      >
        <template #icon><ViewModeIcon mode="cards" /></template>
        {{ t('Cards') }}
      </n-button>
      <n-button
        :type="view === 'table' ? 'primary' : 'default'"
        :secondary="view === 'table'"
        :aria-pressed="view === 'table'"
        @click="view = 'table'"
      >
        <template #icon><ViewModeIcon mode="table" /></template>
        {{ t('Table') }}
      </n-button>
    </div>
    <div class="filter-panel">
      <div class="filter-row">
        <span class="filter-label">{{ t('Category') }}</span>
        <n-select
          v-model:value="query.categoryIds"
          multiple
          clearable
          filterable
          :options="categories"
          :placeholder="t('All categories')"
        />
        <n-checkbox v-model:checked="query.uncategorized">{{ t('Include uncategorized') }}</n-checkbox>
      </div>
      <div class="filter-row">
        <span class="filter-label">{{ t('Avatar') }}</span>
        <div class="filter-choice mode-select">
          <span v-for="option in avatarModeOptions" :key="option.value" class="filter-choice-sizer" aria-hidden="true">{{ option.label }}</span>
          <n-select v-model:value="query.avatarMode" :options="avatarModeOptions" :render-label="renderFilterLabel" :virtual-scroll="false" />
        </div>
        <n-select
          v-if="query.avatarMode === 'selected'"
          v-model:value="query.avatarIds"
          multiple
          clearable
          filterable
          :options="avatars"
          :placeholder="t('Select avatar (includes universal assets)')"
        />
        <div v-if="query.avatarMode === 'selected'" class="filter-choice match-select">
          <span v-for="option in matchOptions" :key="option.value" class="filter-choice-sizer" aria-hidden="true">{{ option.label }}</span>
          <n-select v-model:value="query.avatarMatch" :options="matchOptions" :render-label="renderFilterLabel" :virtual-scroll="false" />
        </div>
      </div>
      <div class="filter-row">
        <span class="filter-label">{{ t('Custom tags') }}</span>
        <n-select
          v-model:value="query.freeTagIds"
          class="tag-filter-select"
          multiple
          clearable
          filterable
          :options="freeTags"
          :render-label="renderFilterLabel"
          :virtual-scroll="false"
          :placeholder="t('All custom tags')"
        />
        <div class="filter-choice match-select">
          <span v-for="option in freeModeOptions" :key="option.value" class="filter-choice-sizer" aria-hidden="true">{{ option.label }}</span>
          <n-select v-model:value="query.freeMode" :options="freeModeOptions" :render-label="renderFilterLabel" :virtual-scroll="false" />
        </div>
        <div class="filter-choice match-select">
          <span v-for="option in matchOptions" :key="option.value" class="filter-choice-sizer" aria-hidden="true">{{ option.label }}</span>
          <n-select v-model:value="query.freeMatch" :options="matchOptions" :render-label="renderFilterLabel" :virtual-scroll="false" />
        </div>
      </div>
    </div>
    <div class="results-toolbar">
      <div class="toolbar-left">
        <n-checkbox
          :checked="items.length > 0 && items.every((a) => selected.includes(a.id))"
          :indeterminate="items.some((a) => selected.includes(a.id)) &&
            !items.every((a) => selected.includes(a.id))
          "
          @update:checked="(v) => items.forEach((a) => toggle(a.id, v))"
        >{{ t('Select all on this page') }}</n-checkbox>
        <n-tag v-if="activeCount" closable size="small" :bordered="false" @close="clear">
          {{ t('{p0} filters · Clear', { p0: activeCount }) }}</n-tag>
        <n-button
          v-if="selected.length"
          size="small"
          type="primary"
          secondary
          @click="batchOpen = true"
        >{{ t('Bulk actions ({p0})', { p0: selected.length }) }}</n-button>
        <n-button v-if="selected.length" size="small" text @click="selected = []">{{ t('Clear selection') }}</n-button>
      </div>
      <div class="toolbar-right">
        <div class="filter-choice folder-select">
          <span v-for="option in folderOptions" :key="option.value" class="filter-choice-sizer" aria-hidden="true">{{ option.label }}</span>
          <n-select v-model:value="query.folder" :options="folderOptions" :render-label="renderFilterLabel" :virtual-scroll="false" />
        </div>
        <n-select
          v-model:value="query.sort"
          class="compact-select"
          :options="[
            { label: t('Created'), value: 'createdAt' },
            { label: t('Updated'), value: 'updatedAt' },
            { label: t('Name'), value: 'name' },
            { label: 'Booth ID', value: 'boothId' },
          ]"
        />
        <n-button size="small" @click="query.descending = !query.descending">
          {{ query.descending ? t('↓ Descending') : t('↑ Ascending') }}
        </n-button>
      </div>
    </div>
    <n-alert v-if="error" type="error" style="margin-bottom: 20px">
      {{ localize(error) }}
      <n-button text @click="load">{{ t('Retry') }}</n-button>
    </n-alert>
    <n-spin class="library-results" content-class="library-results-content" :show="showLoading" :aria-busy="loading">
      <div
        v-if="items.length && view === 'cards'"
        class="asset-grid"
        :class="`size-${preferences.cardSize}`"
      >
        <article
          v-for="asset in items"
          :key="asset.id"
          class="asset-card"
          :class="{ selected: selected.includes(asset.id) }"
          tabindex="0"
          @click="emit('open', asset.id)"
          @keydown.enter="emit('open', asset.id)"
        >
          <div class="asset-cover">
            <img
              v-if="asset.thumbnailUrl || asset.coverUrl"
              :src="asset.thumbnailUrl || asset.coverUrl"
              :alt="asset.name"
              loading="lazy"
              @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"
            />
            <div v-else class="image-placeholder">
              <CoverPlaceholderIcon />
              <small>{{ t('No cover') }}</small>
            </div>
            <div class="card-select" @click.stop>
              <n-checkbox
                :checked="selected.includes(asset.id)"
                :aria-label="t('Select {p0}', { p0: asset.name })"
                @update:checked="(v) => toggle(asset.id, v)"
              />
            </div>
            <span v-if="asset.universal" class="universal-badge">{{ t('Universal compatibility') }}</span>
          </div>
          <div class="asset-card-body">
            <div class="card-meta">
              <span>{{ categoryName(asset.categoryId) }}</span>
              <span>{{ asset.boothId ? `#${asset.boothId}` : t('No ID recorded') }}</span>
            </div>
            <h3 :title="asset.name">{{ asset.name }}</h3>
            <div class="card-tags">
              <span v-for="tag in (asset.avatars || []).slice(0, 3)" :key="tag.id">
                {{ tag.name }}
              </span>
              <span v-if="(asset.avatars?.length || 0) > 3">+{{ asset.avatars.length - 3 }}</span>
              <span v-if="!asset.avatars?.length && !asset.universal" class="subtle">{{ t('No avatar compatibility set') }}</span>
            </div>
            <div class="card-footer">
              <button :disabled="!asset.folderPath" @click.stop="openFolder(asset)">
                {{ asset.folderPath ? t('▱ Open folder') : t('▱ No linked folder') }}
              </button>
              <button v-if="asset.boothId" @click.stop="booth(asset)">{{ t('Booth ↗') }}</button>
            </div>
          </div>
        </article>
      </div>
      <n-data-table
        v-else-if="items.length"
        :columns="columns"
        :data="items"
        :row-key="(a: Asset) => a.id"
        :checked-row-keys="selected"
        :scroll-x="1100"
        @update:checked-row-keys="(keys) => (selected = keys.map(String))"
      />
      <div v-else class="empty-library">
        <n-empty
          v-if="hasLoaded"
          :description="activeCount ? t('No matching assets') : t('No assets')"
        />
      </div>
    </n-spin>
    <footer v-if="total" class="pagination-footer">
      <span class="subtle">{{ t(
          'Showing {p0}–{p1} / {p2}',
          {
            p0: query.offset + 1,
            p1: Math.min(query.offset + query.limit, total),
            p2: total,
          },
        ) }}
      </span>
      <n-pagination
        v-model:page="page"
        :item-count="total"
        :page-size="query.limit"
        show-size-picker
        :page-sizes="[24, 60, 120]"
        @update:page-size="(n) => {
            query.limit = n;
            query.offset = 0;
          }
        "
      />
    </footer>
    <n-modal
      v-model:show="batchOpen"
      preset="card"
      :title="t('Bulk actions')"
      class="small-modal"
      :mask-closable="!batchBusy"
      :close-on-esc="!batchBusy"
      :closable="!batchBusy"
    >
      <p>{{ t('{p0} assets selected (including other pages)', { p0: selected.length }) }}</p>
      <n-select
        v-model:value="batchOperation"
        :options="[
          { label: t('Change category'), value: 'category' },
          { label: t('Add tags'), value: 'addTags' },
          { label: t('Remove tags'), value: 'removeTags' },
          { label: t('Delete assets'), value: 'delete' },
        ]"
      />
      <n-select
        v-if="batchOperation === 'category'"
        v-model:value="batchCategory"
        style="margin-top: 16px"
        :options="[{ label: t('Uncategorized'), value: '' }, ...categories]"
      />
      <n-select
        v-if="['addTags', 'removeTags'].includes(batchOperation)"
        v-model:value="batchTags"
        multiple
        filterable
        style="margin-top: 16px"
        :options="[
          ...avatars.map((x) => ({ ...x, label: `${t('Avatar')} · ${x.label}` })),
          ...freeTags.map((x) => ({ ...x, label: t('Tags · {p0}', { p0: x.label }) })),
        ]"
        :placeholder="t('Select tags to add or remove')"
      />
      <n-alert v-if="batchOperation === 'delete'" type="warning" style="margin-top: 16px">{{ t('Delete asset records and covers, keeping linked files. Restore deleted data only from a full backup.') }}</n-alert>
      <template #footer>
        <n-space justify="end">
          <n-button :disabled="batchBusy" @click="batchOpen = false">{{ t('Cancel') }}</n-button>
          <n-button
            :type="batchOperation === 'delete' ? 'error' : 'primary'"
            :loading="batchBusy"
            :disabled="['addTags', 'removeTags'].includes(batchOperation) && !batchTags.length"
            @click="confirmBatch"
          >{{ t('Review & confirm') }}</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>
