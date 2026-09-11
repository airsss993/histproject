// import { useEffect, useCallback } from 'react'
// import { EventObject } from '../../features/marker-location/evenPositions'
// import { fetchEvents } from '../../features/filter-function/typeven.ts'
// import { setEvents, resetFilters, setIsLoading, setError } from '../../features/filter-function/evenFilterProvider/evenFilterProvider.tsx'
import { takeLatest, put, call } from 'redux-saga/effects'
import { PayloadAction } from '@reduxjs/toolkit'
import { api, FetchDataPayload, DataResponse } from '../../entities/api.ts'

export type { FetchDataPayload, DataResponse }

const FETCH_DATA = 'FETCH_DATA'; export const FETCH_CARD_DATA = 'FETCH_CARD_DATA';
const FETCH_DATA_SUCCESS = 'FETCH_DATA_SUCCESS'; export const FETCH_CARD_DATA_SUCCESS = 'FETCH_CARD_DATA_SUCCESS';
const FETCH_DATA_FAILURE = 'FETCH_DATA_FAILURE'; export const FETCH_CARD_DATA_FAILURE = 'FETCH_CARD_DATA_FAILURE';

function* fetchData(action: PayloadAction<FetchDataPayload>): Generator<any, void, DataResponse> {
  try {
    const response: DataResponse = yield call(api.fetchData, action.payload);

    yield put({
      type: FETCH_DATA_SUCCESS,
      payload: response.data
    });

  } catch (error: any) {
    yield put({
      type: FETCH_DATA_FAILURE,
      payload: error?.message || 'Unknown error'
    });
  }
}

function* fetchCardData(action: PayloadAction<number>): Generator<any, void, any> {
  try {
    const data: any = yield call(api.fetchCard, action.payload);
    console.log(`[Card API] id: ${action.payload}, siteUrl: ${data.object?.siteUrl ?? 'null'}, typEvent: ${data.object?.eventType}, coords: (${data.object?.latitude}, ${data.object?.longitude})`, data.object);
    yield put({
      type: FETCH_CARD_DATA_SUCCESS,
      payload: data.object
    });
  } catch (error: any) {
    yield put({
      type: FETCH_CARD_DATA_FAILURE,
      payload: error?.message || 'Unknown error'
    });
  }
}

export function* watchFetchData(): Generator<any, void, any> {
  yield takeLatest(FETCH_DATA, fetchData);
}

export function* watchFetchCardData(): Generator<any, void, any> {
  yield takeLatest(FETCH_CARD_DATA, fetchCardData);
}

  // export const loadInitialEvents = useCallback(async () => {
  //   try {
  //     setIsLoading(true);
  //     setError(null);
  //     const fetchedEvents = await fetchEvents();
  //     setEvents(Array.isArray(fetchedEvents) ? (fetchedEvents as EventObject[]) : []);
  //   } catch (err) {
  //     console.error('Failed to load events:', err);
  //     setError('Не удалось загрузить события');
  //   } finally {
  //     setIsLoading(false);
  //   }
  // }, []);

  // useEffect(() => {
  //   loadInitialEvents();
  // }, [loadInitialEvents]);

  // export const resetAndReload = useCallback(async () => {
  //   resetFilters();
  //   await loadInitialEvents();
  // }, [loadInitialEvents, resetFilters]);