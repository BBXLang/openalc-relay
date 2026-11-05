<script setup lang="ts">
import { computed, ref } from 'vue'
import { readEvents, uuidPattern, type SyntheticEvent } from './api'

const installationId = ref('')
const loadedId = ref('')
const events = ref<SyntheticEvent[]>([])
const nextAfter = ref<string | null>(null)
const total = ref(0)
const loading = ref(false)
const status = ref('')
const failed = ref(false)
const queried = ref(false)
const lastRead = ref('')
const selectedTab = ref<'inbox' | 'device'>('inbox')
const panelOpen = ref(false)
const current = computed(() => loadedId.value === installationId.value.trim())

function clearResults() {
  events.value = []
  nextAfter.value = null
  total.value = 0
  queried.value = false
  lastRead.value = ''
}

function changed() {
  clearResults()
  loadedId.value = ''
  failed.value = false
  status.value = ''
}

async function load(more = false) {
  if (loading.value) return
  const id = installationId.value.trim()
  if (!uuidPattern.test(id)) {
    clearResults()
    failed.value = true
    status.value = '请输入规范的小写 UUID 安装 ID。'
    return
  }
  loading.value = true
  failed.value = false
  status.value = '正在读取…'
  if (!more) clearResults()
  try {
    const page = await readEvents(id, more ? nextAfter.value : null)
    events.value = more ? [...events.value, ...page.events] : page.events
    total.value = page.total
    nextAfter.value = page.next_after
    loadedId.value = id
    queried.value = true
    lastRead.value = new Date().toLocaleString('zh-CN', { hour12: false })
    status.value = '读取成功；数据来自 collector 的 SQLite。'
  } catch {
    clearResults()
    failed.value = true
    status.value = '读取失败。请检查本机 collector 是否运行，再手动重试。'
  } finally {
    loading.value = false
  }
}

const sourceTitle = (source: SyntheticEvent['source']) => source === 'sms_broadcast' ? '合成短信事件' : '本机测试事件'
const eventTime = (value: number) => new Date(value).toLocaleString('zh-CN', { hour12: false })
</script>

<template>
  <div class="app-shell">
    <header class="top-bar">
      <button class="round-control menu-control" type="button" aria-label="打开菜单" :aria-expanded="panelOpen" @click="panelOpen = true">
        <span class="menu-lines" aria-hidden="true"><i></i><i></i></span>
      </button>
      <nav class="segmented" aria-label="主导航">
        <button type="button" :class="{ selected: selectedTab === 'inbox' }" :aria-current="selectedTab === 'inbox' ? 'page' : undefined" @click="selectedTab = 'inbox'">收件箱</button>
        <button type="button" :class="{ selected: selectedTab === 'device' }" :aria-current="selectedTab === 'device' ? 'page' : undefined" @click="selectedTab = 'device'">设备</button>
      </nav>
      <button class="round-control profile-control" type="button" aria-label="打开验证信息" @click="panelOpen = true">R</button>
    </header>

    <main v-if="selectedTab === 'inbox'" class="main-area">
      <section class="inbox-content" aria-label="合成事件" :aria-busy="loading">
        <div v-if="queried && current && events.length > 0" class="list-caption">
          <span>合成事件</span>
          <span data-testid="summary">已显示 {{ events.length }} / {{ total }} 条 · 最近读取：{{ lastRead }}</span>
        </div>
        <div v-else-if="queried && current" class="queried-empty">
          <span data-testid="summary" class="visually-hidden">已显示 0 / 0 条 · 最近读取：{{ lastRead }}</span>
          <p>此安装暂无已提交的合成事件。</p>
        </div>
        <div v-if="events.length > 0 && current" class="event-list">
          <article v-for="event in events" :key="event.event_id" class="event-row" data-testid="event-row">
            <span class="event-avatar" aria-hidden="true">{{ event.source === 'sms_broadcast' ? '短' : '测' }}</span>
            <div class="event-body">
              <div class="event-topline"><h2>{{ sourceTitle(event.source) }}</h2><time :datetime="new Date(event.received_at_ms).toISOString()">{{ eventTime(event.received_at_ms) }}</time></div>
              <code>{{ event.event_id }}</code>
            </div>
          </article>
        </div>
        <button v-if="queried && current && nextAfter" class="more-button" :disabled="loading" type="button" @click="load(true)">加载下一页</button>
      </section>
    </main>
    <main v-else class="main-area device-area">
      <div class="device-content">
        <h1>本机设备</h1>
        <p>当前页面仅通过安装 ID 读取本机合成数据。正式设备连接尚未开放。</p>
        <code v-if="current && loadedId">{{ loadedId }}</code>
      </div>
    </main>

    <div v-if="selectedTab === 'inbox'" class="dock-wrap">
      <p v-if="status && (failed || loading)" role="status" class="status-message" :class="{ error: failed }">{{ status }}</p>
      <p v-else role="status" class="visually-hidden">{{ status }}</p>
      <form class="composer" @submit.prevent="load(false)">
        <button class="add-control" type="button" aria-label="查看验证信息" @click="panelOpen = true">+</button>
        <label class="visually-hidden" for="installation">安装 ID（Installation ID）</label>
        <input id="installation" v-model="installationId" :disabled="loading" @input="changed" autocomplete="off" spellcheck="false" placeholder="输入安装 ID 读取合成事件">
        <button class="submit-control" type="submit" :disabled="loading" :aria-label="loading ? '读取中…' : '读取 / 刷新'">
          <span aria-hidden="true">{{ loading ? '…' : '↻' }}</span>
        </button>
      </form>
    </div>

    <div v-if="panelOpen" class="panel-backdrop" @click.self="panelOpen = false">
      <aside class="info-panel" aria-label="验证信息">
        <div class="panel-heading"><strong>Relay</strong><button type="button" aria-label="关闭菜单" @click="panelOpen = false">×</button></div>
        <p class="panel-kicker">本机合成验证</p>
        <p>此页面只读取 collector 已提交的合成事件元数据，不读取正文或号码，也不发送短信。</p>
        <p>安装 ID 是筛选条件，并非身份认证。读取需要本机 collector；页面不会自动轮询。</p>
        <p class="panel-footnote">真实会话、设备配对和推送尚未接入。</p>
      </aside>
    </div>
  </div>
</template>
