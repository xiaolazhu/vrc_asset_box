import { beforeEach } from 'vitest';
import { setLanguage } from './index';

// 既有组件回归固定使用简体中文，语言匹配和切换由专门的测试覆盖。
beforeEach(() => setLanguage('zh-CN'));
