import { takeLatest, put, call, select } from 'redux-saga/effects'
import { actions } from './slice'
import { selectEventPayload, selectEmail, selectTelegram, selectIsoDate, selectEventTypeId, selectIsFileSizeValid } from './selectors'
import { formApi } from '../../entities/cons'

function validateFormData(_eventPayload: any, eventDate: string | null, eventTypeId: number | null, isFileValid: boolean): string | null {
  if (!eventDate) {
    return 'Некорректная дата. Используйте формат ДД.ММ.ГГГГ!'
  }
  
  if (!eventTypeId) {
    return 'Некорректный тип события.'
  }
  
  if (!isFileValid) {
    return 'Необходим сайт в ZIP-архиве с размером менее 350 МБ. Внутри архива в корне или на первом уровне вложенности должен быть index.html.'
  }
  
  return null
}

function* handleSubmitForm(): Generator<any, void, any> {
  try {
    const eventPayload: any = yield select(selectEventPayload)
    const email: string = yield select(selectEmail)
    const telegram: string = yield select(selectTelegram)
    const eventDate: string | null = yield select(selectIsoDate)
    const eventTypeId: number | null = yield select(selectEventTypeId)
    const isFileValid: boolean = yield select(selectIsFileSizeValid)

    if (!eventPayload) {
      yield put(actions.submitFailure('Данные события не найдены'))
      return
    }

    const validationError = validateFormData(eventPayload, eventDate, eventTypeId, isFileValid)
    if (validationError) {
      yield put(actions.submitFailure(validationError))
      return
    }

    const formData = new FormData()
    formData.append('title', eventPayload.name)
    formData.append('description', eventPayload.description)
    if (eventPayload.zipFile) {
      formData.append('archive', eventPayload.zipFile)
    }
    formData.append('email', email)
    formData.append('telegramUsername', telegram)
    formData.append('eventDate', eventDate!)
    formData.append('eventTypeId', String(eventTypeId!))

    const res = yield call(fetch, formApi, {
      method: 'POST',
      body: formData,
      headers: {
        'Accept': 'application/json',
      },
    })

    if (!res.ok) {
      let serverMessage = ''
      try {
        const errBody = yield call([res, 'json'])
        serverMessage = errBody?.message || errBody?.error || JSON.stringify(errBody)
      } catch {
        try {
          serverMessage = yield call([res, 'text'])
        } catch {
          serverMessage = ''
        }
      }
      throw new Error(serverMessage || `HTTP error! status: ${res.status}`)
    }

    const data = yield call([res, 'json'])
    if (data.message !== 'success') {
      throw new Error('Unexpected response format')
    }
    
    yield put(actions.submitSuccess())
  } catch (err) {
    const msg = err instanceof Error ? err.message : ''
    if (!msg || msg.includes('NetworkError') || msg.includes('Failed to fetch')) {
      yield put(actions.submitFailure('Не удалось подключиться к серверу. Возможно, архив слишком большой или проблема с соединением.'))
    } else {
      yield put(actions.submitFailure(`Не удалось отправить заявку: ${msg}`))
    }
  }
}

export function* formInit(): Generator<any, void, any> {
  yield takeLatest(actions.submitRequest.type, handleSubmitForm)
}