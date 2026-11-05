<script setup lang="ts">
import { computed } from 'vue'
import StateBlock from './StateBlock.vue'
import { formatTime, type SmsDetailModel } from '../lib/model'

/*
 * SmsDetail：短信详情完整布局（FORMAL-UI-MAIL-PLAN §2.3）——
 * 完整来源（设备、SIM、号码/未确认语义、发送方）、时间、内容区、附件区、状态轨迹。
 * record 缺省时渲染「记录不存在或未同步」。demo 为 true 时显示演示数据标记。
 */
const props = defineProps<{ record?: SmsDetailModel; requestId?: string; demo?: boolean }>()

const notFoundDetail = computed(() => (
  props.requestId
    ? `此记录尚未通过正式链路同步到本客户端。请求的记录 ID：${props.requestId}`
    : '此记录尚未通过正式链路同步到本客户端。'
))

const simLine = computed(() => {
  const record = props.record
  if (!record) return ''
  return record.simNumber === null
    ? `${record.simLabel}（号码未确认）`
    : `${record.simLabel}（${record.simNumber}）`
})
</script>

<template>
  <StateBlock
    v-if="!record"
    variant="notfound"
    :detail="notFoundDetail"
  />
  <article v-else class="detail">
    <header class="detail-head">
      <h1 class="detail-title">短信详情</h1>
      <p v-if="demo" class="demo-banner" data-testid="detail-demo-banner">演示数据 · 非真实会话</p>
      <dl class="detail-source">
        <div class="detail-source-row"><dt>来源设备</dt><dd>{{ record.deviceName }}</dd></div>
        <div class="detail-source-row"><dt>SIM</dt><dd data-testid="detail-sim">{{ simLine }}</dd></div>
        <div class="detail-source-row"><dt>{{ record.direction === 'received' ? '发送方' : '收件方' }}</dt><dd>{{ record.counterpart }}</dd></div>
        <div class="detail-source-row"><dt>时间</dt><dd><time :datetime="record.time">{{ formatTime(record.time) }}</time></dd></div>
        <div class="detail-source-row"><dt>收发状态</dt><dd>{{ record.status }}</dd></div>
      </dl>
    </header>

    <section class="detail-body" aria-label="内容">
      <p class="detail-text">{{ record.body }}</p>
    </section>

    <section class="detail-section" aria-label="附件">
      <h2>附件</h2>
      <p class="detail-empty-line">短信不包含附件。</p>
    </section>

    <section class="detail-section" aria-label="状态轨迹">
      <h2>状态轨迹</h2>
      <ol class="detail-timeline">
        <li v-for="(entry, index) in record.timeline" :key="index">
          <span class="timeline-state">{{ entry.state }}</span>
          <time :datetime="entry.at">{{ formatTime(entry.at) }}</time>
          <span v-if="entry.note" class="timeline-note">{{ entry.note }}</span>
        </li>
      </ol>
    </section>
  </article>
</template>
