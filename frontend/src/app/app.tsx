import React, { lazy, Suspense } from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Provider } from 'react-redux'
import { store } from './store.ts'
import { LogIn } from '../pages/login/login.tsx'
import { Admin } from '../pages/admin/admin.tsx'
import { HisLog } from '../pages/hislog/hislog.tsx'
import { Moder } from '../pages/moder/moder.tsx'
import { Madder } from '../pages/admader/madder.tsx'

// Карта (MapLibre) грузится отдельным чанком, админка её не тянет.
const PublicMap = lazy(() => import('../pages/public/PublicMap.tsx').then(m => ({ default: m.PublicMap })))
const NotFound = lazy(() => import('../pages/public/NotFound.tsx').then(m => ({ default: m.NotFound })))

const rootElement = document.getElementById('root');
  if (rootElement) {
    ReactDOM.createRoot(rootElement).render(
      <React.StrictMode>
        <Provider store={store}>
          <BrowserRouter>
            <Suspense fallback={null}>
            <Routes>
              <Route path="/" element={<Navigate to="/map" replace />} />
              <Route path="/app" element={<Navigate to="/map" replace />} />
              <Route path="/map" element={<PublicMap />} />
              <Route path="/log" element={<LogIn />} />
              <Route path="/adm" element={<Admin />} />
              <Route path="/mad" element={<Madder />} />
              <Route path="/his" element={<HisLog />} />
              <Route path="/mod" element={<Moder />} />
              <Route path="*" element={<NotFound />} />
            </Routes>
            </Suspense>
          </BrowserRouter>
        </Provider>
      </React.StrictMode>
    );
  }