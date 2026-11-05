<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { loadDemoAccounts, loadDemoDevices } from '../lib/demoData'
import { formatTime, type DeviceModel, type MailAccountModel } from '../lib/model'

/*
 * SideBar（FORMAL-UI-MAIL-PLAN §2.2）：文字 logo（Relay + by OpenALC）、「设备」「邮件」
 * 两个可独立展开的组、「配置」入口。侧栏只展示合成状态：演示设备/账号来自本机
 * localStorage 且必须标注「演示数据」；无数据时展示空态与配对/添加账号动作。
 */
defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const route = useRoute()
const devicesOpen = ref(false)
const mailOpen = ref(false)
// 每次打开侧栏前重新读取，保证 /dev/synthetic 写入演示数据后侧栏立即可见。
const demoDevices = ref<DeviceModel[]>([])
const demoAccounts = ref<MailAccountModel[]>([])

function refreshDemo() {
  demoDevices.value = loadDemoDevices()
  demoAccounts.value = loadDemoAccounts()
}
refreshDemo()

defineExpose({ refreshDemo })
</script>

<template>
  <aside v-if="open" class="sidebar" role="dialog" aria-modal="true" aria-label="侧栏菜单">
    <header class="sidebar-head">
      <div class="sidebar-logo">
        <span class="logo-relay">Relay</span>
        <span class="logo-by">by OpenALC</span>
      </div>
      <button class="sidebar-close" type="button" aria-label="关闭侧栏" @click="emit('close')">×</button>
    </header>

    <nav class="sidebar-nav" aria-label="侧栏导航">
      <section class="sidebar-group">
        <button
          class="sidebar-group-toggle"
          type="button"
          :aria-expanded="devicesOpen"
          @click="devicesOpen = !devicesOpen"
        >
          <span>设备</span>
          <span class="sidebar-caret" aria-hidden="true">{{ devicesOpen ? '▾' : '▸' }}</span>
        </button>
        <div v-if="devicesOpen" class="sidebar-group-body">
          <p v-if="demoDevices.length === 0" class="sidebar-empty">未配对设备</p>
          <ul v-else class="sidebar-items">
            <li v-for="device in demoDevices" :key="device.id">
              <RouterLink class="sidebar-item" :to="`/devices/${encodeURIComponent(device.id)}`">
                <span class="sidebar-item-main">{{ device.name }}</span>
                <span class="sidebar-item-sub">
                  {{ device.sims.map(sim => sim.label).join(' · ') || '无 SIM' }}
                </span>
                <span class="demo-badge" data-testid="sidebar-demo-badge">演示数据</span>
              </RouterLink>
            </li>
          </ul>
          <RouterLink class="sidebar-action" to="/settings">配对设备</RouterLink>
        </div>
      </section>

      <section class="sidebar-group">
        <button
          class="sidebar-group-toggle"
          type="button"
          :aria-expanded="mailOpen"
          @click="mailOpen = !mailOpen"
        >
          <span>邮件</span>
          <span class="sidebar-caret" aria-hidden="true">{{ mailOpen ? '▾' : '▸' }}</span>
        </button>
        <div v-if="mailOpen" class="sidebar-group-body">
          <p v-if="demoAccounts.length === 0" class="sidebar-empty">未添加邮箱账号</p>
          <ul v-else class="sidebar-items">
            <li v-for="account in demoAccounts" :key="account.id">
              <span class="sidebar-item sidebar-item-static">
                <span class="sidebar-item-main">{{ account.address }}</span>
                <span class="sidebar-item-sub">{{ account.connected ? '连接正常（演示）' : '未验证（演示）' }}</span>
                <span class="demo-badge">演示数据</span>
              </span>
            </li>
          </ul>
          <RouterLink class="sidebar-action" to="/mail/accounts">添加账号</RouterLink>
        </div>
      </section>

      <RouterLink
        class="sidebar-entry"
        to="/settings"
        :aria-current="route.name === 'settings' ? 'page' : undefined"
      >配置</RouterLink>
    </nav>

    <footer class="sidebar-foot">
      <p>设备配对与邮箱账号尚未接入正式服务；此侧栏仅显示本机演示状态。</p>
      <p v-if="demoDevices.length > 0 || demoAccounts.length > 0" class="sidebar-foot-time">
        演示设备最近观测：{{ demoDevices.length > 0 ? formatTime(demoDevices[0].lastSeenAt) : '无' }}
      </p>
    </footer>
  </aside>
</template>
