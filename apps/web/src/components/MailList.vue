<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import StateBlock from './StateBlock.vue'

/*
 * 邮件列表（FORMAL-UI-MAIL-PLAN §2.2）。
 * 正式数据链路未接通：本列表不渲染任何会话卡片，统一显示 StateBlock
 * 「正式同步未接通——待服务端批次 2/3 开放」；断网时显示断网态。
 * 卡片组件只在 /dev/synthetic 以演示数据展示。
 */
const online = ref(true)

function updateOnline() {
  online.value = navigator.onLine
}

onMounted(() => {
  updateOnline()
  window.addEventListener('online', updateOnline)
  window.addEventListener('offline', updateOnline)
})

onBeforeUnmount(() => {
  window.removeEventListener('online', updateOnline)
  window.removeEventListener('offline', updateOnline)
})
</script>

<template>
  <div class="list-page" aria-label="邮件列表">
    <StateBlock
      v-if="!online"
      variant="offline"
      detail="网络恢复后将自动进入正常状态。"
    />
    <StateBlock
      v-else
      variant="locked"
      title="正式同步未接通"
      detail="待服务端批次 2/3 开放。邮箱账号、收件箱同步与真实邮件数据将随正式接口一并开放。"
    />
  </div>
</template>
