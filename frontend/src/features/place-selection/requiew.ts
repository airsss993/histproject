import { PublForm } from '../../entities/cons'

export const submitMarkerReview = async (
  id: number,
  lat: number,
  lng: number,
): Promise<void> => {
  const response = await fetch(PublForm(id), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ latitude: lat, longitude: lng }),
  })
  if (!response.ok) {
    throw new Error(`submitMarkerReview failed: ${response.status}`)
  }
}
