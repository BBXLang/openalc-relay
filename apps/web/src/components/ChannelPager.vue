<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { prefersReducedMotion, swipePosition, SWIPE_THRESHOLD_PX } from '../lib/channelSwitch'
import type { Channel } from '../lib/model'

/*
 * ChannelPager：短信/邮件列表的双窗格容器。
 * - 两个窗格常驻（切换频道不重挂载，滚动位置与状态保留）；
 * - 选中内容随频道切换滑移约 250ms（tokens.css --motion-duration）；
 * - 内容区左右滑动手势可切换频道；prefers-reduced-motion 时取消位移且禁用手势；
 * - 手势不绑定到侧栏、输入框与详情页（仅本组件视口），且手势起点落在交互控件上时忽略。
 */
const props = defineProps<{ channel: Channel }>()
const router = useRouter()

const viewport = ref<HTMLElement | null>(null)
const paneSms = ref<HTMLElement | null>(null)
const paneMail = ref<HTMLElement | null>(null)

const baseIndex = computed(() => (props.channel === 'mail' ? 1 : 0))

const trackStyle = computed(() => {
  if (swipePosition.value !== null) {
    // 滑动中：位移完全跟随手势，禁用过渡。
    return { transform: `translateX(${(-swipePosition.value * 100).toFixed(3)}%)`, transition: 'none' }
  }
  // 静止：位移由频道决定，类上的 250ms 过渡负责标签切换动画。
  return { transform: `translateX(${(-baseIndex.value * 50).toFixed(3)}%)` }
})

// 独立滚动记忆：频道切换与详情往返后恢复原列表位置。
const scrollMemory: Partial<Record<Channel, number>> = {}

function rememberScroll() {
  if (paneSms.value) scrollMemory.sms = paneSms.value.scrollTop
  if (paneMail.value) scrollMemory.mail = paneMail.value.scrollTop
}

function restoreScroll() {
  nextTick(() => {
    if (paneSms.value) paneSms.value.scrollTop = scrollMemory.sms ?? 0
    if (paneMail.value) paneMail.value.scrollTop = scrollMemory.mail ?? 0
  })
}

function switchTo(target: Channel) {
  if (target === props.channel) return
  rememberScroll()
  router.push(target === 'sms' ? '/sms' : '/mail')
}

watch(() => props.channel, restoreScroll)

// ---- 滑动手势 ----
let startX = 0
let startY = 0
let startBase = 0
let tracking = false
let swiping = false
let pointerId: number | null = null

function onPointerDown(event: PointerEvent) {
  if (prefersReducedMotion()) return
  if (event.pointerType === 'mouse' && event.button !== 0) return
  if (swipePosition.value !== null) return
  const target = event.target as Element | null
  if (target && target.closest('input, textarea, select, button, a, [role="button"]')) return
  tracking = true
  swiping = false
  startX = event.clientX
  startY = event.clientY
  startBase = baseIndex.value
  pointerId = event.pointerId
}

function onPointerMove(event: PointerEvent) {
  if (!tracking || event.pointerId !== pointerId) return
  const deltaX = event.clientX - startX
  const deltaY = event.clientY - startY
  if (!swiping) {
    // 水平位移明显大于垂直位移时才进入频道滑动，避免抢竖向滚动。
    if (Math.abs(deltaX) < 12 || Math.abs(deltaX) < Math.abs(deltaY)) return
    swiping = true
    try { viewport.value?.setPointerCapture(event.pointerId) } catch { /* 忽略捕获失败 */ }
  }
  const width = viewport.value?.clientWidth || 1
  const next = Math.min(1, Math.max(0, startBase - deltaX / width))
  swipePosition.value = next
}

function onPointerUp(event: PointerEvent) {
  if (!tracking || event.pointerId !== pointerId) return
  const wasSwiping = swiping
  const delta = event.clientX - startX
  tracking = false
  swiping = false
  pointerId = null
  if (!wasSwiping) return
  if (Math.abs(delta) >= SWIPE_THRESHOLD_PX) {
    const target: Channel = delta < 0 ? 'mail' : 'sms'
    swipePosition.value = null
    switchTo(target)
  } else {
    // 未达阈值：清除滑动进度，让内容过渡回原频道。
    swipePosition.value = null
  }
}

function onPointerCancel(event: PointerEvent) {
  if (event.pointerId !== pointerId) return
  tracking = false
  swiping = false
  pointerId = null
  swipePosition.value = null
}

onMounted(() => {
  restoreScroll()
})

onBeforeUnmount(() => {
  rememberScroll()
  swipePosition.value = null
})
</script>

<template>
  <div
    ref="viewport"
    class="pager-viewport"
    @pointerdown="onPointerDown"
    @pointermove="onPointerMove"
    @pointerup="onPointerUp"
    @pointercancel="onPointerCancel"
  >
    <div class="pager-track" :style="trackStyle">
      <section
        id="pane-sms"
        ref="paneSms"
        class="pager-pane"
        role="tabpanel"
        aria-labelledby="tab-sms"
        :inert="channel !== 'sms'"
        :aria-hidden="channel !== 'sms'"
        @scroll.passive="rememberScroll"
      >
        <slot name="sms" />
      </section>
      <section
        id="pane-mail"
        ref="paneMail"
        class="pager-pane"
        role="tabpanel"
        aria-labelledby="tab-mail"
        :inert="channel !== 'mail'"
        :aria-hidden="channel !== 'mail'"
        @scroll.passive="rememberScroll"
      >
        <slot name="mail" />
      </section>
    </div>
  </div>
</template>
