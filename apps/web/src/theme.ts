import { ref, watch } from 'vue'

/*
 * 主题处理：默认跟随系统（prefers-color-scheme），Settings 页可覆盖 light/dark/system。
 * 覆盖值保存在 localStorage；解析后的主题写入 <html data-theme> 并同步 index.html 的
 * theme-color。注意：本仓库 PoC collector 的 CSP 禁止内联脚本，因此不允许在 index.html
 * 放置防闪烁内联脚本，主题在 main.ts 挂载应用前应用。
 */
export type ThemeSetting = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'relay.theme.v1'
const RESOLVED_BG: Record<ThemeResolution, string> = { light: '#fafafa', dark: '#111214' }
export type ThemeResolution = 'light' | 'dark'

function isThemeSetting(value: unknown): value is ThemeSetting {
  return value === 'light' || value === 'dark' || value === 'system'
}

export function loadThemeSetting(): ThemeSetting {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return isThemeSetting(raw) ? raw : 'system'
  } catch {
    return 'system'
  }
}

export function persistThemeSetting(setting: ThemeSetting): void {
  try {
    localStorage.setItem(STORAGE_KEY, setting)
  } catch {
    // localStorage 不可用时主题仍可即时生效，仅不能跨会话记住。
  }
}

const systemDark = typeof matchMedia === 'function' ? matchMedia('(prefers-color-scheme: dark)') : null

export const themeSetting = ref<ThemeSetting>(loadThemeSetting())

export function resolveTheme(setting: ThemeSetting): ThemeResolution {
  if (setting === 'system') return systemDark?.matches ? 'dark' : 'light'
  return setting
}

export function applyTheme(setting: ThemeSetting): void {
  const resolved = resolveTheme(setting)
  document.documentElement.dataset.theme = resolved
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', RESOLVED_BG[resolved])
}

export function setThemeSetting(setting: ThemeSetting): void {
  themeSetting.value = setting
  persistThemeSetting(setting)
}

let started = false

export function initTheme(): void {
  if (started) return
  started = true
  applyTheme(themeSetting.value)
  watch(themeSetting, applyTheme)
  systemDark?.addEventListener('change', () => {
    if (themeSetting.value === 'system') applyTheme('system')
  })
}
