import { all, fork } from 'redux-saga/effects'
import { watchFetchData, watchFetchCardData } from './saga'
import { LogInFeature } from '../../features/login'
import { LogOutFeature } from '../../features/logout'
import { ReFreshFeature } from '../../features/refresh'
import { FormFeature } from '../../features/form'
import { watchFetchAllEvents } from '../../features/card-information/saga'

export default function* rootSaga() {
  yield all([
    watchFetchData(),
    watchFetchCardData(),
    fork(LogInFeature.sagas.init),
    fork(LogOutFeature.sagas.init),
    fork(ReFreshFeature.sagas.init),
    fork(FormFeature.sagas.init),
    watchFetchAllEvents(),
  ]);
}
