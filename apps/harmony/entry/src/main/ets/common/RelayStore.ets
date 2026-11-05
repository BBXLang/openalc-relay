/*
 * RelayStore — 本机持久化与小工具。
 * - 主题覆盖（theme_mode）：settings 页写入，EntryAbility 启动时恢复，经
 *   applicationContext.setColorMode 切换 light/dark/system，颜色一律走资源引用自动适配。
 * - 正式频道草稿（正式链路未接通时仅保存输入元数据；含真实正文的草稿在正式链路
 *   开放时必须改为加密保存，见 FORMAL-UI-MAIL-PLAN §2.3）。
 * - isReducedAnimation：读取系统动画时长缩放，<=0 视为“减少动态效果”，用于禁用
 *   频道滑移动画（FORMAL-UI-MAIL-PLAN §2.2）。读取失败按未开启处理（Needs Verification）。
 */
import { preferences } from '@kit.ArkData'
import { common, ConfigurationConstant } from '@kit.AbilityKit'
import { settings } from '@kit.BasicServicesKit'

/** 主题模式取值：'light' | 'dark' | 'system'（常量类避免字面量联合类型）。 */
export class ThemeModeValue {
  static readonly LIGHT: string = 'light'
  static readonly DARK: string = 'dark'
  static readonly SYSTEM: string = 'system'
}

const THEME_STORE: string = 'relay_settings'
const THEME_KEY: string = 'theme_mode'
const DRAFT_STORE: string = 'relay_formal_drafts'

export class RelayStore {
  static themeMode(context: common.Context): string {
    try {
      const opts: preferences.Options = { name: THEME_STORE }
      const store = preferences.getPreferencesSync(context, opts)
      const value = store.getSync(THEME_KEY, ThemeModeValue.SYSTEM) as string
      if (value === ThemeModeValue.LIGHT || value === ThemeModeValue.DARK) {
        return value
      }
      return ThemeModeValue.SYSTEM
    } catch (_error) {
      return ThemeModeValue.SYSTEM
    }
  }

  static saveThemeMode(context: common.Context, mode: string): void {
    try {
      const opts: preferences.Options = { name: THEME_STORE }
      const store = preferences.getPreferencesSync(context, opts)
      store.putSync(THEME_KEY, mode)
      store.flush().catch(() => {
      })
    } catch (_error) {
      // 保存失败不阻塞界面；主题覆盖仅是本机偏好。
    }
  }

  static applyColorMode(appContext: common.ApplicationContext, mode: string): void {
    let value: ConfigurationConstant.ColorMode = ConfigurationConstant.ColorMode.COLOR_MODE_NOT_SET
    if (mode === ThemeModeValue.LIGHT) {
      value = ConfigurationConstant.ColorMode.COLOR_MODE_LIGHT
    } else if (mode === ThemeModeValue.DARK) {
      value = ConfigurationConstant.ColorMode.COLOR_MODE_DARK
    }
    try {
      appContext.setColorMode(value)
    } catch (_error) {
      // 忽略：保持跟随系统。
    }
  }

  static loadDraft(context: common.Context, channel: string, field: string): string {
    try {
      const opts: preferences.Options = { name: DRAFT_STORE }
      const store = preferences.getPreferencesSync(context, opts)
      return store.getSync(channel + '_' + field, '') as string
    } catch (_error) {
      return ''
    }
  }

  static saveDraft(context: common.Context, channel: string, field: string, value: string): void {
    try {
      const opts: preferences.Options = { name: DRAFT_STORE }
      const store = preferences.getPreferencesSync(context, opts)
      store.putSync(channel + '_' + field, value)
      store.flush().catch(() => {
      })
    } catch (_error) {
      // 忽略。
    }
  }

  static isReducedAnimation(context: common.Context): boolean {
    try {
      const raw = settings.getValueSync(context, settings.display.ANIMATOR_DURATION_SCALE, '1')
      const scale = parseFloat(raw)
      return !isNaN(scale) && scale <= 0
    } catch (_error) {
      return false
    }
  }
}
