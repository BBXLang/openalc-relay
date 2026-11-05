<script setup lang="ts">
import { computed } from 'vue'
import StateBlock from './StateBlock.vue'
import { sanitizeHtml } from '../lib/sanitizeHtml'
import { formatSize, formatTime, type MailDetailModel } from '../lib/model'

/*
 * MailDetail：邮件详情完整布局（FORMAL-UI-MAIL-PLAN §2.3）——
 * 完整来源（收件账号、发件人）、时间、内容区（HTML 一律先过 sanitizeHtml 保守白名单净化，
 * 远程图片默认不加载）、附件列表（附件只在用户点击后下载，正式链路未接通前禁用）、状态轨迹。
 * record 缺省时渲染「记录不存在或未同步」。demo 为 true 时显示演示数据标记。
 */
const props = defineProps<{ record?: MailDetailModel; requestId?: string; demo?: boolean }>()

const notFoundDetail = computed(() => (
  props.requestId
    ? `此记录尚未通过正式链路同步到本客户端。请求的记录 ID：${props.requestId}`
    : '此记录尚未通过正式链路同步到本客户端。'
))

const safeHtml = computed(() => sanitizeHtml(props.record?.htmlBody ?? ''))
</script>

<template>
  <StateBlock
    v-if="!record"
    variant="notfound"
    :detail="notFoundDetail"
  />
  <article v-else class="detail">
    <header class="detail-head">
      <h1 class="detail-title">邮件详情</h1>
      <p v-if="demo" class="demo-banner" data-testid="detail-demo-banner">演示数据 · 非真实会话</p>
      <dl class="detail-source">
        <div class="detail-source-row"><dt>收件账号</dt><dd>{{ record.account }}</dd></div>
        <div class="detail-source-row"><dt>发件人</dt><dd>{{ record.from }}</dd></div>
        <div class="detail-source-row"><dt>主题</dt><dd>{{ record.subject }}</dd></div>
        <div class="detail-source-row"><dt>时间</dt><dd><time :datetime="record.time">{{ formatTime(record.time) }}</time></dd></div>
        <div class="detail-source-row"><dt>状态</dt><dd>{{ record.unread ? '未读' : '已读' }}</dd></div>
      </dl>
    </header>

    <section class="detail-body" aria-label="内容">
      <div
        v-if="record.htmlBody"
        class="detail-html"
        data-testid="mail-html-body"
        v-html="safeHtml"
      ></div>
      <p v-else class="detail-text">{{ record.textBody ?? record.snippet }}</p>
      <p class="detail-note">HTML 邮件经保守白名单净化；远程图片默认不加载。</p>
    </section>

    <section class="detail-section" aria-label="附件">
      <h2>附件</h2>
      <p v-if="record.attachments.length === 0" class="detail-empty-line">无附件。</p>
      <ul v-else class="detail-attachments">
        <li v-for="attachment in record.attachments" :key="attachment.name">
          <span class="attachment-name">{{ attachment.name }}</span>
          <span class="attachment-size">{{ formatSize(attachment.sizeBytes) }}</span>
          <button
            type="button"
            class="attachment-download"
            disabled
            title="附件下载未接通：正式链路未接通。附件只在用户点击后下载。"
            aria-label="下载附件：正式链路未接通，暂不可用"
          >下载</button>
        </li>
      </ul>
      <p class="detail-note">附件只在用户点击后下载，不会自动打开或执行。</p>
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
