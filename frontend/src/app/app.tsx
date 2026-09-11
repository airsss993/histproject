import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Provider } from 'react-redux'
import { store } from './store.ts'
import { Index } from '../pages/index/index.tsx'
import { Main } from '../pages/main/main.tsx'
import { Mapp } from '../pages/map/mapp.tsx'
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
              <Route path="/" element={<Index />} />
              <Route path="/app" element={<Main />} />
              <Route path="/map" element={<Mapp />} />
              <Route path="/log" element={<LogIn />} />
              <Route path="/adm" element={<Admin />} />
              <Route path="/mad" element={<Madder />} />
              <Route path="/his" element={<HisLog />} />
              <Route path="/mod" element={<Moder />} />
            </Routes>
          </BrowserRouter>
        </Provider>
      </React.StrictMode>
    );
  }