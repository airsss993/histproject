import React, { lazy, Suspense } from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Splash } from '../shared/ui/Loader.tsx'
import { CookieNotice } from '../shared/ui/CookieNotice.tsx'
import '../pages/public/public.css'

// Страницы грузятся отдельными чанками: карта (MapLibre) не попадает в первый экран.
const PublicMap = lazy(() => import('../pages/public/PublicMap.tsx').then(m => ({ default: m.PublicMap })))
const About = lazy(() => import('../pages/public/About.tsx').then(m => ({ default: m.About })))
const AuthPage = lazy(() => import('../pages/public/Auth.tsx').then(m => ({ default: m.AuthPage })))
const ProfilePage = lazy(() => import('../pages/public/Profile.tsx').then(m => ({ default: m.ProfilePage })))
const Panel = lazy(() => import('../pages/panel/Panel.tsx').then(m => ({ default: m.Panel })))
const Viewer = lazy(() => import('../pages/public/Viewer.tsx').then(m => ({ default: m.Viewer })))
const Guide = lazy(() => import('../pages/public/Guide.tsx').then(m => ({ default: m.Guide })))
const Legal = lazy(() => import('../pages/public/Legal.tsx').then(m => ({ default: m.Legal })))
const NotFound = lazy(() => import('../pages/public/NotFound.tsx').then(m => ({ default: m.NotFound })))

const rootElement = document.getElementById('root');
  if (rootElement) {
    ReactDOM.createRoot(rootElement).render(
      <React.StrictMode>
        <BrowserRouter>
          <Suspense fallback={null}>
            <Routes>
              <Route path="/" element={<Navigate to="/map" replace />} />
              <Route path="/app" element={<Navigate to="/map" replace />} />
              {/* Пока грузится код карты - та же заставка, что потом покажет сама карта. */}
              <Route path="/map" element={<Suspense fallback={<Splash phase="load" />}><PublicMap /></Suspense>} />
              <Route path="/about" element={<About />} />
              <Route path="/guide" element={<Guide />} />
              <Route path="/login" element={<AuthPage key="login" mode="login" />} />
              <Route path="/register" element={<AuthPage key="register" mode="register" />} />
              {['/privacy', '/rules', '/consent/registration', '/consent/submission', '/consent/distribution'].map(p => (
                <Route key={p} path={p} element={<Legal key={p} path={p} />} />
              ))}
              <Route path="/user/:id" element={<ProfilePage />} />
              <Route path="/u/:id" element={<ProfilePage />} />
              <Route path="/event/:id" element={<Viewer />} />
              <Route path="/admin" element={<Panel />} />
              {['/log', '/adm', '/mad', '/his', '/mod'].map(p => <Route key={p} path={p} element={<Navigate to="/admin" replace />} />)}
              <Route path="*" element={<NotFound />} />
            </Routes>
          </Suspense>
          <CookieNotice />
        </BrowserRouter>
      </React.StrictMode>
    );
  }