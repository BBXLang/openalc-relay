<script setup lang="ts">
/*
 * 合成验证入口（/dev/synthetic）：原 App.vue PoC（collector 阅读 UI）与 api.ts 原样迁入，
 * 保留安装 ID 校验、分页、错误路径行为；另含「正式 UI 演示区」，以明确标注的演示数据
 * 展示正式卡片与详情组件（FORMAL-UI-MAIL-PLAN §2.2：测试卡片只出现在独立验证入口）。
 * 本页面不属于正式导航，侧栏与顶栏不提供此入口。
 */
import { computed, ref } from 'vue'
import SmsCard from '../components/SmsCard.vue'
import MailCard from '../components/MailCard.vue'
import SmsDetail from '../components/SmsDetail.vue'
import MailDetail from '../components/MailDetail.vue'
import {
  clearDemoData, demoMailDetail, demoSmsDetail, saveDemoAccounts, saveDemoDevices,
} from '../lib/demoData'
import type { DeviceModel, MailAccountModel } from '../lib/model'

// ---- 以下为原 src/api.ts 内容，原样迁入（script setup 内不允许 export，去修饰不改行为） ----
interface SyntheticEvent {
  event_id: string
  fixture_id: string
  source: 'injected' | 'sms_broadcast'
  received_at_ms: number
  subscription_id: number | null
}

interface EventPage {
  installation_id: string
  events: SyntheticEvent[]
  total: number
  next_after: string | null
}

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/

function validEvent(event: SyntheticEvent): boolean {
  return event !== null && typeof event === 'object' &&
    typeof event.event_id === 'string' && uuidPattern.test(event.event_id) &&
    event.fixture_id === event.event_id && ['injected', 'sms_broadcast'].includes(event.source) &&
    Number.isSafeInteger(event.received_at_ms) && event.received_at_ms > 0 && event.received_at_ms < 4102444800000 &&
    (event.subscription_id === null || (Number.isInteger(event.subscription_id) && event.subscription_id >= 0 && event.subscription_id <= 2147483647))
}

async function readEvents(installationId: string, after: string | null): Promise<EventPage> {
  const query = new URLSearchParams({ installation_id: installationId, limit: '25' })
  if (after) query.set('after', after)
  const response = await fetch(`/poc/v1/events?${query}`, {
    cache: 'no-store', credentials: 'omit', signal: AbortSignal.timeout(5000),
  })
  if (!response.ok) throw new Error('read_failed')
  const page: EventPage = await response.json()
  if (page === null || page.installation_id !== installationId || !Array.isArray(page.events) ||
      page.events.length > 25 || !page.events.every(validEvent) ||
      new Set(page.events.map(event => event.event_id)).size !== page.events.length ||
      !Number.isSafeInteger(page.total) || page.total < 0 ||
      !(page.next_after === null || (typeof page.next_after === 'string' && uuidPattern.test(page.next_after)))) {
    throw new Error('invalid_response')
  }
  let previous = after ?? ''
  for (const event of page.events) {
    if (event.event_id <= previous) throw new Error('invalid_order')
    previous = event.event_id
  }
  if (page.events.length > page.total || (page.next_after !== null && page.next_after !== page.events.at(-1)?.event_id)) {
    throw new Error('invalid_page')
  }
  return page
}

// ---- 以下为原 App.vue PoC 逻辑，原样迁入 ----
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

// ---- 正式 UI 演示区（全部为演示数据） ----
const showSmsDetail = ref(false)
const showMailDetail = ref(false)

function seedDemoData() {
  const device: DeviceModel = {
    id: 'demo-device-0001',
    name: '演示设备 A',
    sims: [{ label: 'SIM 1', number: null }],
    permissions: '演示状态：短信读取已授权',
    lastSeenAt: '2026-11-05T09:00:00+08:00',
    revoked: false,
  }
  const account: MailAccountModel = {
    id: 'demo-account-0001',
    address: 'demo.account@example.invalid',
    connected: false,
  }
  saveDemoDevices([device])
  saveDemoAccounts([account])
}
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

      <section class="demo-lab" aria-label="正式 UI 演示区">
        <h2 class="demo-lab-title">正式 UI 演示区</h2>
        <p class="demo-lab-note">
          以下卡片、详情与演示设备/账号全部为<b>演示数据</b>，仅存于本机 localStorage，
          用于验证正式组件的字段渲染；正式列表不会显示任何卡片。
        </p>

        <div class="demo-actions">
          <button type="button" class="more-button" @click="seedDemoData()">写入演示设备与账号</button>
          <button type="button" class="more-button" @click="clearDemoData()">清除演示数据</button>
        </div>
        <p class="demo-lab-note">写入后可在正式 UI 侧栏「设备 / 邮件」组、设备详情与邮箱管理看到标注「演示数据」的条目。</p>

        <h3 class="demo-subtitle">短信卡片（演示数据）</h3>
        <SmsCard :record="demoSmsDetail" demo @open="showSmsDetail = !showSmsDetail" />
        <SmsDetail v-if="showSmsDetail" class="demo-detail" :record="demoSmsDetail" demo />

        <h3 class="demo-subtitle">邮件卡片（演示数据）</h3>
        <MailCard :record="demoMailDetail" demo @open="showMailDetail = !showMailDetail" />
        <MailDetail v-if="showMailDetail" class="demo-detail" :record="demoMailDetail" demo />
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
        <p class="panel-footnote">真实会话、设备配对和推送尚未接入。正式 UI 见「短信 / 邮件」页面；本页仅供开发验证。</p>
      </aside>
    </div>
  </div>
</template>

<style scoped>
/* ---- 原 src/style.css 中 PoC 专用样式，原样迁入（scoped） ---- */
.app-shell {
  min-height: 100dvh;
  background: #fafafa;
  color: #111;
}

.top-bar {
  width: min(100%, 960px);
  margin: 0 auto;
  padding: 28px 20px 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}

.round-control {
  width: 50px;
  height: 50px;
  flex: 0 0 50px;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 50%;
  background: #fff;
  color: #111;
}

.menu-lines {
  width: 22px;
  display: flex;
  flex-direction: column;
  gap: 7px;
}
.menu-lines i {
  display: block;
  width: 22px;
  height: 2.5px;
  border-radius: 3px;
  background: #111;
}
.menu-lines i:last-child { width: 15px; }
.profile-control { font-size: 24px; font-weight: 650; }

.segmented {
  display: flex;
  flex: 0 1 220px;
  min-width: 0;
  padding: 4px;
  background: #e9e9e9;
  border-radius: 100px;
}
.segmented button {
  width: 50%;
  min-height: 46px;
  border: 0;
  border-radius: 100px;
  background: transparent;
  color: #111;
  font-size: 16px;
}
.segmented button.selected { background: #fff; }

.main-area {
  width: min(100%, 760px);
  min-height: calc(100dvh - 160px);
  margin: 0 auto;
  padding: 26px 24px 140px;
}
.inbox-content { width: 100%; }
.list-caption {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
  justify-content: space-between;
  padding: 10px 0 14px;
  color: #888;
  font-size: 12px;
}
.event-row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 0;
  border-bottom: 1px solid #ececec;
}
.event-avatar {
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: #e9e9e9;
  font-size: 15px;
}
.event-body { flex: 1; min-width: 0; }
.event-topline {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
}
.event-topline h2 { margin: 0; font-size: 14px; font-weight: 600; }
.event-topline time { color: #929292; font-size: 10px; white-space: nowrap; }
.event-row code {
  display: block;
  overflow-wrap: anywhere;
  margin-top: 5px;
  color: #858585;
  font-size: 11px;
}
.queried-empty {
  display: grid;
  place-items: center;
  min-height: calc(100dvh - 270px);
  color: #858585;
  font-size: 13px;
}
.queried-empty p { margin: 0; }
.more-button {
  display: block;
  margin: 24px auto 0;
  padding: 10px 18px;
  background: #fff;
  border: 1px solid #e5e5e5;
  border-radius: 24px;
  color: #333;
}

.device-content { padding: 20px 5px; }
.device-content h1 { font-size: 22px; font-weight: 600; }
.device-content p { color: #858585; font-size: 13px; line-height: 1.7; }
.device-content code { color: #555; font-size: 12px; overflow-wrap: anywhere; }

.dock-wrap {
  position: fixed;
  z-index: 3;
  left: 0;
  right: 0;
  bottom: 24px;
  width: min(100% - 40px, 720px);
  margin: 0 auto;
}
.composer {
  height: 66px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 8px;
  border-radius: 40px;
  background: #fff;
  box-shadow: 0 10px 35px #00000012;
}
.add-control {
  width: 46px;
  height: 46px;
  flex: 0 0 46px;
  border: 0;
  background: transparent;
  color: #111;
  font-size: 37px;
  font-weight: 300;
  line-height: 1;
}
.composer input {
  min-width: 0;
  flex: 1;
  border: 0;
  outline: 0;
  background: transparent;
  color: #111;
  font-size: 16px;
}
.composer input::placeholder { color: #8d8d8d; opacity: 1; }
.submit-control {
  width: 50px;
  height: 50px;
  flex: 0 0 50px;
  border: 0;
  border-radius: 50%;
  background: #ffd12e;
  color: #111;
  font-size: 29px;
  line-height: 1;
}
.status-message {
  max-width: max-content;
  margin: 0 auto 10px;
  padding: 9px 14px;
  border-radius: 18px;
  background: #fff;
  box-shadow: 0 5px 20px #0000000c;
  color: #555;
  font-size: 12px;
}
.status-message.error { color: #ab3c39; }

.panel-backdrop {
  position: fixed;
  z-index: 10;
  inset: 0;
  background: #0005;
}
.info-panel {
  width: min(330px, 86vw);
  min-height: 100%;
  padding: 28px 24px;
  background: #fafafa;
  box-shadow: 8px 0 32px #0002;
}
.panel-heading { display: flex; align-items: center; justify-content: space-between; }
.panel-heading strong { font-size: 24px; }
.panel-heading button {
  width: 40px;
  height: 40px;
  border: 0;
  border-radius: 50%;
  background: #fff;
  font-size: 27px;
}
.panel-kicker { margin-top: 32px; font-weight: 600; }
.info-panel p { color: #555; font-size: 13px; line-height: 1.8; }
.info-panel .panel-footnote { color: #888; font-size: 12px; }

@media (max-width: 480px) {
  .top-bar { padding: 24px 14px 0; gap: 8px; }
  .round-control { width: 48px; height: 48px; flex-basis: 48px; }
  .segmented { flex-basis: 190px; }
  .segmented button { min-height: 42px; font-size: 15px; }
  .main-area { padding: 24px 20px 135px; }
  .dock-wrap { width: calc(100% - 32px); bottom: 20px; }
  .composer input { font-size: 14px; }
}

/* ---- 正式 UI 演示区 ---- */
.demo-lab {
  margin-top: 40px;
  padding-top: 8px;
  border-top: 2px dashed #d9d9d9;
}
.demo-lab-title { margin: 0; font-size: 18px; font-weight: 650; }
.demo-lab-note { color: #666; font-size: 12.5px; line-height: 1.8; }
.demo-lab-note b { color: #111; }
.demo-actions { display: flex; flex-wrap: wrap; gap: 12px; }
.demo-actions .more-button { margin: 12px 0 0; }
.demo-subtitle { margin: 26px 0 10px; font-size: 14px; font-weight: 600; color: #333; }
.demo-detail { margin-top: 12px; }
</style>
