import type { ApiResult, EventType } from '../public/api.ts'

const API = `${import.meta.env.VITE_API_BASE ?? ''}/api/v1`

export type Role = 'admin' | 'super_admin'
export type Status = 'processing' | 'new' | 'review' | 'rejected' | 'published'

// name пустое у модераторов, заведённых до появления имени: показывать login.
export interface Me { id: number; login: string; name?: string; role: Role }

export interface Submission {
  id: number
  publicId: string
  title: string
  description: string
  eventDate: string
  eventType: EventType
  email: string
  telegramUsername: string
  latitude?: number
  longitude?: number
  siteUrl?: string
  screenshotUrl?: string
  status: Status
  adminComment?: string
  createdAt: string
  updatedAt: string
}

export interface AuditEntry {
  id: number
  submissionId: number
  submissionPublicId: string
  submissionTitle: string
  adminLogin: string
  adminRole: Role
  action: 'reviewed' | 'rejected' | 'published'
  comment?: string
  createdAt: string
}

export interface Moderator { id: number; login: string; name: string; email: string; role: Role; createdAt: string }

export interface Page<T> { items: T[]; page: number; pageSize: number; total: number }

export const ROLE_LABEL: Record<Role, string> = { admin: 'Модератор', super_admin: 'Супер-админ' }

// status 0 - нет связи. 401 разбирает вызывающий: сессия кончилась, надо на вход.
async function call<T>(method: string, path: string, body?: unknown): Promise<ApiResult<T>> {
  let res: Response
  try {
    res = await fetch(`${API}${path}`, {
      method, credentials: 'include',
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    return { ok: false, status: 0 }
  }
  const data = res.status === 204 ? {} : await res.json().catch(() => ({}))
  if (res.ok) return { ok: true, data }
  return {
    ok: false, status: res.status, code: data.error?.code, message: data.error?.message, fields: data.error?.fields,
    retryAfter: Number(res.headers.get('Retry-After')) || undefined,
  }
}

const query = (params: Record<string, string | number | undefined>) =>
  new URLSearchParams(Object.entries(params).filter(([, v]) => v !== undefined && v !== '').map(([k, v]) => [k, String(v)])).toString()

export const getSession = () => call<{ admin: Me }>('GET', '/auth/session')

export const listSubmissions = (p: { status?: Status | ''; q?: string; page: number }) =>
  call<Page<Submission>>('GET', `/admin/submissions?${query({ ...p, pageSize: 20 })}`)
export const reviewSubmission = (id: number) => call<{ submission: Submission }>('POST', `/admin/submissions/${id}/review`, {})
export const rejectSubmission = (id: number, comment: string) =>
  call<{ submission: Submission }>('POST', `/admin/submissions/${id}/reject`, { comment })
export const publishSubmission = (id: number, latitude: number, longitude: number, eventType: EventType) =>
  call<{ submission: Submission }>('POST', `/admin/submissions/${id}/publish`, { latitude, longitude, eventType })

export const listAudit = (p: { q?: string; page: number }) => call<Page<AuditEntry>>('GET', `/admin/audit-log?${query({ ...p, pageSize: 30 })}`)

export const listModerators = () => call<{ items: Moderator[] }>('GET', '/admin/admins')
export const createModerator = (body: { name: string; email: string; role: Role }) =>
  call<{ admin: Moderator }>('POST', '/admin/admins', body)
export const changeRole = (id: number, role: Role) => call<{ admin: Moderator }>('PATCH', `/admin/admins/${id}/role`, { role })
export const deleteModerator = (id: number) => call('DELETE', `/admin/admins/${id}`)
export const updateModerator = (id: number, body: { name: string; email: string }) =>
  call<{ admin: Moderator }>('PATCH', `/admin/admins/${id}`, body)

// name, address и domain приходят, только если почта настроена. domain - подтверждённый в Postbox.
export interface MailStatus { configured: boolean; name?: string; address?: string; domain?: string }
export const getMail = () => call<{ mail: MailStatus }>('GET', '/admin/mail')
export const setMailSender = (body: { name: string; address: string }) => call<{ mail: MailStatus }>('PUT', '/admin/mail/sender', body)
export const testMail = (email: string) => call('POST', '/admin/mail/test', { email })

export interface TelegramChat { chatId: number; title: string; username?: string }
export interface BotStatus { configured: boolean; username?: string; recipients: TelegramChat[] }

export const getBot = () => call<{ bot: BotStatus }>('GET', '/admin/bot')
export const setBotToken = (token: string) => call<{ bot: BotStatus }>('PUT', '/admin/bot/token', { token })
export const listBotCandidates = () => call<{ items: TelegramChat[] }>('GET', '/admin/bot/candidates')
export const grantBot = (chatId: number) => call<{ bot: BotStatus }>('POST', '/admin/bot/recipients', { chatId })
export const revokeBot = (chatId: number) => call<{ bot: BotStatus }>('DELETE', `/admin/bot/recipients/${chatId}`)
export const testBot = () => call('POST', '/admin/bot/test')

export function problem(res: Extract<ApiResult, { ok: false }>) {
  if (res.status === 0) return 'Нет связи с сервером. Проверьте интернет и попробуйте ещё раз.'
  if (res.status === 429) return `Слишком много попыток. Попробуйте через ${res.retryAfter ?? 60} с.`
  if (res.status === 403) return 'Недостаточно прав.'
  return res.message ?? 'Что-то пошло не так. Попробуйте ещё раз.'
}

export const dateTime = (iso: string) =>
  new Date(iso).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })

export interface MetricsEvent {
  id: number; title: string; views: number; visitors: number; avgActiveSeconds: number
  avgScroll: number; readThrough: number; scrolledViews: number; stars: number; cardOpens: number; cardClicks: number
}
export interface MetricsTotals {
  submissions: number; approved: number; rejected: number; published: number; eventsTotal: number
  views: number; visitors: number; avgActiveSeconds: number; stars: number; starsTotal: number
  cardOpens: number; cardClicks: number; avgScroll: number; readThrough: number; scrolledViews: number
}
export interface Metrics extends MetricsTotals {
  days: number
  previous: MetricsTotals
  funnel: { step: number; visitors: number }[]
  daily: { date: string; views: number; visitors: number }[]
  events: MetricsEvent[]
}
export const getMetrics = (days: number) => call<Metrics>('GET', `/admin/metrics?days=${days}`)
