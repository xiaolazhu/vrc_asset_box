import { afterEach, describe, expect, it } from 'vitest';
import { resolveLocale, setLanguage, t, localize, locale, messages } from './index';

afterEach(() => setLanguage('zh-CN'));

describe('应用语言', () => {
  it('以英文作为消息键和翻译基准', () => {
    for (const key of Object.keys(messages.en)) {
      expect(key, key).not.toMatch(/[\u3400-\u9fff]/);
      expect(messages.en[key]).toBe(key);
    }
    setLanguage('ja');
    expect(t('ID copied')).toBe('ID をコピーしました');
    setLanguage('zh-CN');
    expect(t('ID copied')).toBe('ID 已复制');
  });

  it('中文统一称为素体，保留具体素体的英文名称', () => {
    setLanguage('zh-CN');
    expect(t('Avatar tags')).toBe('素体标签');
    expect(t('Avatar')).toBe('素体');
    expect(t('Compatible avatars')).toBe('适配素体');
    expect(localize('Shinano')).toBe('Shinano');
    setLanguage('zh-TW');
    expect(t('Avatar tags')).toBe('素體標籤');
  });

  it('区分简繁中文，未知系统语言回退英文', () => {
    expect(resolveLocale('system', ['zh-Hant-HK'])).toBe('zh-TW');
    expect(resolveLocale('system', ['zh-SG'])).toBe('zh-CN');
    expect(resolveLocale('system', ['ko-KR'])).toBe('ko');
    expect(resolveLocale('system', ['fr-FR'])).toBe('en');
    expect(resolveLocale('ja', ['zh-CN'])).toBe('ja');
  });

  it('即时切换语言并保留插值中的用户数据', () => {
    setLanguage('en');
    expect(locale.value).toBe('en');
    expect(t('Settings')).toBe('Settings');
    expect(t('Report saved: {p0}', { p0: 'Z:/我的素材/report.csv' })).toContain('Z:/我的素材/report.csv');
    expect(localize('我的素材名称')).toBe('我的素材名称');
    expect(localize('Images must not exceed 20 MiB')).toBe('Images must not exceed 20 MiB');
    expect(t('{p0} assets', { p0: 12345 })).toContain('12,345');
  });

  it('所有语言的文案键与插值保持完整', () => {
    const reference = messages.en;
    for (const [language, dictionary] of Object.entries(messages)) {
      expect(Object.keys(dictionary).sort(), language).toEqual(Object.keys(reference).sort());
      for (const key of Object.keys(reference)) {
        expect(dictionary[key], `${language}: ${key}`).toBeTruthy();
        const params = (value: string) => (value.match(/\{p\d+\}/g) || []).sort();
        expect(params(dictionary[key]), `${language}: ${key}`).toEqual(params(reference[key]));
      }
    }
  });

  it('翻译包装错误而不改写标签、工作表名和路径', () => {
    setLanguage('en');
    expect(localize('New tag needs confirmation: 素材不存在')).toBe('New tag needs confirmation: 素材不存在');
    expect(localize('素材不存在 row 3: Asset not found')).toBe('素材不存在 row 3: Asset not found');
    expect(localize('Invalid cover path: Z:/素材不存在/@alice|dress.png')).toContain(
      'Z:/素材不存在/@alice|dress.png',
    );
    expect(localize('Preferences were not saved: Asset not found'))
      .toBe('Preferences were not saved: Asset not found');
    expect(localize('Automatic backup failed: Asset not found. Check disk space and permissions, then export a backup manually.'))
      .toContain('Automatic backup failed: Asset not found');
    setLanguage('ja');
    expect(localize('Backup before import failed: Asset not found'))
      .toBe(t('Backup before import failed: {p0}', { p0: t('Asset not found') }));
    expect(localize('Invalid cover path: Z:/素材不存在/@alice|dress.png'))
      .toBe('カバー画像のパスが無効です：Z:/素材不存在/@alice|dress.png');
    expect(localize('素材不存在')).toBe('素材不存在');
  });

  it('繁体、日文和韩文均生效，纯文本符号不参与消息语法', () => {
    for (const language of ['zh-TW', 'ja', 'ko'] as const) {
      setLanguage(language);
      expect(t('Settings')).toBe(messages[language]['Settings']);
      expect(document.documentElement.lang).toBe(language);
      expect(t('Report saved: {p0}', { p0: '@alice | {dress}' })).toContain('@alice | {dress}');
    }
  });

  it('系统语言变化只影响跟随系统模式', () => {
    setLanguage('system', 'zh-TW');
    setLanguage('system', 'ja-JP');
    expect(locale.value).toBe('ja');
    setLanguage('ko', 'zh-CN');
    expect(locale.value).toBe('ko');
  });
});
