import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Provider } from 'react-redux'
import { store } from './store.ts'
import { PublicMap } from '../pages/public/PublicMap.tsx'
import { NotFound } from '../pages/public/NotFound.tsx'
import { LogIn } from '../pages/login/login.tsx'
import { Admin } from '../pages/admin/admin.tsx'
import { HisLog } from '../pages/hislog/hislog.tsx'
import { Moder } from '../pages/moder/moder.tsx'
import { Madder } from '../pages/admader/madder.tsx'

const rootElement = document.getElementById('root');
  if (rootElement) {
    ReactDOM.createRoot(rootElement).render(
      <React.StrictMode>
        <Provider store={store}>
          <BrowserRouter>
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
          </BrowserRouter>
        </Provider>
      </React.StrictMode>
    );
  }