import { HisForm } from '../../../entities/cons'

type ActionType = 'rejected' | 'approved' | 'review'

const ACTION_LABEL: Record<ActionType, string> = {
  rejected: 'Отклонение заявки',
  approved: 'Одобрение заявки',
  review:   'Взята на проверку',
}

const ACTION_TYPE_MAP: Record<string, ActionType> = {
  'approved': 'approved',
  'rejected': 'rejected',
  'review': 'review',
  'Одобрена': 'approved',
  'Одобрено': 'approved',
  'Оклонена': 'rejected',
  'Отклонена': 'rejected',
  'Отклонено': 'rejected',
  'Отклонен': 'rejected',
  'На проверке': 'review',
  'На проверку': 'review',
  'В проверке': 'review',
  'одобрена': 'approved',
  'одобрено': 'approved',
  'отклонена': 'rejected',
  'отклонено': 'rejected',
  'на проверке': 'review',
}

export interface LogEntry {
  id: string
  action: ActionType
  actionLabel: string
  requestTitle: string
  requestId: string
  comment: string
  adminLogin: string
  date: string
}

export interface FetchHistoryParams {
  q?: string
  page?: number
  limit?: number
}

export interface FetchHistoryResult {
  entries: LogEntry[]
  total: number
}

export const fetchHistory = async (params: FetchHistoryParams = {}): Promise<FetchHistoryResult> => {
  const query = new URLSearchParams()
  if (params.q)     query.set('q',     params.q)
  if (params.page)  query.set('page',  String(params.page))
  if (params.limit) query.set('limit', String(params.limit))

  const url = query.toString() ? `${HisForm}?${query}` : HisForm

  const response = await fetch(url, {
    method: 'GET',
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`) // выбрасываем ошибку
  }

  const json = await response.json()
  console.log('[fetchHistory] raw response:', json)

  const data: any[] = Array.isArray(json?.history) ? json.history
    : Array.isArray(json?.data)    ? json.data
    : Array.isArray(json?.results) ? json.results
    : Array.isArray(json?.items)   ? json.items
    : Array.isArray(json)          ? json
    : []

  const total: number = typeof json?.total === 'number' ? json.total : data.length

  const entries = data.map((item): LogEntry => {
    let rawAction = item.action ?? item.status ?? item.type ?? 'review'
    rawAction = String(rawAction).trim()

    let action: ActionType = ACTION_TYPE_MAP[rawAction] ?? 'approved'

    if (!ACTION_TYPE_MAP[rawAction]) {
      const lowerAction = rawAction.toLowerCase()
      if (lowerAction.includes('одобр')) action = 'approved'
      else if (lowerAction.includes('откл')) action = 'rejected'
      else if (lowerAction.includes('провер')) action = 'review'
    }

    console.log(`[fetchHistory] action mapping: "${rawAction}" -> "${action}"`)

    return {
      id:           String(item.id || item._id || Date.now()),
      action,
      actionLabel:  ACTION_LABEL[action] ?? rawAction ?? action,
      requestTitle: item.requestTitle  ?? item.request_title  ?? item.title ?? '',
      requestId:    String(item.requestId ?? item.request_id ?? item.id ?? ''),
      comment:      item.comment       ?? item.description    ?? item.note ?? '',
      adminLogin:   item.adminLogin    ?? item.admin_login    ?? item.moderator ?? item.admin ?? '',
      date:         item.createdAt     ?? item.created_at     ?? item.date ?? item.timestamp ?? new Date().toISOString(),
    }
  })

  return { entries, total }
}