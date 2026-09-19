import React, { lazy, Suspense } from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Provider } from 'react-redux'
import { store } from './store.ts'

// Страницы грузятся отдельными чанками: карта (MapLibre) не попадает в первый экран.
const PublicMap = lazy(() => import('../pages/public/PublicMap.tsx').then(m => ({ default: m.PublicMap })))
const About = lazy(() => import('../pages/public/About.tsx').then(m => ({ default: m.About })))
const AuthPage = lazy(() => import('../pages/public/Auth.tsx').then(m => ({ default: m.AuthPage })))
const Panel = lazy(() => import('../pages/panel/Panel.tsx').then(m => ({ default: m.Panel })))
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
              <Route path="/about" element={<About />} />
              <Route path="/login" element={<AuthPage key="login" mode="login" />} />
              <Route path="/register" element={<AuthPage key="register" mode="register" />} />
              <Route path="/admin" element={<Panel />} />
              {['/log', '/adm', '/mad', '/his', '/mod'].map(p => <Route key={p} path={p} element={<Navigate to="/admin" replace />} />)}
              <Route path="*" element={<NotFound />} />
            </Routes>
            </Suspense>
          </BrowserRouter>
        </Provider>
      </React.StrictMode>
    );
  }