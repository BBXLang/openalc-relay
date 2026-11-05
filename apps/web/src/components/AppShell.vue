<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SideBar from './SideBar.vue'
import ComposeBar from './ComposeBar.vue'
import { swipePosition } from '../lib/channelSwitch'
import type { Channel } from '../lib/model'

/*
 * AppShell（FORMAL-UI-MAIL-PLAN §2.1 / §2.2）：
 * - 顶栏：左圆形菜单钮开侧栏；中央「短信/邮件」双标签（role=tablist）；右圆形搜索钮。
 * - 侧栏遮罩：遮罩点击、关闭钮、浏览器返回（popstate）均可关；开合不改变列表、筛选或草稿。
 * - 列表路由（/sms、/mail）在内容区渲染双窗格 Pager 与底部撰写栏；正式导航不出现 PoC 入口。
 */
const route = useRoute()
const router = useRouter()
const navOpen = ref(false)
const sidebar = ref<InstanceType<typeof SideBar> | null>(null)

const channel = computed<Channel>(() => {
  if (route.name === 'mail-list' || route.name === 'mail-detail' || route.name === 'mail-accounts') return 'mail'
  if (route.name === 'search') return route.params.channel === 'mail' ? 'mail' : 'sms'
  return 'sms'
})

const onChannelList = computed(() => route.name === 'sms-list' || route.name === 'mail-list')

// 双标签滑动指示块：滑动中跟随手势进度，其余时间由频道决定并走 CSS 过渡。
const pillStyle = computed(() => {
  if (swipePosition.value !== null) {
    return { transform: `translateX(${(swipePosition.value * 100).toFixed(3)}%)`, transition: 'none' }
  }
  return { transform: `translateX(${channel.value === 'mail' ? '100%' : '0%'})` }
})

function openNav() {
  if (navOpen.value) return
  navOpen.value = true
  sidebar.value?.refreshDemo()
  // 压入一条同 URL 的历史记录，浏览器返回即关闭侧栏。
  history.pushState({ relayNavOpen: true }, '')
}

function closeNav(consumeHistory = true) {
  if (!navOpen.value) return
  navOpen.value = false
  if (consumeHistory && (history.state as Record<string, unknown> | null)?.relayNavOpen === true) {
    history.back()
  }
}

function onPopState() {
  if (navOpen.value) navOpen.value = false
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && navOpen.value) closeNav()
}

onMounted(() => {
  window.addEventListener('popstate', onPopState)
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('popstate', onPopState)
  window.removeEventListener('keydown', onKeydown)
})

// 从侧栏发起路由跳转后自动关闭侧栏（不回退历史）。
watch(() => route.fullPath, () => {
  if (navOpen.value) navOpen.value = false
})

function goChannel(target: Channel) {
  router.push(target === 'sms' ? '/sms' : '/mail')
}

function onTabKeydown(event: KeyboardEvent) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
  event.preventDefault()
  const target: Channel = event.key === 'ArrowLeft' ? 'sms' : 'mail'
  goChannel(target)
  document.getElementById(target === 'sms' ? 'tab-sms' : 'tab-mail')?.focus()
}

function goSearch() {
  router.push(`/search/${channel.value}`)
}
</script>

<template>
  <div class="shell">
    <header class="shell-topbar">
      <button
        class="round-control"
        type="button"
        aria-label="打开侧栏菜单"
        :aria-expanded="navOpen"
        aria-controls="sidebar"
        @click="openNav"
      >
        <span class="menu-lines" aria-hidden="true"><i></i><i></i></span>
      </button>

      <nav class="segmented" role="tablist" aria-label="频道切换">
        <span class="segmented-pill" aria-hidden="true" :style="pillStyle"></span>
        <button
          id="tab-sms"
          type="button"
          role="tab"
          :aria-selected="channel === 'sms'"
          aria-controls="pane-sms"
          @keydown="onTabKeydown"
          @click="goChannel('sms')"
        >短信</button>
        <button
          id="tab-mail"
          type="button"
          role="tab"
          :aria-selected="channel === 'mail'"
          aria-controls="pane-mail"
          @keydown="onTabKeydown"
          @click="goChannel('mail')"
        >邮件</button>
      </nav>

      <button class="round-control" type="button" aria-label="搜索" @click="goSearch">
        <svg class="icon-search" viewBox="0 0 24 24" width="22" height="22" aria-hidden="true" focusable="false">
          <circle cx="10.5" cy="10.5" r="6" fill="none" stroke="currentColor" stroke-width="2"></circle>
          <line x1="15.2" y1="15.2" x2="20" y2="20" stroke="currentColor" stroke-width="2" stroke-linecap="round"></line>
        </svg>
      </button>
    </header>

    <main class="shell-main">
      <RouterView />
    </main>

    <ComposeBar v-if="onChannelList" :channel="channel" />

    <div v-if="navOpen" class="shell-scrim" aria-hidden="true" @click="closeNav()"></div>
    <div id="sidebar" class="sidebar-host">
      <SideBar ref="sidebar" :open="navOpen" @close="closeNav()" />
    </div>
  </div>
</template>
