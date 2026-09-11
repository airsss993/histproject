import { configureStore } from '@reduxjs/toolkit'
import createSagaMiddleware from 'redux-saga'
import rootSaga from './saga/rootSaga'
import { MainContent } from '../features/main-content'
import cardReducer from './saga/cardSlice'
import eventsReducer from './saga/slice' // импорт slice.ts как eventsReducer
import { LogInFeature } from '../features/login'
import { LogOutFeature } from '../features/logout'
import { ReFreshFeature } from '../features/refresh'
import { FormFeature } from '../features/form'

const sagaMiddleware = createSagaMiddleware();

export const store = configureStore({
  reducer: {
    ...MainContent.reducer,
    card: cardReducer,
    events: eventsReducer, // добавь эту строку
    ...LogInFeature.reducer,
    ...LogOutFeature.reducer,
    ...ReFreshFeature.reducer,
    ...FormFeature.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware({
      thunk: false,
      serializableCheck: false,
    }).concat(sagaMiddleware),
});

sagaMiddleware.run(ReFreshFeature.sagas.init);
sagaMiddleware.run(rootSaga);

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
