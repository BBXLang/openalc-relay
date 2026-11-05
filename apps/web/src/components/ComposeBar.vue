<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  loadMailDraft, loadSmsDraft, saveMailDraft, saveSmsDraft,
  type MailDraft, type SmsDraft,
} from '../lib/drafts'
import { loadDemoAccounts, loadDemoDevices } from '../lib/demoData'
import type { Channel } from '../lib/model'

/*
 * ComposeBar（FORMAL-UI-MAIL-PLAN §2.3 / 导航表底栏）：
 * - 短信页：小号收件号码输入 + 左侧设备/SIM 选择钮（未配对时禁用并显示原因）；
 * - 邮件页：小号收件地址标签输入 + 左侧发件账号选择（无账号时点击跳 /mail/accounts）；
 * - 主输入栏填写正文；发送按钮一律禁用并显示原因「正式链路未接通」；
 * - 草稿按频道独立保存（src/lib/drafts.ts），切频道互不覆盖。
 */
const props = defineProps<{ channel: Channel }>()
const router = useRouter()

const smsDraft = reactive<SmsDraft>(loadSmsDraft())
const mailDraft = reactive<MailDraft>(loadMailDraft())

watch(smsDraft, draft => saveSmsDraft({ to: draft.to, body: draft.body }), { deep: true })
watch(mailDraft, draft => saveMailDraft({ recipients: [...draft.recipients], body: draft.body }), { deep: true })

// 演示设备/账号：仅用于展示底栏控件状态，均标注演示数据；正式设备与账号未接通。
const demoDevices = ref(loadDemoDevices())
const demoAccounts = ref(loadDemoAccounts())
watch(() => props.channel, () => {
  demoDevices.value = loadDemoDevices()
  demoAccounts.value = loadDemoAccounts()
})

const hasDemoDevice = computed(() => demoDevices.value.length > 0)
const hasDemoAccount = computed(() => demoAccounts.value.length > 0)
const selectedDeviceLabel = computed(() => {
  if (!hasDemoDevice.value) return '设备 / SIM'
  const device = demoDevices.value[0]
  const sim = device.sims[0]?.label ?? '无 SIM'
  return `${device.name} · ${sim}（演示）`
})

// 发送在正式链路接通前一律禁用；此状态不可被任何演示数据解锁。
const sendDisabled = true

const sendReason = computed(() => {
  if (props.channel === 'sms' && !hasDemoDevice.value) {
    return '发送不可用：正式链路未接通；未配对设备。'
  }
  return '发送不可用：正式链路未接通。'
})

const deviceButtonTitle = computed(() => (
  hasDemoDevice.value
    ? '设备与 SIM 选择未接通：正式链路未接通（演示设备仅用于展示）。'
    : '未配对设备：请先在「配置」中配对设备（正式配对未接通）。'
))

const accountButtonLabel = computed(() => (
  hasDemoAccount.value ? demoAccounts.value[0].address : '选择发件账号'
))
const accountButtonTitle = computed(() => (
  hasDemoAccount.value
    ? '发件账号管理（当前为演示账号）'
    : '尚无邮箱账号：前往邮箱管理添加账号'
))

function openAccountSettings() {
  router.push('/mail/accounts')
}

// ---- 邮件收件地址标签输入 ----
const tagInput = ref('')

function addRecipient() {
  const value = tagInput.value.trim().replace(/[,，;；]$/, '')
  if (value.length === 0) return
  if (!mailDraft.recipients.includes(value) && mailDraft.recipients.length < 50) {
    mailDraft.recipients.push(value)
  }
  tagInput.value = ''
}

function onTagKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ',' || event.key === '，') {
    event.preventDefault()
    addRecipient()
  } else if (event.key === 'Backspace' && tagInput.value.length === 0 && mailDraft.recipients.length > 0) {
    mailDraft.recipients.pop()
  }
}

function removeRecipient(index: number) {
  mailDraft.recipients.splice(index, 1)
}
</script>

<template>
  <div class="compose-wrap">
    <form class="compose" :aria-label="channel === 'sms' ? '短信撰写栏' : '邮件撰写栏'" @submit.prevent>
      <div v-if="channel === 'sms'" class="compose-recipient">
        <label class="compose-recipient-label" for="compose-sms-to">收件号码</label>
        <input
          id="compose-sms-to"
          v-model="smsDraft.to"
          type="text"
          inputmode="tel"
          autocomplete="off"
          spellcheck="false"
          placeholder="输入收件号码"
        >
      </div>
      <div v-else class="compose-recipient compose-recipient-tags">
        <span class="compose-recipient-label">收件地址</span>
        <span class="compose-tags">
          <span v-for="(recipient, index) in mailDraft.recipients" :key="`${recipient}-${index}`" class="compose-tag">
            {{ recipient }}
            <button type="button" :aria-label="`移除收件地址 ${recipient}`" @click="removeRecipient(index)">×</button>
          </span>
          <input
            v-model="tagInput"
            type="email"
            autocomplete="off"
            spellcheck="false"
            placeholder="输入地址后回车添加"
            aria-label="添加收件地址"
            @keydown="onTagKeydown"
            @blur="addRecipient"
          >
        </span>
      </div>

      <div class="compose-bar">
        <button
          v-if="channel === 'sms'"
          type="button"
          class="compose-side"
          :disabled="true"
          :title="deviceButtonTitle"
          :aria-label="deviceButtonTitle"
        >
          <span class="compose-side-text">{{ selectedDeviceLabel }}</span>
        </button>
        <button
          v-else
          type="button"
          class="compose-side"
          :title="accountButtonTitle"
          :aria-label="accountButtonTitle"
          @click="openAccountSettings"
        >
          <span class="compose-side-text">{{ accountButtonLabel }}</span>
        </button>

        <label class="visually-hidden" for="compose-body">正文</label>
        <input
          v-if="channel === 'sms'"
          id="compose-body"
          v-model="smsDraft.body"
          class="compose-body"
          type="text"
          autocomplete="off"
          placeholder="短信正文"
        >
        <input
          v-else
          id="compose-body"
          v-model="mailDraft.body"
          class="compose-body"
          type="text"
          autocomplete="off"
          placeholder="邮件正文"
        >

        <button
          type="submit"
          class="compose-send"
          :disabled="sendDisabled"
          aria-label="发送：正式链路未接通，暂不可用"
        >发送</button>
      </div>
      <p class="compose-reason" role="note">{{ sendReason }}</p>
    </form>
  </div>
</template>
