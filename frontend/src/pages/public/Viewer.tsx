import React, { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { NotFound } from './NotFound.tsx'
import { LoadMark } from '../../shared/ui/Loader.tsx'
import { fetchEvent } from './api.ts'
import './public.css'

// Старые ссылки открывают сайт напрямую, без рамки и учёта просмотров.
export const Viewer: React.FC = () => {
  const { id } = useParams()
  const valid = !!id && /^\d+$/.test(id)
  const [status, setStatus] = useState<'load' | 'missing' | 'failed'>('load')
  const [retry, setRetry] = useState(0)

  useEffect(() => {
    if (!valid) return
    const ac = new AbortController()
    setStatus('load')
    fetchEvent(Number(id), ac.signal).then(event => {
      if (event) window.location.replace(event.siteUrl)
      else setStatus('missing')
    }, (e: Error) => { if (e.name !== 'AbortError') setStatus('failed') })
    return () => ac.abort()
  }, [id, valid, retry])

  if (!valid || status === 'missing') return <NotFound path={`/event/${id}`} kind="event" />
  return (
    <div className="pm-root">
      {status === 'failed' ? (
        <div className="pm-state pm-glass pm-state-error" role="alert">
          <div className="pm-state-icon">!</div>
          <div className="pm-state-title">Нет связи с сервером</div>
          <div className="pm-state-text">Не удалось открыть сайт события.</div>
          <div className="pm-row">
            <button type="button" className="pm-btn" onClick={() => setRetry(r => r + 1)}>Повторить</button>
            <Link className="pm-btn-ghost" to={`/map?event=${id}`}>К карте</Link>
          </div>
        </div>
      ) : (
        <div className="pm-state pm-glass pm-state-loading" role="status">
          <LoadMark done={false} />
          <div className="pm-state-title">Открываем сайт...</div>
        </div>
      )}
    </div>
  )
}
