<script setup lang="ts">
import {
  NConfigProvider,
  NDialogProvider,
  NMessageProvider,
  darkTheme,
  zhCN,
  dateZhCN,
  zhTW,
  dateZhTW,
  enUS,
  dateEnUS,
  jaJP,
  dateJaJP,
  koKR,
  dateKoKR,
} from 'naive-ui';
import { computed, ref } from 'vue';
import AppShell from './app/AppShell.vue';
import { locale } from './i18n';

const isDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches);
const componentLocale = computed(() => ({
  en: enUS, 'zh-CN': zhCN, 'zh-TW': zhTW, ja: jaJP, ko: koKR,
})[locale.value]);
const componentDateLocale = computed(() => ({
  en: dateEnUS, 'zh-CN': dateZhCN, 'zh-TW': dateZhTW, ja: dateJaJP, ko: dateKoKR,
})[locale.value]);
const overrides = {
  common: {
    primaryColor: '#168b79',
    primaryColorHover: '#20a18c',
    primaryColorPressed: '#10715f',
    borderRadius: '8px',
    fontFamily: '"Segoe UI", "Microsoft YaHei", sans-serif',
  },
};
</script>

<template>
  <n-config-provider
    :theme="isDark ? darkTheme : null"
    :theme-overrides="overrides"
    :locale="componentLocale"
    :date-locale="componentDateLocale"
  >
    <n-message-provider>
      <n-dialog-provider><AppShell @theme="isDark = $event" /></n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
