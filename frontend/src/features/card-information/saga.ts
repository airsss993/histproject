import { call, put, takeLatest } from 'redux-saga/effects'
import { cards } from '../../entities/cons'
// import { EventObject } from './ui/reurlcard'
import { EventObject, fetchAllEventsSuccess, fetchAllEventsFailure } from './slice'

export const FETCH_ALL_EVENTS = 'FETCH_ALL_EVENTS';

async function fetchAllEventsApi(): Promise<EventObject[]> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 10000);

    const results = await Promise.all(
        cards.map(url =>
            fetch(url, {
                method: 'GET',
                signal: controller.signal,
                mode: 'cors',
                headers: {
                    'Accept': 'application/json',
                    'Content-Type': 'application/json'
                }
            })
            .then(res => res.ok ? res.json().then((data: any) => {
                const obj = data.object;
                return {
                    id: obj.id,
                    title: obj.title,
                    description: obj.description,
                    latitude: obj.latitude,
                    longitude: obj.longitude,
                    eventDate: obj.eventDate,
                    eventType: obj.eventType,
                    previewUrlImage: obj.previewUrlImage,
                    siteUrl: obj.siteUrl ?? null
                } as EventObject;
            }) : null)
            .catch(() => null)
        )
    );

    clearTimeout(timeoutId);

    const events = results.filter((e): e is EventObject => e !== null);

    if (events.length === 0) {
        throw new Error('Нет данных в ответе сервера или неверный формат');
    }

    return events;
}

function* fetchAllEventsSaga(): Generator<any, void, EventObject[]> {
    try {
        const events: EventObject[] = yield call(fetchAllEventsApi);
        yield put(fetchAllEventsSuccess(events));
    } catch (error: any) {
        console.error('❌ [EventsDataService] Ошибка при загрузке данных:', error);
        yield put(fetchAllEventsFailure(error?.message || 'Unknown error'));
    }
}

export const formatDate = (dateString: string): string => {
    const date = new Date(dateString);
    if (isNaN(date.getTime())) return dateString;
    return date.toLocaleDateString('ru-RU', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).replace(/\//g, '.');
  };

export function* watchFetchAllEvents(): Generator<any, void, any> {
    yield takeLatest(FETCH_ALL_EVENTS, fetchAllEventsSaga);
}