import { createRouter, createWebHashHistory, type RouteRecordRaw } from 'vue-router'
import AppShell from './components/AppShell.vue'

/*
 * 正式 UI 路由表（FORMAL-UI-MAIL-PLAN §2.1）。
 *
 * 采用 hash 模式（/#/sms 等）：仓库既有的 PoC e2e 链路（scripts/test-web-poc.ps1）
 * 由 server/internal/poccollector/web.go 提供静态页面，该服务只响应 "/" 与 "/assets/*"，
 * 没有 SPA fallback，history 模式的深链会 404。hash 模式保证每条路由仍是独立 URL，
 * 且既有验证链路与任意静态服务器都能直接打开。
 *
 * 结构：正式页面统一挂在 AppShell 布局记录之下；/dev/synthetic 是独立记录，
 * 不经过正式导航。
 */
export const routes: RouteRecordRaw[] = [
  {
    path: '/',
    component: AppShell,
    children: [
      { path: '', redirect: '/sms' },
      {
        path: 'sms',
        name: 'sms-list',
        // 与 /mail 共用同一组件实例，保证双标签切换时列表状态与滑移动画保持。
        component: () => import('./views/ChannelPagerPage.vue'),
      },
      {
        path: 'mail',
        name: 'mail-list',
        component: () => import('./views/ChannelPagerPage.vue'),
      },
      {
        path: 'search/:channel',
        name: 'search',
        component: () => import('./views/SearchView.vue'),
      },
      {
        path: 'sms/:id',
        name: 'sms-detail',
        component: () => import('./views/SmsDetailView.vue'),
      },
      {
        path: 'mail/:id',
        name: 'mail-detail',
        component: () => import('./views/MailDetailView.vue'),
      },
      {
        path: 'mail/accounts',
        name: 'mail-accounts',
        component: () => import('./views/MailAccountsView.vue'),
      },
      {
        path: 'devices/:id',
        name: 'device-detail',
        component: () => import('./views/DeviceDetailView.vue'),
      },
      {
        path: 'settings',
        name: 'settings',
        component: () => import('./views/SettingsView.vue'),
      },
    ],
  },
  {
    path: '/dev/synthetic',
    name: 'dev-synthetic',
    component: () => import('./views/SyntheticVerification.vue'),
  },
  { path: '/:pathMatch(.*)*', redirect: '/sms' },
]

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
})
