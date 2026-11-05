<script setup lang="ts">
import { computed } from 'vue'
import { formatTime, type SmsCardModel } from '../lib/model'

/*
 * SmsCard：短信卡片完整字段渲染（FORMAL-UI-MAIL-PLAN §2.2）——
 * 来源设备、SIM 显示名与「号码未确认」语义、发送方、正文缩略、时间、收发状态。
 * 正式列表暂不渲染卡片（正式同步未接通）；卡片目前只出现在 /dev/synthetic，
 * demo 为 true 时必须显示「演示数据」标记。
 */
const props = defineProps<{ record: SmsCardModel; demo?: boolean }>()

const emit = defineEmits<{ open: [] }>()

const simText = computed(() => (
  props.record.simNumber === null
    ? `${props.record.simLabel} · 号码未确认`
    : `${props.record.simLabel} · ${props.record.simNumber}`
))
const directionText = computed(() => (props.record.direction === 'received' ? '收到' : '发出'))
</script>

<template>
  <article class="msg-card" data-testid="demo-sms-card" @click="emit('open')">
    <div class="msg-card-top">
      <span class="msg-card-primary">{{ record.counterpart }}</span>
      <span class="msg-card-direction" :data-direction="record.direction">{{ directionText }}</span>
      <time class="msg-card-time" :datetime="record.time">{{ formatTime(record.time) }}</time>
    </div>
    <p class="msg-card-snippet">{{ record.snippet }}</p>
    <div class="msg-card-meta">
      <span class="msg-card-chip">{{ record.deviceName }}</span>
      <span class="msg-card-chip" :data-unconfirmed="record.simNumber === null">{{ simText }}</span>
      <span class="msg-card-chip" data-testid="sms-status">{{ record.status }}</span>
      <span v-if="demo" class="demo-badge">演示数据</span>
    </div>
  </article>
</template>
