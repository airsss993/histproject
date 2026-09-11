import { DelEven } from '../../../entities/cons'

export const deleteObject = async (id: number): Promise<boolean> => {
    try {
        const response = await fetch(DelEven(id), {
            method: 'DELETE',
            credentials: 'include',
        })
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`)
        }
        const json = await response.json()
        console.log('[deleteObject] response:', json)
        return json?.message === 'success'
    } catch (err) {
        console.error(`Ошибка удаления объекта заявки #${id}:`, err)
        return false
    }
}
