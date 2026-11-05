/*
 * 草稿存储：每个频道独立 localStorage 键（relay.draft.sms.v1 / relay.draft.mail.v1），
 * 切换频道互不覆盖。
 * 注意：含真实正文的草稿在正式链路开放时须加密保存，当前仅本地明文临时草稿。
 */
export interface SmsDraft {
  to: string
  body: string
}

export interface MailDraft {
  recipients: string[]
  body: string
}

const KEYS = {
  sms: 'relay.draft.sms.v1',
  mail: 'relay.draft.mail.v1',
} as const

export type DraftChannel = keyof typeof KEYS

export function loadSmsDraft(): SmsDraft {
  try {
    const raw = localStorage.getItem(KEYS.sms)
    if (!raw) return { to: '', body: '' }
    const parsed = JSON.parse(raw) as unknown
    if (parsed === null || typeof parsed !== 'object') return { to: '', body: '' }
    const draft = parsed as Record<string, unknown>
    return {
      to: typeof draft.to === 'string' ? draft.to.slice(0, 64) : '',
      body: typeof draft.body === 'string' ? draft.body.slice(0, 4000) : '',
    }
  } catch {
    return { to: '', body: '' }
  }
}

export function loadMailDraft(): MailDraft {
  try {
    const raw = localStorage.getItem(KEYS.mail)
    if (!raw) return { recipients: [], body: '' }
    const parsed = JSON.parse(raw) as unknown
    if (parsed === null || typeof parsed !== 'object') return { recipients: [], body: '' }
    const draft = parsed as Record<string, unknown>
    const recipients = Array.isArray(draft.recipients)
      ? draft.recipients.filter((item): item is string => typeof item === 'string').slice(0, 50)
      : []
    return {
      recipients,
      body: typeof draft.body === 'string' ? draft.body.slice(0, 20000) : '',
    }
  } catch {
    return { recipients: [], body: '' }
  }
}

export function saveSmsDraft(draft: SmsDraft): void {
  try {
    localStorage.setItem(KEYS.sms, JSON.stringify(draft))
  } catch {
    // 存储不可用时草稿仅在当前页面会话内有效。
  }
}

export function saveMailDraft(draft: MailDraft): void {
  try {
    localStorage.setItem(KEYS.mail, JSON.stringify(draft))
  } catch {
    // 存储不可用时草稿仅在当前页面会话内有效。
  }
}

export function clearDrafts(): void {
  try {
    localStorage.removeItem(KEYS.sms)
    localStorage.removeItem(KEYS.mail)
  } catch {
    // 忽略。
  }
}
