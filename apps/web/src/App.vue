<script setup lang="ts">
/*
 * 根组件：路由决定渲染内容——正式页面渲染 AppShell 布局记录，
 * /dev/synthetic 独立渲染原 PoC 合成验证页（不带正式导航）。
 * 根路径由路由重定向到 /sms，不再默认命中 collector。
 */
</script>

<template>
  <RouterView />
</template>
