import { adminCard } from '../../../entities/cons'
import { Request } from './reques'

export const fetchRequestCard = async (id: string | number): Promise<Request | null> => {
    try {
        const response = await fetch(adminCard(Number(id)), {
            method: 'GET',
            credentials: 'include',
        })
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`)
        }
        const item = await response.json()
        console.log('[fetchRequestCard] raw response:', item)
        console.log('[fetchRequestCard] mapped fields:', {
            id: item.request.id,
            title: item.request.title,
            description: item.request.description,
            eventDate: item.request.event_date ?? item.request.eventDate,
            eventType: item.request.event_type ?? item.request.eventType,
            telegram: item.request.telegram,
            email: item.request.email,
            siteUrl: item.request.siteUrl ?? '',
            adminComment: item.request.adminComment ?? '',
        })
        const r = item.request ?? item
        return {
            id: String(r.id),
            title: r.title ?? '',
            date: r.createdAt ?? r.date ?? '',
            status: r.status ?? 'new',
            description: r.description ?? '',
            eventDate: r.event_date ?? r.eventDate ?? '',
            eventTypeId: r.eventTypeId ?? r.event_type_id ?? r.event_type ?? r.eventType ?? '',
            telegram: r.telegram ?? r.telegramUsername ?? r.telegram_username ?? '',
            email: r.email ?? '',
            siteUrl: r.siteUrl ?? r.site_url ?? r.url ?? r.website ?? r.preview_url ?? '',
            adminComment: item.request.adminComment ?? '',
        }
    } catch (err) {
        console.error(`Ошибка загрузки заявки #${id}:`, err)
        return null
    }
}
