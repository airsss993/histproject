import React from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { MapView } from './MapView.tsx'
import { Logo } from './PublicMap.tsx'
import './public.css'

const noop = () => {}
const none = () => false

export const NotFound: React.FC<{ path?: string; kind?: 'event' | 'page' }> = ({ path, kind = 'page' }) => {
  const navigate = useNavigate()
  const loc = useLocation()
  return (
    <div className="pm-root pm-404">
      <div className="pm-404-map">
        <MapView events={[]} isActive={none} selectedId={null} onSelect={noop} pad={48} labels={false} sea="#08080A" interactive={false} />
      </div>
      <div role="banner" className="pm-header pm-header-plain">
        <Logo onClick={() => navigate('/map')} />
      </div>
      <main className="pm-404-body">
        <div className="pm-404-code">404</div>
        <h1>{kind === 'event' ? 'Событие не найдено' : 'Страница не найдена'}</h1>
        <p>
          {kind === 'event' ? 'Событие удалено с карты или ссылка неверна.' : 'Такой страницы нет.'}
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
