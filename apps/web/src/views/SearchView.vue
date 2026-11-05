<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import StateBlock from '../components/StateBlock.vue'
import type { Channel } from '../lib/model'

/*
 * 频道搜索页（FORMAL-UI-MAIL-PLAN §2.1 / §2.3）。
 * 搜索限定当前频道（channel ∈ sms|mail，非法值重定向 /sms），不跨频道混合结果；
 * 搜索词只保存在组件状态中，不进入 URL。正式搜索未接通：提交后显示未接通状态。
 */
const route = useRoute()
const router = useRouter()

const channel = ref<Channel>(route.params.channel === 'mail' ? 'mail' : 'sms')
watch(() => route.params.channel, value => {
  if (value !== 'sms' && value !== 'mail') {
    router.replace('/sms')
    return
  }
  channel.value = value
})
if (route.params.channel !== 'sms' && route.params.channel !== 'mail') {
  router.replace('/sms')
}

const query = ref('')
const searched = ref(false)

const channelLabel = ref<'短信' | '邮件'>('短信')
watch(channel, value => { channelLabel.value = value === 'mail' ? '邮件' : '短信' }, { immediate: true })

function submit() {
  const trimmed = query.value.trim()
  if (trimmed.length === 0) return
  searched.value = true
}

function clearSearch() {
  query.value = ''
  searched.value = false
}
</script>

<template>
  <div class="page-scroll search-page">
    <RouterLink class="detail-back" :to="channel === 'mail' ? '/mail' : '/sms'" aria-label="返回列表">← 返回列表</RouterLink>
    <h1 class="page-title">搜索{{ channelLabel }}</h1>
    <p class="page-lead">搜索限定当前频道（{{ channelLabel }}），不跨频道混合结果；清空搜索或返回均恢复原列表。</p>

    <form class="search-form" role="search" :aria-label="`搜索${channelLabel}`" @submit.prevent="submit">
      <label class="visually-hidden" for="search-query">搜索关键词</label>
      <input
        id="search-query"
        v-model="query"
        type="search"
        autocomplete="off"
        :placeholder="channel === 'sms' ? '匹配发件方、收件方及正文' : '匹配发件人、收件方及主题'"
      >
      <button type="submit" class="search-submit" aria-label="搜索">搜索</button>
    </form>

    <StateBlock
      v-if="!searched"
      variant="empty"
      title="输入关键词开始搜索"
      detail="正式搜索将随服务端批次 2/3 开放。"
    />
    <StateBlock
      v-else
      variant="locked"
      title="正式同步未接通"
      detail="待服务端批次 2/3 开放。当前无法返回搜索结果。"
    >
    </StateBlock>

    <button v-if="searched" type="button" class="state-action search-clear" @click="clearSearch">清空搜索</button>
  </div>
</template>
