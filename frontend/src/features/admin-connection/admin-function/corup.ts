import { UpdEven } from '../../../entities/cons'

export interface UpdateCoordinatesParams {
    id: number
    latitude: number
    longitude: number
}

export const updateObjectCoordinates = async (params: UpdateCoordinatesParams): Promise<boolean> => {
    try {
        const response = await fetch(UpdEven(params.id), {
            method: 'PATCH',
            credentials: 'include',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                latitude: params.latitude,
                longitude: params.longitude,
            }),
        })
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`)
        }
        const json = await response.json()
        console.log('[updateObjectCoordinates] response:', json)
        return json?.message === 'success'
    } catch (err) {
        console.error(`Ошибка обновления координат заявки #${params.id}:`, err)
        return false
    }
}
