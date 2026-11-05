<script setup lang="ts">
import { useRoute } from 'vue-router'
import MailDetail from '../components/MailDetail.vue'

/** 邮件详情路由页：正式数据未接通，详情组件以「记录不存在或未同步」呈现，并显示请求 ID。 */
const route = useRoute()
const requestId = typeof route.params.id === 'string' ? route.params.id : ''
</script>

<template>
  <div class="page-scroll detail-page">
    <RouterLink class="detail-back" to="/mail" aria-label="返回邮件列表">← 返回邮件列表</RouterLink>
    <MailDetail :request-id="requestId" />
  </div>
</template>
