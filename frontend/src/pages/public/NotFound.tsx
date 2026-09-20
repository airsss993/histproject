import React from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { MapView } from './MapView.tsx'
import { Logo } from './PublicMap.tsx'
import './public.css'

const noop = () => {}
const none = () => false

const TITLE = { event: 'Событие не найдено', profile: 'Автор не найден', page: 'Страница не найдена' }
const TEXT = {
  event: 'Событие удалено с карты или ссылка неверна.',
  profile: 'Такой учётки нет или её удалили.',
  page: 'Такой страницы нет.',
}

export const NotFound: React.FC<{ path?: string; kind?: 'event' | 'profile' | 'page' }> = ({ path, kind = 'page' }) => {
  const navigate = useNavigate()
  const loc = useLocation()
  return (
    <div className="pm-root pm-404">
      <div className="pm-404-map">
        <MapView events={[]} isActive={none} selectedId={null} onSelect={noop} pad={48} labels={false} sea="#010011" interactive={false} />
      </div>
      <div role="banner" className="pm-header pm-header-plain">
        <Logo onClick={() => navigate('/map')} />
      </div>
      <main className="pm-404-body">
        <div className="pm-404-code">404</div>
        <h1>{TITLE[kind]}</h1>
        <p>
          {TEXT[kind]}
          {' '}Вернитесь к карте - на ней события с IX по XXI век.
        </p>
        <div className="pm-row">
          <button type="button" className="pm-btn" onClick={() => navigate('/map')}>Перейти к карте</button>
          <button type="button" className="pm-btn-ghost" onClick={() => navigate('/map?propose=1')}>Предложить событие</button>
        </div>
        <div className="pm-mono pm-dim-text">{path ?? loc.pathname + loc.search}</div>
      </main>
    </div>
  )
}
