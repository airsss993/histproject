export type EventType = 'military' | 'political'

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
}

const API = `${import.meta.env.VITE_API_BASE ?? ''}/api/v1`

export const TYPE_LABEL: Record<EventType, string> = { military: 'Военное', political: 'Политическое' }
export const TYPE_PLURAL: Record<EventType, string> = { military: 'Военные', political: 'Политические' }
export const TYPES: EventType[] = ['military', 'political']

export const yearOf = (e: HistEvent) => parseInt(e.eventDate.slice(0, 4), 10)
export const formatDate = (iso: string) => iso.slice(0, 10).split('-').reverse().join('.')

// Бекенд отдаёт не больше 100 на страницу, карта нужна целиком.
export async function fetchAllEvents(signal: AbortSignal): Promise<HistEvent[]> {
  const all: HistEvent[] = []
  for (let page = 1; ; page++) {
    const res = await fetch(`${API}/events?page=${page}&pageSize=100`, { signal })
    if (!res.ok) throw new Error(`events: HTTP ${res.status}`)
    const body: { items: HistEvent[]; total: number } = await res.json()
    all.push(...body.items)
    if (body.items.length === 0 || all.length >= body.total) return all
  }
}

// null - события нет (404).
export async function fetchEvent(id: number, signal: AbortSignal): Promise<HistEvent | null> {
  const res = await fetch(`${API}/events/${id}`, { signal })
  if (res.status === 404) return null
  if (!res.ok) throw new Error(`event ${id}: HTTP ${res.status}`)
  return (await res.json()).event
}
