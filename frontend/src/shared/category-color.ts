// 分类 ID 决定颜色；新增、改名或排序不会改变已有分类的颜色。
export function categoryColor(id: string): string {
  let hash = 2166136261;
  for (const character of id) {
    hash = Math.imul(hash ^ character.codePointAt(0)!, 16777619);
  }

  const value = hash >>> 0;
  // 限制在柔和色系中，再轻微改变色相和明度，避免高饱和色和固定周期循环。
  const hues = [155, 205, 260, 30, 180, 345, 75, 230];
  const hue = hues[value % hues.length] + ((value >>> 3) % 100) / 10 - 5;
  const saturation = 18 + ((value >>> 12) % 100) / 10;
  const lightness = 52 + ((value >>> 20) % 80) / 10;
  return `hsl(${hue} ${saturation}% ${lightness}%)`;
}
