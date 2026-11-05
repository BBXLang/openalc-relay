<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import ChannelPager from '../components/ChannelPager.vue'
import SmsList from '../components/SmsList.vue'
import MailList from '../components/MailList.vue'
import type { Channel } from '../lib/model'

/*
 * /sms 与 /mail 共用的列表页。两条路由指向同一组件，vue-router 复用实例，
 * 因此双标签切换时双窗格保持挂载：滑动动画、滚动位置与筛选状态不丢失。
 */
const route = useRoute()
const channel = computed<Channel>(() => (route.name === 'mail-list' ? 'mail' : 'sms'))
</script>

<template>
  <ChannelPager :channel="channel">
    <template #sms><SmsList /></template>
    <template #mail><MailList /></template>
  </ChannelPager>
</template>
