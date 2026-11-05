<script setup lang="ts">
import { formatTime, type MailCardModel } from '../lib/model'

/*
 * MailCard：邮件卡片完整字段渲染（FORMAL-UI-MAIL-PLAN §2.2）——
 * 收件账号、发件人、主题、正文缩略、时间、附件标记、未读状态。
 * 正式列表暂不渲染卡片（正式同步未接通）；卡片目前只出现在 /dev/synthetic，
 * demo 为 true 时必须显示「演示数据」标记。
 */
defineProps<{ record: MailCardModel; demo?: boolean }>()

const emit = defineEmits<{ open: [] }>()
</script>

<template>
  <article class="msg-card" :class="{ 'is-unread': record.unread }" data-testid="demo-mail-card" @click="emit('open')">
    <div class="msg-card-top">
      <span v-if="record.unread" class="unread-dot" aria-label="未读"></span>
      <span class="msg-card-primary">{{ record.from }}</span>
      <time class="msg-card-time" :datetime="record.time">{{ formatTime(record.time) }}</time>
    </div>
    <p class="msg-card-subject">{{ record.subject }}</p>
    <p class="msg-card-snippet">{{ record.snippet }}</p>
    <div class="msg-card-meta">
      <span class="msg-card-chip">收件账号：{{ record.account }}</span>
      <span v-if="record.hasAttachment" class="msg-card-chip" data-testid="mail-attachment-flag">附件</span>
      <span class="msg-card-chip" data-testid="mail-unread-state">{{ record.unread ? '未读' : '已读' }}</span>
      <span v-if="demo" class="demo-badge">演示数据</span>
    </div>
  </article>
</template>
