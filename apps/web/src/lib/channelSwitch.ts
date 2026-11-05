import { ref } from 'vue'

/*
 * 频道切换（FORMAL-UI-MAIL-PLAN §2.2）：
 * 点击顶部标签或列表内容区左右滑动均可切换，选中背景与内容同步滑移约 250ms；
 * prefers-reduced-motion 或系统关闭动画时取消位移。侧栏、输入框、详情页不触发滑动切换
 * （手势只绑定在列表 Pager 视口上，输入框位于 Pager 之外）。
 */
export const SWIPE_THRESHOLD_PX = 64

/** 滑动进行中的分数位置：0 = 短信，1 = 邮件；未滑动时为 null。 */
export const swipePosition = ref<number | null>(null)

export function prefersReducedMotion(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
}
