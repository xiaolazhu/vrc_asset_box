<script setup lang="ts">
import { confirmAction } from '../../shared/confirm-action';
import { localize, t } from '../../i18n';
import { computed, h, ref } from 'vue';
import {
  NButton,
  NInput,
  NDataTable,
  NTag,
  NSpace,
  NModal,
  NForm,
  NFormItem,
  NSelect,
  NAlert,
  useDialog,
  useMessage,
} from 'naive-ui';
import type { DataTableColumns } from 'naive-ui';
import type { Bootstrap, Tag, Category } from '../../api/types';
import { api, errorMessage, sourceErrorMessage } from '../../api/client';

const props = defineProps<{ kind: 'category' | 'avatar' | 'free'; data: Bootstrap }>();
const emit = defineEmits<{ changed: []; browse: [kind: string, id: string] }>();

// 当前列表查询与分类 / 标签编辑表单。
const search = ref('');
const editing = ref(false);
const saving = ref(false);
const error = ref('');
const id = ref('');
const name = ref('');
const aliases = ref('');

// 合并标签与删除分类分别保留独立的确认状态。
const mergeOpen = ref(false);
const mergeSource = ref<Tag | null>(null);
const mergeTarget = ref('');
const deleteCategoryOpen = ref(false);
const deletingCategory = ref<Category | null>(null);
const deleteTarget = ref('');
const dialog = useDialog();
const message = useMessage();

const title = computed(() =>
  props.kind === 'category' ? t('Category management') : props.kind === 'avatar' ? t('Avatar tags') : t('Custom tags'),
);
const rows = computed(() => {
  const list =
    props.kind === 'category'
      ? props.data.categories
      : props.kind === 'avatar'
        ? props.data.avatarTags
        : props.data.freeTags;
  return list.filter((tag) =>
    `${tag.name} ${'aliases' in tag ? tag.aliases.join(' ') : ''}`
      .toLowerCase()
      .includes(search.value.toLowerCase()),
  );
});

function edit(row?: Tag | Category) {
  id.value = row?.id || '';
  name.value = row?.name || '';
  aliases.value = row && 'aliases' in row ? row.aliases.join('\n') : '';
  error.value = '';
  editing.value = true;
}

async function save() {
  if (!name.value.trim()) {
    error.value = 'Name cannot be empty';
    return;
  }

  saving.value = true;
  error.value = '';

  try {
    if (props.kind === 'category') {
      await api.saveCategory({
        id: id.value,
        name: name.value,
        position:
          props.data.categories.find((c) => c.id === id.value)?.position ??
          props.data.categories.length,
      });
    } else {
      await api.saveTag({
        id: id.value,
        kind: props.kind,
        name: name.value,
        aliases:
          props.kind === 'avatar'
            ? aliases.value
                .split(/\n|,|，|;|；/)
                .map((s) => s.trim())
                .filter(Boolean)
            : [],
      });
    }

    editing.value = false;
    message.success(t('Saved'));
    emit('changed');
  } catch (e) {
    error.value = sourceErrorMessage(e);
  } finally {
    saving.value = false;
  }
}

function remove(row: Tag | Category) {
  if (props.kind === 'category') {
    deletingCategory.value = row as Category;
    deleteTarget.value = '';
    deleteCategoryOpen.value = true;

    return;
  }

  confirmAction(dialog, {
    title: t('Delete tag “{p0}”?', { p0: row.name }),
    content: t('Remove this tag from {p0} assets. Asset records and files are preserved.', { p0: row.count }),
    positiveText: t('Delete tag'),
    negativeText: t('Cancel'),
    onPositiveClick: async () => {
      try {
        await api.deleteTag(row.id);
        message.success(t('Tag deleted'));
        emit('changed');
      } catch (e) {
        message.error(errorMessage(e));
      }
    },
  });
}

async function deleteCategory() {
  if (!deletingCategory.value) {
    return;
  }

  saving.value = true;

  try {
    await api.deleteCategory(deletingCategory.value.id, deleteTarget.value);
    deleteCategoryOpen.value = false;
    emit('changed');
    message.success(t('Category deleted and assets transferred'));
  } catch (e) {
    message.error(errorMessage(e));
  } finally {
    saving.value = false;
  }
}

function merge(row: Tag) {
  mergeSource.value = row;
  mergeTarget.value = '';
  mergeOpen.value = true;
  error.value = '';
}

function confirmMerge() {
  const source = mergeSource.value;
  if (!source || !mergeTarget.value) {
    return;
  }

  const target = (props.kind === 'avatar' ? props.data.avatarTags : props.data.freeTags).find(
    (tag) => tag.id === mergeTarget.value,
  );
  confirmAction(dialog, {
    title: t('Merge {p0} into {p1}?', { p0: source.name, p1: target?.name }),
    content: t('Transfer tag assignments for {p0} assets, remove duplicates, then delete the source tag. Available avatar aliases are also transferred.', { p0: source.count }),
    positiveText: t('Confirm merge'),
    negativeText: t('Back to review'),
    onPositiveClick: async () => {
      saving.value = true;

      try {
        await api.mergeTags(source.id, mergeTarget.value);
        mergeOpen.value = false;
        emit('changed');
        message.success(t('Tags merged'));
      } catch (e) {
        error.value = sourceErrorMessage(e);
      } finally {
        saving.value = false;
      }
    },
  });
}

// 仅手柄启动拖动；完整分类顺序用于保存，搜索结果不会覆盖未显示的分类。
const draggedCategory = ref('');
const dropTarget = ref('');
const dropAfter = ref(false);
const reordering = ref(false);

function endDrag() {
  draggedCategory.value = '';
  dropTarget.value = '';
  dropAfter.value = false;
}

function startDrag(event: DragEvent, category: Category) {
  if (reordering.value) {
    event.preventDefault();
    return;
  }

  draggedCategory.value = category.id;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', category.id);
  }
}

function dragOver(event: DragEvent, category: Category) {
  if (!draggedCategory.value || reordering.value || draggedCategory.value === category.id) {
    return;
  }

  event.preventDefault();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'move';
  }
  const bounds = (event.currentTarget as HTMLElement).getBoundingClientRect();
  dropTarget.value = category.id;
  dropAfter.value = event.clientY > bounds.top + bounds.height / 2;
}

async function dropCategory(event: DragEvent, category: Category) {
  const source = draggedCategory.value;
  if (!source || source === category.id || reordering.value) {
    return;
  }

  event.preventDefault();
  const bounds = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const after = event.clientY > bounds.top + bounds.height / 2;
  const list = props.data.categories.map(item => item.id);
  const sourceIndex = list.indexOf(source);
  if (sourceIndex < 0 || !list.includes(category.id)) {
    endDrag();
    return;
  }

  list.splice(sourceIndex, 1);
  list.splice(list.indexOf(category.id) + Number(after), 0, source);
  endDrag();
  if (list.every((id, index) => id === props.data.categories[index].id)) {
    return;
  }

  reordering.value = true;
  try {
    await api.reorderCategories(list);
    emit('changed');
  } catch (e) {
    message.error(errorMessage(e));
  } finally {
    reordering.value = false;
  }
}

function categoryRowProps(row: Tag | Category) {
  if (props.kind !== 'category') {
    return {};
  }

  return {
    'data-category-id': row.id,
    class: {
      'category-drop-before': dropTarget.value === row.id && !dropAfter.value,
      'category-drop-after': dropTarget.value === row.id && dropAfter.value,
    },
    onDragover: (event: DragEvent) => dragOver(event, row as Category),
    onDrop: (event: DragEvent) => void dropCategory(event, row as Category),
  };
}

const columns = computed<DataTableColumns<Tag | Category>>(() => {
  const cols: DataTableColumns<Tag | Category> = [
    {
      title: t('Name'),
      key: 'name',
      minWidth: 160,
      render: (row) => h('div', { class: 'taxonomy-name' }, [
        ...(props.kind === 'category' ? [
          h('button', {
            type: 'button',
            class: 'category-drag-handle',
            draggable: !reordering.value,
            disabled: reordering.value,
            title: t('Drag to reorder categories'),
            'aria-label': t('Drag to reorder categories'),
            onDragstart: (event: DragEvent) => startDrag(event, row as Category),
            onDragend: endDrag,
          }, '⠿'),
        ] : []),
        h('strong', row.name),
      ]),
    },
  ];
  if (props.kind === 'avatar') {
    cols.push({
      title: t('Aliases'),
      key: 'aliases',
      minWidth: 220,
      render: (r) =>
        h(
          NSpace,
          { size: 5 },
          {
            default: () =>
              ('aliases' in r ? r.aliases : []).map((a) =>
                h(NTag, { size: 'small', bordered: false }, { default: () => a }),
              ),
          },
        ),
    });
  }

  cols.push(
    {
      title: t('Linked assets'),
      key: 'count',
      width: 100,
      render: (r) =>
        h(
          NButton,
          { text: true, type: 'primary', onClick: () => emit('browse', props.kind, r.id) },
          { default: () => t('{p0} assets', { p0: r.count }) },
        ),
    },
    {
      title: t('Actions'),
      key: 'actions',
      width: props.kind === 'category' ? 140 : 220,
      render: (r) =>
        h(
          NSpace,
          { size: 14 },
          {
            default: () => [
              h(NButton, { text: true, onClick: () => edit(r) }, { default: () => t('Edit') }),
              ...(props.kind === 'category' ? [] : [
                h(
                  NButton,
                  { text: true, onClick: () => merge(r as Tag) },
                  { default: () => t('Merge') },
                ),
              ]),
              h(
                NButton,
                { text: true, type: 'error', onClick: () => remove(r) },
                { default: () => t('Delete') },
              ),
            ],
          },
        ),
    },
  );

  return cols;
});
</script>

<template>
  <div class="management-page">
    <header class="page-heading">
      <div>
        <h1>{{ title }}</h1>
        <p class="subtle">
          {{
            kind === 'avatar'
              ? t('Manage avatar names and aliases. Changes apply to all linked assets.')
              : kind === 'free'
                ? t('Manage custom tags. Renaming updates all linked assets.')
                : t('Manage categories and drag to reorder. Transfer assets when deleting a category.')
          }}
        </p>
      </div>
      <n-button type="primary" @click="edit()">{{ t('＋ Add {p0}', { p0: kind === 'category' ? t('Category') : t('Tags') }) }}
      </n-button>
    </header>
    <div class="management-toolbar">
      <n-input
        v-model:value="search"
        clearable
        :placeholder="t('Search {p0}{p1}', { p0: title, p1: kind === 'avatar' ? t(' or alias') : '' })"
        style="max-width: 380px"
      />
      <span class="subtle">{{ t('{p0} items in total', { p0: rows.length }) }}</span>
    </div>
    <n-data-table
      :columns="columns"
      :data="rows"
      :row-key="(r) => r.id"
      :pagination="kind === 'category' ? false : { pageSize: 20 }"
      :row-props="categoryRowProps"
      :loading="reordering"
      :bordered="false"
    />
  </div>
  <n-modal
    v-model:show="editing"
    preset="card"
    :title="kind === 'category'
      ? (id ? t('Edit category') : t('Add category'))
      : (id ? t('Edit tag') : t('Add tag'))"
    class="small-modal"
    :mask-closable="!saving"
    :close-on-esc="!saving"
    :closable="!saving"
  >
    <n-alert v-if="error" type="error" style="margin-bottom: 16px">{{ localize(error) }}</n-alert>
    <n-form label-placement="top">
      <n-form-item :label="kind === 'avatar' ? t('English display name') : t('Name')">
        <n-input v-model:value="name" autofocus @keydown.enter="save" />
      </n-form-item>
      <n-form-item v-if="kind === 'avatar'" :label="t('Aliases (separate with lines or commas)')">
        <n-input
          v-model:value="aliases"
          type="textarea"
          :autosize="{ minRows: 4, maxRows: 10 }"
          :placeholder="t('Japanese name, alternative spellings…')"
        />
      </n-form-item>
    </n-form>
    <template #footer>
      <n-space justify="end">
        <n-button :disabled="saving" @click="editing = false">{{ t('Cancel') }}</n-button>
        <n-button type="primary" :loading="saving" @click="save">{{ t('Save') }}</n-button>
      </n-space>
    </template>
  </n-modal>
  <n-modal
    v-model:show="mergeOpen"
    preset="card"
    :title="t('Merge tags')"
    class="small-modal"
    :mask-closable="!saving"
    :close-on-esc="!saving"
    :closable="!saving"
  >
    <n-alert v-if="error" type="error">{{ localize(error) }}</n-alert>
    <p>{{ t('Source tag:') }}<strong>{{ mergeSource?.name }}</strong>{{ t('· {p0} linked assets', { p0: mergeSource?.count }) }}</p>
    <n-select
      v-model:value="mergeTarget"
      filterable
      :placeholder="t('Select target tag')"
      :options="(kind === 'avatar' ? data.avatarTags : data.freeTags)
          .filter((tag) => tag.id !== mergeSource?.id)
          .map((tag) => ({ label: `${tag.name} (${tag.count})`, value: tag.id }))
      "
    />
    <template #footer>
      <n-space justify="end">
        <n-button :disabled="saving" @click="mergeOpen = false">{{ t('Cancel') }}</n-button>
        <n-button type="primary" :disabled="!mergeTarget" :loading="saving" @click="confirmMerge">{{ t('Review & confirm') }}</n-button>
      </n-space>
    </template>
  </n-modal>
  <n-modal
    v-model:show="deleteCategoryOpen"
    preset="card"
    :title="t('Delete category & transfer assets')"
    class="small-modal"
    :mask-closable="!saving"
    :close-on-esc="!saving"
    :closable="!saving"
  >
    <p>{{ t('The {p1} assets in “{p0}” will be transferred to:', { p0: deletingCategory?.name, p1: deletingCategory?.count }) }}</p>
    <n-select
      v-model:value="deleteTarget"
      :options="[
        { label: t('Uncategorized'), value: '' },
        ...data.categories
          .filter((c) => c.id !== deletingCategory?.id)
          .map((c) => ({ label: c.name, value: c.id })),
      ]"
    />
    <p class="subtle">{{ t('Asset records, covers and linked files are preserved.') }}</p>
    <template #footer>
      <n-space justify="end">
        <n-button :disabled="saving" @click="deleteCategoryOpen = false">{{ t('Cancel') }}</n-button>
        <n-button type="error" :loading="saving" @click="deleteCategory">{{ t('Delete category') }}</n-button>
      </n-space>
    </template>
  </n-modal>
</template>
