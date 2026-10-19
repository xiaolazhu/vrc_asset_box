import { computed, ref } from 'vue';
import { createI18n } from 'vue-i18n';
import english from './locales/en.json';

export type Locale = 'en' | 'zh-CN' | 'zh-TW' | 'ja' | 'ko';
export type Language = 'system' | Locale;
export type Dictionary = Record<string, string>;
type Message = (context: { named: (name: string) => unknown }) => string;

const resources = import.meta.glob('./locales/*.json', { eager: true, import: 'default' });
export const messages: Record<Locale, Dictionary> = {
  en: english,
  'zh-CN': resources['./locales/zh-CN.json'] as Dictionary,
  'zh-TW': resources['./locales/zh-TW.json'] as Dictionary,
  ja: resources['./locales/ja.json'] as Dictionary,
  ko: resources['./locales/ko.json'] as Dictionary,
};

// 预编译为纯文本消息函数，路径中的 @、| 等字符不会被解析为消息语法。
function compileDictionary(dictionary: Dictionary, language: string): Record<string, Message> {
  const numberFormat = new Intl.NumberFormat(language);

  return Object.fromEntries(Object.entries(dictionary).map(([key, source]) => [
    key,
    (context: Parameters<Message>[0]) => source.replace(/\{(p\d+)\}/g, (_, name: string) => {
      const value = context.named(name);
      return typeof value === 'number' ? numberFormat.format(value) : String(value ?? '');
    }),
  ]));
}

const compiledMessages = Object.fromEntries(
  Object.entries(messages).map(([language, dictionary]) => [
    language,
    compileDictionary(dictionary, language),
  ]),
);

export const i18n = createI18n({
  legacy: false,
  locale: 'en',
  fallbackLocale: 'en',
  missingWarn: false,
  fallbackWarn: false,
  messages: compiledMessages,
  messageResolver: (dictionary, key) =>
    (dictionary as Record<string, Message>)[key] ?? null,
});

export const locale = computed(() => i18n.global.locale.value as Locale);
export const language = ref<Language>('system');
const systemLocale = ref('');
const supported = new Set<Language>(['system', 'en', 'zh-CN', 'zh-TW', 'ja', 'ko']);

export function resolveLocale(preference: string, systemLanguages: readonly string[]): Locale {
  if (preference !== 'system' && supported.has(preference as Language)) {
    return preference as Locale;
  }

  for (const entry of systemLanguages) {
    const normalized = entry.toLowerCase().replace(/_/g, '-');
    if (/^zh(?:-|$)/.test(normalized)) {
      return /(?:^|-)(hant|tw|hk|mo)(?:-|$)/.test(normalized) ? 'zh-TW' : 'zh-CN';
    }
    if (/^ja(?:-|$)/.test(normalized)) {
      return 'ja';
    }
    if (/^ko(?:-|$)/.test(normalized)) {
      return 'ko';
    }
    if (/^en(?:-|$)/.test(normalized)) {
      return 'en';
    }
  }

  return 'en';
}

export function setLanguage(preference: string = 'system', system?: string): void {
  language.value = supported.has(preference as Language) ? preference as Language : 'system';
  if (system !== undefined) {
    systemLocale.value = system;
  }

  const detected = systemLocale.value
    ? [systemLocale.value]
    : typeof navigator === 'undefined'
      ? []
      : navigator.languages || [navigator.language];
  i18n.global.locale.value = resolveLocale(language.value, detected);
  if (typeof document !== 'undefined') {
    document.documentElement.lang = locale.value;
  }
}

export function t(key: string, params: Record<string, unknown> = {}): string {
  // 显式读取响应式语言，使普通函数在 computed 与模板中也能随切换更新。
  void locale.value;
  return i18n.global.t(key, params);
}

// 与 Go 输出格式对应；%v、%w 参数是嵌套错误，其他参数保持原文。
const formattedSources = [
  "Text in row %d exceeds the Excel cell length limit",
  "%s row %d: %w",
  "%s!%s: Cannot read richData image: %v",
  "%s!%s: Image metadata is invalid or unsupported. Confirm importing without an image or skip this row.",
  "%s!%s: Invalid string index",
  "Booth returned HTTP %d. Try again later or enter details manually.",
  "Unsupported absoluteAnchor image (%s) cannot be located in a cell. No associated row was inferred. Confirm importing without an image or skip this row.",
  "Failed to preserve data from the interrupted restore: %w",
  "Invalid cover path: %s",
  "Unsupported image format: %v",
  "Failed to read image: %v",
  "Image spans rows %d to %d and is shown on its starting row. Confirm its association or skip it.",
  "Backup database version %d is unsupported",
  "Incompatible backup database schema: %s",
  "Backup is missing %s",
  "Backup is missing a referenced cover: %s",
  "Backup before import failed: %w",
  "Workbook is missing %s",
  "Sheet %s is missing relationships",
  "Cannot read sheet image relationships or determine affected rows: %v",
  "Current library validation failed: %w",
  "Invalid restore folder: %s",
  "Restore log exists but no complete library was found. Staging directory preserved: %s",
  "Failed to restore the previous library: %w",
  "Database integrity check failed: %s",
  "File content exceeds the limit: %s",
  "File content is too large: %s",
  "File integrity check failed: %s",
  "Cannot read XLSX: %w",
  "Previous library validation failed. All data has been preserved: %w",
  "Cannot import local image path: %v",
  "Manifest does not list %s",
  "Failed to clear restore log: %w",
  "Drawing image on row %d is missing or unsupported (%s, %s). Confirm importing without an image or skip this row.",
  "Library version %d is newer than this app supports and cannot be opened",
  "Library version %d is newer than this app supports and cannot be migrated",
  "Drawing image is on empty row %d (%s). No other associated row was inferred.",
  "Cannot read drawing image relationships (%s) or determine affected rows: %v",
  "Drawing image relationships are missing (%s); affected rows cannot be determined",
  "Cannot parse drawing image (%s) or determine affected rows: %v",
  "Drawing image has no valid starting point (%s); affected rows cannot be determined. No associated row was inferred.",
  "Image in header area: %s",
  "Failed to read restore log: %w",
];

const formattedMessages = formattedSources.map(source => {
  const tokens = source.match(/%[+\-#0-9.]*[sdvwq]/g) || [];
  let index = 0;
  const key = source.replace(/%[+\-#0-9.]*[sdvwq]/g, () => `{p${index++}}`);
  const pattern = source.split(/%[+\-#0-9.]*[sdvwq]/g)
    .map(part => part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
    .join('([\\s\\S]+?)');
  return { key, tokens, pattern: new RegExp(`^${pattern}$`) };
});

// 只对应用提示调用此适配器，素材名称、备注、标签和路径直接显示原文。
export function localize(message: string, depth = 0): string {
  if (!message || depth > 5) {
    return message;
  }
  if (Object.prototype.hasOwnProperty.call(english, message)) {
    return t(message);
  }

  // 前缀后的标签名保持原样，仅嵌套的应用错误继续翻译。
  const wrappedMessages = [
    { prefix: 'Preferences were not saved: ', suffix: '', key: 'Preferences were not saved: {p0}', nested: true },
    { prefix: 'Failed to read merge target: ', suffix: '', key: 'Failed to read merge target: {p0}', nested: true },
    { prefix: 'New tag needs confirmation: ', suffix: '', key: 'New tag needs confirmation: {p0}', nested: false },
    {
      prefix: 'Automatic backup failed: ',
      suffix: '. Check disk space and permissions, then export a backup manually.',
      key: 'Automatic backup failed: {p0}. Check disk space and permissions, then export a backup manually.',
      nested: true,
    },
    { prefix: 'Old cover cleanup is incomplete: ', suffix: '', key: 'Old cover cleanup is incomplete: {p0}', nested: true },
    { prefix: 'This batch was not committed: ', suffix: '', key: 'This batch was not committed: {p0}', nested: true },
  ];
  for (const entry of wrappedMessages) {
    if (message.startsWith(entry.prefix) && (!entry.suffix || message.endsWith(entry.suffix))) {
      const end = entry.suffix ? -entry.suffix.length : undefined;
      const detail = message.slice(entry.prefix.length, end);
      return t(entry.key, { p0: entry.nested ? localize(detail, depth + 1) : detail });
    }
  }

  for (const entry of formattedMessages) {
    const match = message.match(entry.pattern);
    if (match) {
      const params = Object.fromEntries(match.slice(1).map((value, index) => [
        `p${index}`, /[vw]$/.test(entry.tokens[index]) ? localize(value, depth + 1) : value,
      ]));
      return t(entry.key, params);
    }
  }

  if (message.includes('\n')) {
    return message.split('\n').map(part => localize(part, depth + 1)).join('\n');
  }
  return message;
}

export function formatDate(value: string): string {
  if (!value) {
    return '—';
  }

  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(locale.value);
}

export function formatNumber(value: number, fractionDigits = 0): string {
  return new Intl.NumberFormat(locale.value, {
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits,
  }).format(value);
}

setLanguage('system');
