<script setup lang="ts">
import { ref } from 'vue'
import StateBlock from '../components/StateBlock.vue'
import { loadDemoAccounts } from '../lib/demoData'

/*
 * 邮箱账号管理（导航表：侧栏「邮件」组的添加账号入口）。
 * 通用 IMAP/SMTP 手填账号属于正式服务接口（批次 2/3），当前未接通：
 * 本页不提供任何表单或假开关，仅显示未接通状态；本机演示账号（如有）标注展示。
 */
const demoAccounts = ref(loadDemoAccounts())
</script>

<template>
  <div class="page-scroll settings-page">
    <h1 class="page-title">邮箱管理</h1>
    <p class="page-lead">
      正式邮箱账号管理未接通——待服务端批次 2/3 开放。开放后可在此手填 IMAP／SMTP 参数，
      服务端加密保存应用密码或授权码，凭证不会回显。
    </p>

    <section v-if="demoAccounts.length > 0" class="settings-section" aria-label="演示账号">
      <h2>本机演示账号</h2>
      <ul class="settings-list">
        <li v-for="account in demoAccounts" :key="account.id" class="settings-row">
          <span class="settings-row-main">{{ account.address }}</span>
          <span class="settings-row-sub">{{ account.connected ? '连接正常' : '未验证' }}</span>
          <span class="demo-badge">演示数据</span>
        </li>
      </ul>
      <p class="detail-note">演示账号仅存于本机 localStorage，不代表任何真实邮箱。</p>
    </section>

    <StateBlock
      variant="locked"
      title="正式同步未接通"
      detail="待服务端批次 2/3 开放。届时支持连接测试、账号状态、撤销与删除。"
    />
  </div>
</template>
