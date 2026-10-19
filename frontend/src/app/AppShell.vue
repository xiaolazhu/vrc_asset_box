<script setup lang="ts">
import appIcon from '../../../build/appicon.png';
import { language, localize, setLanguage, t } from '../i18n';
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { NAlert, NButton, NSpin, NEmpty, useMessage } from 'naive-ui';
import type { Asset, Bootstrap, Preferences } from '../api/types';
import { api, errorMessage, sourceErrorMessage } from '../api/client';
import { createQuery } from '../features/editor/draft';
import AssetLibrary from '../features/library/AssetLibrary.vue';
import AssetDetail from '../features/library/AssetDetail.vue';
import AssetEditor from '../features/editor/AssetEditor.vue';
import TaxonomyPage from '../features/manage/TaxonomyPage.vue';
import SettingsPage from '../features/settings/SettingsPage.vue';
import ImportWizard from '../features/import/ImportWizard.vue';
import NavIcon from '../shared/NavIcon.vue';
import { categoryColor } from '../shared/category-color';
import { useDelayedLoading } from '../shared/use-delayed-loading';

type Page = 'library' | 'category' | 'avatar' | 'free' | 'import' | 'settings';

const emit = defineEmits<{ theme: [dark: boolean] }>();
const message = useMessage();

// 资料库启动状态与持久化偏好。
const data = ref<Bootstrap | null>(null);
const preferences = ref<Preferences>({
  language: 'system',
  theme: 'system',
  view: 'cards',
  cardSize: 'medium',
  query: createQuery(),
});
const page = ref<Page>('library');
const maintenance = ref(false);
const loading = ref(true);
const showLoading = useDelayedLoading(loading);
const categoriesExpanded = ref(false);
const visibleCategories = computed(() => {
  const categories = data.value?.categories || [];
  return categoriesExpanded.value ? categories : categories.slice(0, 5);
});
const hiddenCategoryCount = computed(() => Math.max(0, (data.value?.categories.length || 0) - 5));
const startupError = ref('');
const saveError = ref('');
const notice = ref('');
const version = ref(0);
const libraryGeneration = ref(0);

// 素材列表、详情和编辑器共享的导航状态。
const editorOpen = ref(false);
const editorMounted = ref(false);
const editorGeneration = ref(0);
const editorAsset = ref<Asset | null>(null);
const detail = ref<Asset | null>(null);
const detailOpen = ref(false);
const library = ref<InstanceType<typeof AssetLibrary> | null>(null);

const media = window.matchMedia('(prefers-color-scheme: dark)');
const activeCategory = computed(() =>
  preferences.value.query.uncategorized
    ? 'uncategorized'
    : preferences.value.query.categoryIds.length === 1
      ? preferences.value.query.categoryIds[0]
      : null,
);

// 串行保存偏好；恢复资料库时用 epoch 拒绝旧队列的回写。
let prefsTimer: ReturnType<typeof setTimeout> | undefined;
let disposeNotice: (() => void) | undefined;
let disposed = false;
let preferenceQueue = Promise.resolve();
let preferenceEpoch = 0;
let resettingLibrary = false;

function theme() {
  const dark =
    preferences.value.theme === 'dark' || (preferences.value.theme === 'system' && media.matches);
  emit('theme', dark);
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
}

async function refreshSystemLanguage() {
  if (language.value !== 'system') {
    return;
  }

  try {
    const detected = await api.refreshSystemLocale();
    if (!disposed && language.value === 'system') {
      setLanguage('system', detected);
    }
  } catch (e) {
    if (!disposed) {
      saveError.value = sourceErrorMessage(e);
    }
  }
}

watch(() => preferences.value.theme, theme, { immediate: true });

async function bootstrap(reset = false) {
  startupError.value = '';
  if (!data.value) {
    loading.value = true;
  }

  try {
    const loaded = await api.bootstrap();
    loaded.categories ||= [];
    loaded.avatarTags ||= [];
    loaded.freeTags ||= [];
    if (!data.value || reset) {
      preferences.value = {
        ...preferences.value,
        ...loaded.preferences,
        query: { ...createQuery(), ...loaded.preferences?.query },
      };
      for (const key of ['categoryIds', 'avatarIds', 'freeTagIds'] as const) {
        preferences.value.query[key] ||= [];
      }
      setLanguage(preferences.value.language || 'system', loaded.systemLocale);
    }

    data.value = loaded;
    data.value.preferences = preferences.value;
    version.value++;
  } catch (e) {
    startupError.value = sourceErrorMessage(e);
  } finally {
    loading.value = false;
  }
}

function updatePreferences(p: Preferences) {
  if (maintenance.value) {
    return;
  }

  preferences.value = p;
  const nextLanguage = p.language || 'system';
  if (nextLanguage !== language.value) {
    setLanguage(nextLanguage);
    void api.setLanguage(nextLanguage).catch((error) => {
      saveError.value = sourceErrorMessage(error);
    });
  }
  if (data.value) {
    data.value.preferences = p;
  }

  if (prefsTimer) {
    clearTimeout(prefsTimer);
  }

  const epoch = preferenceEpoch;
  prefsTimer = setTimeout(() => {
    const saved = JSON.parse(JSON.stringify(preferences.value));
    preferenceQueue = preferenceQueue
      .then(async () => {
        if (disposed || maintenance.value || epoch !== preferenceEpoch) {
          return;
        }

        await api.preferences(saved);
        if (epoch === preferenceEpoch) {
          saveError.value = '';
        }
      })
      .catch((e) => {
        if (epoch === preferenceEpoch) {
          saveError.value = `Preferences were not saved: ${sourceErrorMessage(e)}`;
        }
      });
  }, 350);
}

function setMaintenance(active: boolean) {
  if (active) {
    if (!maintenance.value) {
      preferenceEpoch++;
    }

    if (prefsTimer) {
      clearTimeout(prefsTimer);
      prefsTimer = undefined;
    }

    maintenance.value = true;
  } else if (!resettingLibrary) {
    maintenance.value = false;
  }
}

async function prepareRestore() {
  setMaintenance(true);
  await preferenceQueue;
}

// 素材操作完成后重新加载共享分类与标签计数。
async function open(id: string) {
  try {
    detail.value = await api.get(id);
    detailOpen.value = true;
  } catch (e) {
    message.error(errorMessage(e));
  }
}

function edit(asset: Asset | null = null) {
  // 快速重新打开时也创建新草稿，避免复用尚在退场的编辑器。
  editorGeneration.value++;
  editorAsset.value = asset;
  editorMounted.value = true;
  editorOpen.value = true;
  detailOpen.value = false;
}

// 保留弹层内容，待组件库播放完退场动画后再卸载。
function releaseEditor() {
  if (!editorOpen.value) {
    editorMounted.value = false;
    editorAsset.value = null;
  }
}

function releaseDetail() {
  if (!detailOpen.value) {
    detail.value = null;
  }
}

async function saved() {
  editorOpen.value = false;
  await bootstrap();
}

async function changed() {
  detailOpen.value = false;
  await bootstrap();
}

async function restored() {
  resettingLibrary = true;

  try {
    detail.value = null;
    detailOpen.value = false;
    editorOpen.value = false;
    editorMounted.value = false;
    editorAsset.value = null;
    await preferenceQueue;
    libraryGeneration.value++;
    await bootstrap(true);
    page.value = 'library';
  } finally {
    resettingLibrary = false;
    maintenance.value = false;
  }
}

function navigate(to: Page) {
  if (!maintenance.value) {
    page.value = to;
  }
}

async function chooseCategory(id: string | null) {
  if (maintenance.value) {
    return;
  }

  page.value = 'library';
  await nextTick();
  library.value?.chooseCategory(id);
}

async function browse(kind: string, id: string) {
  page.value = 'library';
  await nextTick();
  if (kind === 'category') {
    library.value?.chooseCategory(id);
  } else {
    library.value?.browseTag(kind, id);
  }
}

async function keyboard(event: KeyboardEvent) {
  if (maintenance.value) {
    return;
  }

  if (event.ctrlKey && (event.key.toLowerCase() === 'n' || event.key.toLowerCase() === 'f')) {
    event.preventDefault();
    if (editorOpen.value) {
      return;
    }

    if (event.key.toLowerCase() === 'n') {
      edit();
    } else {
      page.value = 'library';
      await nextTick();
      library.value?.focusSearch();
    }
  }
}

// 注册与销毁全局监听，避免组件卸载后继续响应任务或快捷键。
onMounted(() => {
  void bootstrap();
  document.addEventListener('keydown', keyboard);
  window.addEventListener('languagechange', refreshSystemLanguage);
  media.addEventListener('change', theme);
  const runtime = (
    window as unknown as {
      runtime?: {
        EventsOn?: (name: string, callback: (event: { message: string }) => void) => () => void;
        EventsOff?: (name: string) => void;
      };
    }
  ).runtime;
  if (runtime?.EventsOn) {
    const off = runtime.EventsOn('library:notice', (event) => {
      if (disposed) {
        return;
      }

      notice.value = event?.message || String(event);
      message.warning(localize(notice.value), { duration: 7000 });
    });
    disposeNotice = typeof off === 'function' ? off : () => runtime.EventsOff?.('library:notice');
  }
});

onBeforeUnmount(() => {
  disposed = true;
  document.removeEventListener('keydown', keyboard);
  window.removeEventListener('languagechange', refreshSystemLanguage);
  media.removeEventListener('change', theme);
  disposeNotice?.();
  if (prefsTimer) {
    clearTimeout(prefsTimer);
  }
});
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar" :class="{ maintenance }">
      <div class="brand">
        <img class="brand-mark" :src="appIcon" alt="" width="42" height="42" draggable="false" />
        <div>
          <strong>{{ t('VRC Asset Box') }}</strong>
        </div>
      </div>
      <div class="sidebar-content">
        <button
          class="nav-item main-nav"
          :class="{ active: page === 'library' && !activeCategory }"
          @click="chooseCategory(null)"
        >
          <NavIcon name="library" />{{ t('All assets') }}<span class="nav-arrow">›</span>
        </button>
        <div class="nav-section-heading">
          <span>{{ t('Asset categories') }}</span>
          <button :title="t('Manage categories')" @click="navigate('category')">{{ t('Manage') }}</button>
        </div>
        <button
          class="nav-item category-nav"
          :class="{ active: page === 'library' && activeCategory === 'uncategorized' }"
          @click="chooseCategory('uncategorized')"
        >
          <span class="category-dot dot-empty"></span>{{ t('Uncategorized') }}
        </button>
        <button
          v-for="category in visibleCategories"
          :key="category.id"
          class="nav-item category-nav"
          :class="{ active: page === 'library' && activeCategory === category.id }"
          @click="chooseCategory(category.id)"
        >
          <span class="category-dot" :style="{ backgroundColor: categoryColor(category.id) }"></span>
          {{ category.name }}
          <span class="nav-count">{{ category.count }}</span>
        </button>
        <button
          v-if="hiddenCategoryCount"
          class="nav-item category-toggle"
          :aria-expanded="categoriesExpanded"
          @click="categoriesExpanded = !categoriesExpanded"
        >
          <svg viewBox="0 0 20 20" aria-hidden="true" :class="{ expanded: categoriesExpanded }">
            <path d="m5 7.5 5 5 5-5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
          {{ categoriesExpanded ? t('Show less') : t('Show more ({p0})', { p0: hiddenCategoryCount }) }}
        </button>

        <div class="nav-section-heading"><span>{{ t('Asset tags') }}</span></div>
        <button class="nav-item" :class="{ active: page === 'avatar' }" @click="navigate('avatar')">
          <NavIcon name="avatar" />{{ t('Avatar tags') }}</button>
        <button class="nav-item" :class="{ active: page === 'free' }" @click="navigate('free')">
          <NavIcon name="tag" />{{ t('Custom tags') }}</button>
      </div>
      <div class="sidebar-bottom">
        <button
          class="nav-item"
          :class="{ active: page === 'settings' || page === 'import' }"
          @click="navigate('settings')"
        >
          <NavIcon name="settings" />{{ t('Settings') }}</button>
      </div>
    </aside>
    <main class="main-content">
      <div v-if="notice" class="global-notice">
        <n-alert type="warning" closable @close="notice = ''">{{ localize(notice) }}</n-alert>
      </div>
      <div v-if="saveError" class="global-notice">
        <n-alert type="error" closable @close="saveError = ''">{{ localize(saveError) }}</n-alert>
      </div>
      <div v-if="loading" class="app-loading">
        <template v-if="showLoading">
          <n-spin size="large" />
          <p>{{ t('Loading library…') }}</p>
        </template>
      </div>
      <div v-else-if="!data" class="app-loading">
        <n-empty :description="t('Unable to open library')" />
        <n-alert v-if="startupError" type="error">{{ localize(startupError) }}</n-alert>
        <n-button type="primary" @click="bootstrap()">{{ t('Reconnect') }}</n-button>
      </div>
      <template v-else>
        <n-alert v-if="startupError" type="error" style="margin-bottom: 20px">
          {{ localize(startupError) }}
          <n-button text @click="bootstrap()">{{ t('Retry') }}</n-button>
        </n-alert>
        <div v-if="page !== 'import'" :key="page" class="page-content page-enter">
          <AssetLibrary
            v-if="page === 'library'"
            ref="library"
            :data="data"
            :preferences="preferences"
            :version="version"
            @open="open"
            @edit="edit"
            @create="edit()"
            @preferences="updatePreferences"
            @changed="changed"
          />
          <TaxonomyPage
            v-else-if="page === 'category' || page === 'avatar' || page === 'free'"
            :key="page"
            :kind="page"
            :data="data"
            @changed="changed"
            @browse="browse"
          />
          <SettingsPage
            v-if="page === 'settings'"
            :preferences="preferences"
            @preferences="updatePreferences"
            @import="navigate('import')"
            @restored="restored"
            :before-restore="prepareRestore"
            @maintenance="setMaintenance"
          />
        </div>
        <div v-show="page === 'import'" class="page-content" :class="{ 'page-enter': page === 'import' }">
          <ImportWizard
            :key="libraryGeneration"
            :data="data"
            @changed="changed"
            @back="navigate('settings')"
          />
        </div>
      </template>
    </main>
    <AssetDetail
      v-if="detail && data"
      :asset="detail"
      :show="detailOpen"
      :data="data"
      @close="detailOpen = false"
      @after-leave="releaseDetail"
      @edit="edit"
      @deleted="changed"
    />
    <AssetEditor
      v-if="editorMounted && data"
      :key="editorGeneration"
      :show="editorOpen"
      :asset="editorAsset"
      :data="data"
      @close="editorOpen = false"
      @after-leave="releaseEditor"
      @saved="saved"
      @open-existing="open"
    />
  </div>
</template>
