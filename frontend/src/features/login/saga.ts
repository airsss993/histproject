import { takeLatest, put, call, select } from 'redux-saga/effects'
import { adminLogIn, admins } from '../../entities/cons'
import { actions } from './slice'
import { selectors } from './selectors'

function* handleAdminLogIn(): Generator<any, void, any> {
  try {
    const login: string = yield select(selectors.selectLogin);
    const password: string = yield select(selectors.selectPassword);

    const response: Response = yield call(fetch, adminLogIn, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ login, password }),
    });

    if (!response.ok) {
      if (response.status === 401) {
        yield put(actions.loginFailure('Неверный логин или пароль'));
      } else {
        yield put(actions.loginFailure('Ошибка сервера'));
      }
      return;
    }

    let role: string | null = null;
    const data = yield call([response, 'json']);
    role = data?.role ?? data?.type ?? data?.userRole ?? null;

    if (!role) {
      const check: Response = yield call(fetch, admins, {
        method: 'GET',
        credentials: 'include',
      });
      role = check.ok ? 'super_admin' : check.status === 403 ? 'moderator' : null;
    }

    localStorage.setItem('username', login);
    yield put(actions.loginSuccess({ role, username: login }));
  } catch (error) {
    yield put(actions.loginFailure('Ошибка подключения к серверу'));
  }
}

export function* LogInInit(): Generator<any, void, any> {
  yield takeLatest(actions.loginRequest, handleAdminLogIn);
}