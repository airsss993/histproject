export type EventType = 'military' | 'political' | 'personality'

export interface HistEvent {
  id: number
  title: string
  description: string
  latitude: number
  longitude: number
  eventDate: string
  eventType: EventType
  siteUrl: string
  previewImageUrl?: string | null
  stars: number
  starred: boolean
  // Уникальные посетители сайта: аккаунт, у гостя - браузер. Видны всем.
  visitors: number
  // Только в fetchEvent и только если у автора есть учётка.
  author?: { publicId: string; firstName: string; lastName: string }
}

const API = `${import.meta.env.VITE_API_BASE ?? ''}/api/v1`

export const TYPE_LABEL: Record<EventType, string> = { military: 'Военное', political: 'Политическое', personality: 'Личность' }
export const TYPE_PLURAL: Record<EventType, string> = { military: 'Военные', political: 'Политические', personality: 'Личности' }
export const TYPES: EventType[] = ['military', 'political', 'personality']

export const yearOf = (e: HistEvent) => parseInt(e.eventDate.slice(0, 4), 10)
export const formatDate = (iso: string) => iso.slice(0, 10).split('-').reverse().join('.')

// Бекенд отдаёт не больше 100 на страницу, карта нужна целиком.
export async function fetchAllEvents(signal: AbortSignal): Promise<HistEvent[]> {
  const all: HistEvent[] = []
  for (let page = 1; ; page++) {
    const res = await fetch(`${API}/events?page=${page}&pageSize=100`, { signal, credentials: 'include' })
    if (!res.ok) throw new Error(`events: HTTP ${res.status}`)
    const body: { items: HistEvent[]; total: number } = await res.json()
    all.push(...body.items)
    if (body.items.length === 0 || all.length >= body.total) return all
  }
}

// null - события нет (404).
export async function fetchEvent(id: number, signal: AbortSignal): Promise<HistEvent | null> {
  const res = await fetch(`${API}/events/${id}`, { signal, credentials: 'include' })
  if (res.status === 404) return null
  if (!res.ok) throw new Error(`event ${id}: HTTP ${res.status}`)
  return (await res.json()).event
}

export interface StarState { stars: number; starred: boolean }

// null - гость (401) или нет связи; вызывающий отправляет гостя на вход.
export async function setStar(id: number, on: boolean): Promise<StarState | 'guest' | null> {
  try {
    const res = await fetch(`${API}/events/${id}/star`, { method: on ? 'PUT' : 'DELETE', credentials: 'include' })
    if (res.status === 401) return 'guest'
    return res.ok ? res.json() : null
  } catch {
    return null
  }
}

// Адреса страниц: в URL только строчные, код профиля бэкенд принимает в любом регистре.
export const profilePath = (publicId: string) => `/user/${publicId.toLowerCase()}`
export const eventPath = (id: number) => `/event/${id}`

// Просмотр сайта на /event/:id. null - просмотр не пишется (автор смотрит своё) или нет связи.
export async function startView(id: number, visitorId: string): Promise<number | null> {
  const res = await postJSON<{ viewId: number }>(`/events/${id}/views`, { visitorId })
  return res.ok && res.data.viewId ? res.data.viewId : null
}

// sendBeacon переживает закрытие вкладки; он шлёт только POST, поэтому ручка POST.
export function touchView(id: number, viewId: number, visitorId: string, activeSeconds: number, scrollPercent: number) {
  beacon(`/events/${id}/views/${viewId}`, { visitorId, activeSeconds, scrollPercent })
}

function beacon(path: string, body: unknown) {
  navigator.sendBeacon(`${API}${path}`, new Blob([JSON.stringify(body)], { type: 'application/json' }))
}

let memoryVisitor = ''
// Случайный id браузера: по нему считаются уникальные гости. Без localStorage живёт до перезагрузки.
export function visitorId() {
  try {
    const saved = localStorage.getItem('hp-visitor')
    if (saved) return saved
  } catch { /* приватное окно */ }
  memoryVisitor ||= crypto.randomUUID()
  try { localStorage.setItem('hp-visitor', memoryVisitor) } catch { /* приватное окно */ }
  return memoryVisitor
}

// Действие для метрик панели. Через sendBeacon: переход по ссылке не обрывает запрос.
// step у form_step: 0 - форма открыта, 1..5 - шаги мастера, 6 - заявка отправлена.
// step у auth_step: гость пришёл на вход из "Предложить событие", 0 - открыл, 1 - запросил код, 2 - вошёл.
export function track(action: 'card_open' | 'card_click', eventId: number): void
export function track(action: 'form_step' | 'auth_step', step: number): void
export function track(action: string, value: number) {
  beacon('/metrics/actions', action.endsWith('_step')
    ? { action, step: value, visitorId: visitorId() }
    : { action, eventId: value, visitorId: visitorId() })
}

// showLastName - действует согласие на распространение: посторонним видна фамилия целиком.
export interface User { id: number; publicId: string; email: string; firstName: string; lastName: string; showLastName: boolean }

export type ApiResult<T = unknown> =
  | { ok: true; data: T }
  | { ok: false; status: number; code?: string; message?: string; fields?: Record<string, string>; retryAfter?: number }

// status 0 - нет связи.
async function postJSON<T>(path: string, body: unknown, method = 'POST'): Promise<ApiResult<T>> {
  let res: Response
  try {
    res = await fetch(`${API}${path}`, {
      method, credentials: 'include', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
    })
  } catch {
    return { ok: false, status: 0 }
  }
  const data = await res.json().catch(() => ({}))
  if (res.ok) return { ok: true, data }
  return {
    ok: false, status: res.status, code: data.error?.code, message: data.error?.message, fields: data.error?.fields,
    retryAfter: Number(res.headers.get('Retry-After')) || undefined,
  }
}

// Вход посетителя: код на почту, затем verify ставит куку. Регистрация создаёт учётку только после кода.
export const registerUser = (body: { firstName: string; lastName: string; email: string; consent: boolean }) => postJSON('/users/register', body)
export const requestLoginCode = (email: string) => postJSON('/users/login', { email })
export const verifyLoginCode = (email: string, code: string, remember: boolean) =>
  postJSON<{ user: User }>('/users/verify', { email, code, remember })
export const logoutUser = () => postJSON('/users/logout', {})
// Выдача или отзыв согласия на распространение фамилии.
export const setShowLastName = (enabled: boolean) => postJSON<{ user: User }>('/users/me/distribution', { enabled }, 'PUT')
// Удаление учётки: бэкенд гасит и сессию.
export const deleteAccount = () => postJSON('/users/me', {}, 'DELETE')

export interface Place { display_name: string; lat: string; lon: string }

// Поиск места идёт через наш сервер: IP посетителя к Nominatim не попадает.
export async function geocode(q: string, signal: AbortSignal): Promise<Place[]> {
  const res = await fetch(`${API}/geocode?q=${encodeURIComponent(q)}`, { signal })
  if (!res.ok) return []
  return (await res.json()).places
}

// null - не вошёл.
export async function fetchMe(signal: AbortSignal): Promise<User | null> {
  const res = await fetch(`${API}/users/me`, { signal, credentials: 'include' })
  if (res.status === 401) return null
  if (!res.ok) throw new Error(`me: HTTP ${res.status}`)
  return (await res.json()).user
}

export type SubmitResult =
  | { ok: true; publicId: string }
  | { ok: false; status: number; message?: string; fields?: Record<string, string>; retryAfter?: number }

// XHR, а не fetch: нужен прогресс загрузки архива. status 0 - нет связи, -1 - отменено.
export function submitProposal(body: FormData, onProgress: (loaded: number, total: number) => void) {
  const xhr = new XMLHttpRequest()
  const promise = new Promise<SubmitResult>(resolve => {
    xhr.open('POST', `${API}/submissions`)
    xhr.responseType = 'json'
    xhr.upload.onprogress = e => onProgress(e.loaded, e.total)
    xhr.onerror = () => resolve({ ok: false, status: 0 })
    xhr.onabort = () => resolve({ ok: false, status: -1 })
    xhr.onload = () => {
      const data = xhr.response ?? {}
      if (xhr.status === 201) return resolve({ ok: true, publicId: data.submission.publicId })
      resolve({
        ok: false, status: xhr.status, message: data.error?.message, fields: data.error?.fields,
        retryAfter: Number(xhr.getResponseHeader('Retry-After')) || undefined,
      })
    }
    xhr.send(body)
  })
  return { promise, abort: () => xhr.abort() }
}

// Есть ли index.html в корне ZIP: читаем только центральный каталог, архив не распаковываем.
// ponytail: ZIP64 не разбираем, для архивов до 20 МБ он не нужен.
export async function zipHasRootIndex(file: Blob): Promise<boolean> {
  const tail = new DataView(await file.slice(Math.max(0, file.size - 65557)).arrayBuffer())
  let eocd = -1
  for (let i = tail.byteLength - 22; i >= 0; i--) if (tail.getUint32(i, true) === 0x06054b50) { eocd = i; break }
  if (eocd < 0) throw new Error('not a zip')
  const count = tail.getUint16(eocd + 10, true)
  const size = tail.getUint32(eocd + 12, true)
  const offset = tail.getUint32(eocd + 16, true)
  const cd = new DataView(await file.slice(offset, offset + size).arrayBuffer())
  const decoder = new TextDecoder()
  for (let p = 0, n = 0; n < count; n++) {
    if (p + 46 > cd.byteLength || cd.getUint32(p, true) !== 0x02014b50) throw new Error('broken central directory')
    const nameLen = cd.getUint16(p + 28, true)
    const name = decoder.decode(new Uint8Array(cd.buffer, p + 46, nameLen))
    if (name.replace(/^\.\//, '') === 'index.html') return true
    p += 46 + nameLen + cd.getUint16(p + 30, true) + cd.getUint16(p + 32, true)
  }
  return false
}

export type WorkStatus = 'processing' | 'new' | 'review' | 'rejected' | 'published'

export interface Work {
  publicId: string
  title: string
  eventDate: string
  eventType: EventType
  status: WorkStatus
  siteUrl: string
  screenshotUrl: string
  comment?: string
  createdAt: string
  eventId?: number
  stars: number
  starred: boolean
  visitors: number
  // Только владельцу профиля.
  stats?: { views: number; visitors: number; activeSeconds: number }
}

export interface Profile {
  user: { publicId: string; firstName: string; lastName: string; createdAt: string }
  owner: boolean
  works: Work[]
}

// null - такого автора нет (404).
export async function fetchProfile(publicId: string, signal: AbortSignal): Promise<Profile | null> {
  const res = await fetch(`${API}/users/${encodeURIComponent(publicId)}`, { signal, credentials: 'include' })
  if (res.status === 404) return null
  if (!res.ok) throw new Error(`profile ${publicId}: HTTP ${res.status}`)
  return res.json()
}
