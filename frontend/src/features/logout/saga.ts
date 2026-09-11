import { takeLatest, put, call } from 'redux-saga/effects'
import { adminLogOut } from '../../entities/cons'
import { actions } from './slice'

function* handleAdminLogOut(): Generator<any, void, any> {
  try {
    const response: Response = yield call(fetch, adminLogOut, {
      method: 'POST',
      credentials: 'include',
    });
    
    console.log('Logout response status:', response.status);
    
    yield put(actions.logoutSuccess());
    console.log('logoutSuccess dispatched');
    
  } catch (error) {
    console.log('Ошибка при выходе из аккаунта:', error);
  }
}

export function* LogOutInit(): Generator<any, void, any> {
  yield takeLatest(actions.logoutRequest, handleAdminLogOut);
}
