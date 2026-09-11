import { call, put, takeLatest } from 'redux-saga/effects'
import { PayloadAction } from '@reduxjs/toolkit'
import { ApiResponse, EventDates, FilterRequestData, buildFilterRequestData } from './ui/typeven'
import { eventsListDates } from '../../entities/cons'
import {
  setEvents,
  setIsLoading,
  setError,
  resetFilterState,
  ApplyFiltersPayload,
} from './slice'
import { EventObject } from '../marker-location/evenPositions'

export async function fetchEventsAPI(): Promise<EventDates[]> {
  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 10000);

    const response = await fetch(eventsListDates, {
      method: 'GET',
      signal: controller.signal,
      mode: 'cors',
      headers: {
        'Accept': 'application/json',
        'Content-Type': 'application/json',
      },
    });

    clearTimeout(timeoutId);
    if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
    const data: ApiResponse = await response.json();
    return Array.isArray(data?.objects) ? data.objects : [];
  } catch (error) {
    console.error('Failed to fetch events:', error);
    return [];
  }
}

export async function fetchEventsByFiltersAPI(payload: FilterRequestData): Promise<EventDates[]> {
  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 10000);

    console.log('Отправляемый payload на бэкенд:', payload);

    const params = new URLSearchParams();
    if (payload.eventTypeIds?.length) {
      payload.eventTypeIds.forEach(id => params.append('eventTypeIds', String(id)));
    }
    if (payload.dateFrom) params.append('dateFrom', payload.dateFrom);
    if (payload.dateTo) params.append('dateTo', payload.dateTo);

    const url = `${eventsListDates}?${params.toString()}`;

    const response = await fetch(url, {
      method: 'GET',
      signal: controller.signal,
      mode: 'cors',
      headers: { 'Accept': 'application/json' },
    });

    clearTimeout(timeoutId);
    if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
    const data: ApiResponse = await response.json();
    return Array.isArray(data?.objects) ? data.objects : [];
  } catch (error) {
    console.error('Failed to fetch events by filters:', error);
    return [];
  }
}

// ---- Saga Workers ----

function* loadEventsSaga() {
  try {
    yield put(setIsLoading(true));
    yield put(setError(null));
    const events: EventDates[] = yield call(fetchEventsAPI);
    yield put(setEvents(events as EventObject[]));
  } catch (err) {
    console.error('Failed to load events:', err);
    yield put(setError('Не удалось загрузить события'));
  } finally {
    yield put(setIsLoading(false));
  }
}

function* applyFiltersSaga(action: PayloadAction<ApplyFiltersPayload>) {
  try {
    yield put(setIsLoading(true));
    yield put(setError(null));

    if (action.payload.hideAllMarkers) {
      yield put(setEvents([]));
      return;
    }

    const payload = buildFilterRequestData({
      selectedOptions: action.payload.selectedOptions,
      periodRange: action.payload.periodRange,
      selectedPeriod: action.payload.selectedPeriod,
    });

    const events: EventDates[] = yield call(fetchEventsByFiltersAPI, payload);
    yield put(setEvents(events as EventObject[]));
  } catch (err) {
    console.error('Failed to apply filters:', err);
    yield put(setError('Не удалось применить фильтры'));
  } finally {
    yield put(setIsLoading(false));
  }
}

function* resetAndReloadSaga() {
  yield put(resetFilterState());
  yield* loadEventsSaga();
}

// ---- Saga Watchers ----

export function* watchFilterSagas() {
  yield takeLatest('filter/fetchEventsRequest', loadEventsSaga);
  yield takeLatest('filter/applyFiltersRequest', applyFiltersSaga);
  yield takeLatest('filter/resetAndReloadRequest', resetAndReloadSaga);
}
