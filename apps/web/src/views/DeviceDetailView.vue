<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import StateBlock from '../components/StateBlock.vue'
import { loadDemoDevices } from '../lib/demoData'
import { formatTime, type DeviceModel } from '../lib/model'

/*
 * 设备详情（导航表：侧栏设备项进入设备详情；显示已识别 SIM、权限、最后观测时间和撤销状态）。
 * 正式配对未接通：仅当设备 ID 命中本机 localStorage 中的演示设备时展示演示数据
 * （明确标注「演示数据」），否则渲染「记录不存在或未同步」。
 */
const route = useRoute()
const deviceId = typeof route.params.id === 'string' ? route.params.id : ''

const device = computed<DeviceModel | undefined>(() => (
  loadDemoDevices().find(candidate => candidate.id === deviceId)
))

const notFoundDetail = computed(() => (
  deviceId ? `请求的设备 ID：${deviceId}` : undefined
))
</script>

<template>
  <div class="page-scroll detail-page">
    <RouterLink class="detail-back" to="/settings" aria-label="返回配置">← 返回配置</RouterLink>
    <StateBlock
      v-if="!device"
      variant="notfound"
      title="记录不存在或未同步"
      :detail="notFoundDetail"
    />
    <article v-else class="detail">
      <header class="detail-head">
        <h1 class="detail-title">设备详情</h1>
        <p class="demo-banner" data-testid="device-demo-banner">演示数据 · 非真实设备</p>
        <dl class="detail-source">
          <div class="detail-source-row"><dt>设备名称</dt><dd>{{ device.name }}</dd></div>
          <div class="detail-source-row"><dt>已识别 SIM</dt>
            <dd>
              <span v-for="sim in device.sims" :key="sim.label" class="msg-card-chip">
                {{ sim.label }}{{ sim.number ? `（${sim.number}）` : '（号码未确认）' }}
              </span>
              <span v-if="device.sims.length === 0">无</span>
            </dd>
          </div>
          <div class="detail-source-row"><dt>权限</dt><dd>{{ device.permissions }}</dd></div>
          <div class="detail-source-row"><dt>最后观测时间</dt><dd><time :datetime="device.lastSeenAt">{{ formatTime(device.lastSeenAt) }}</time></dd></div>
          <div class="detail-source-row"><dt>撤销状态</dt><dd>{{ device.revoked ? '已撤销' : '未撤销' }}</dd></div>
        </dl>
      </header>
      <section class="detail-section" aria-label="说明">
        <p class="detail-note">正式设备配对、权限同步与撤销尚未接通；此处仅展示本机演示设备。</p>
      </section>
    </article>
  </div>
</template>
