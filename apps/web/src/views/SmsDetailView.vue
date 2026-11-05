<script setup lang="ts">
import { useRoute } from 'vue-router'
import SmsDetail from '../components/SmsDetail.vue'

/** 短信详情路由页：正式数据未接通，详情组件以「记录不存在或未同步」呈现，并显示请求 ID。 */
const route = useRoute()
const requestId = typeof route.params.id === 'string' ? route.params.id : ''
</script>

<template>
  <div class="page-scroll detail-page">
    <RouterLink class="detail-back" to="/sms" aria-label="返回短信列表">← 返回短信列表</RouterLink>
    <SmsDetail :request-id="requestId" />
  </div>
</template>
