<script setup lang="ts">
import { computed } from 'vue'

/*
 * StateBlock：加载、空态、断网、失败、未接通、未找到的统一空状态组件。
 * 每个状态有默认文案，可用 title/detail 覆盖，并可带一个动作链接。
 */
const props = defineProps<{
  variant: 'loading' | 'empty' | 'offline' | 'error' | 'locked' | 'notfound'
  title?: string
  detail?: string
  actionLabel?: string
  actionTo?: string
}>()

const DEFAULTS: Record<'loading' | 'empty' | 'offline' | 'error' | 'locked' | 'notfound', { title: string; glyph: string }> = {
  loading: { title: '正在加载…', glyph: '…' },
  empty: { title: '暂无内容', glyph: '—' },
  offline: { title: '网络连接不可用', glyph: '⌁' },
  error: { title: '加载失败', glyph: '!' },
  locked: { title: '正式同步未接通', glyph: '·' },
  notfound: { title: '记录不存在或未同步', glyph: '∅' },
}

const resolvedTitle = computed(() => props.title ?? DEFAULTS[props.variant].title)
const glyph = computed(() => DEFAULTS[props.variant].glyph)
</script>

<template>
  <div class="state-block" :data-variant="variant" :aria-busy="variant === 'loading' ? 'true' : undefined">
    <span class="state-glyph" aria-hidden="true">{{ glyph }}</span>
    <p class="state-title">{{ resolvedTitle }}</p>
    <p v-if="detail" class="state-detail">{{ detail }}</p>
    <RouterLink v-if="actionLabel && actionTo" class="state-action" :to="actionTo">{{ actionLabel }}</RouterLink>
  </div>
</template>
