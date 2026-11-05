<script setup lang="ts">
import { themeSetting, setThemeSetting, type ThemeSetting } from '../theme'

/*
 * 全局配置页（导航表：侧栏「配置」）。
 * - 主题：跟随系统 / 亮色 / 暗色，写入 localStorage 并即时生效（src/theme.ts）；
 * - 配对与撤销：占位入口（正式配对未接通，不放假开关）；
 * - 邮箱管理：入口跳转 /mail/accounts；
 * - 安全与同步状态：全部为「未接通」状态文案。
 */
const THEME_OPTIONS: Array<{ value: ThemeSetting; label: string }> = [
  { value: 'system', label: '跟随系统' },
  { value: 'light', label: '亮色' },
  { value: 'dark', label: '暗色' },
]

const SECURITY_ROWS: Array<{ label: string; state: string }> = [
  { label: '设备配对与撤销', state: '未接通' },
  { label: '短信同步与远程发送', state: '未接通' },
  { label: '邮箱账号与邮件同步', state: '未接通' },
  { label: '负载认证加密', state: '未接通' },
  { label: '推送辅助通道', state: '未接通' },
]
</script>

<template>
  <div class="page-scroll settings-page">
    <h1 class="page-title">配置</h1>

    <section class="settings-section" aria-label="外观">
      <h2>外观</h2>
      <div class="settings-theme" role="group" aria-label="主题设置">
        <button
          v-for="option in THEME_OPTIONS"
          :key="option.value"
          type="button"
          class="theme-option"
          :class="{ selected: themeSetting === option.value }"
          :aria-pressed="themeSetting === option.value"
          :data-testid="`theme-${option.value}`"
          @click="setThemeSetting(option.value)"
        >{{ option.label }}</button>
      </div>
      <p class="detail-note">默认跟随系统亮暗设置；选择后立即生效并保存到本机。</p>
    </section>

    <section class="settings-section" aria-label="配对与撤销">
      <h2>配对与撤销</h2>
      <div class="settings-row">
        <span class="settings-row-main">设备配对与撤销管理</span>
        <span class="settings-row-state" data-testid="pairing-state">未接通</span>
        <button
          type="button"
          class="settings-disabled-button"
          disabled
          title="正式设备配对未接通：待服务端批次 2/3 开放。"
          aria-label="配对新设备：正式设备配对未接通，暂不可用"
        >配对新设备</button>
      </div>
      <p class="detail-note">配对为短时、单次、需用户明确批准的操作；当前没有可执行的配对入口。</p>
    </section>

    <section class="settings-section" aria-label="邮箱管理">
      <h2>邮箱管理</h2>
      <div class="settings-row">
        <span class="settings-row-main">邮箱账号、连接测试与凭证</span>
        <span class="settings-row-state">未接通</span>
        <RouterLink class="settings-link" to="/mail/accounts" aria-label="打开邮箱管理">打开邮箱管理</RouterLink>
      </div>
    </section>

    <section class="settings-section" aria-label="安全与同步状态">
      <h2>安全与同步状态</h2>
      <ul class="settings-list">
        <li v-for="row in SECURITY_ROWS" :key="row.label" class="settings-row">
          <span class="settings-row-main">{{ row.label }}</span>
          <span class="settings-row-state" data-testid="sync-state">{{ row.state }}</span>
        </li>
      </ul>
      <p class="detail-note">
        当前为架构与可行性验证阶段：没有认证与负载加密，不接受真实通信数据，
        亦不声明生产可用或端到端加密。
      </p>
    </section>
  </div>
</template>
