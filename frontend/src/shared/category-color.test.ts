import { describe, expect, it } from 'vitest';
import { categoryColor } from './category-color';

describe('分类颜色', () => {
  it('按分类 ID 稳定生成颜色，重排后保持一致，且不再五项循环', () => {
    const ids = Array.from({ length: 30 }, (_, index) => `category-${index}`);
    const original = new Map(ids.map(id => [id, categoryColor(id)]));

    for (const id of [...ids].reverse()) {
      expect(categoryColor(id)).toBe(original.get(id));
    }
    expect(new Set(original.values()).size).toBe(30);
    expect(categoryColor(ids[0])).not.toBe(categoryColor(ids[5]));
  });
});
