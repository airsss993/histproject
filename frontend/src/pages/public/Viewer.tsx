import React, { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { NotFound } from './NotFound.tsx'
import { fetchEvent } from './api.ts'

// Старые ссылки открывают сайт напрямую, без рамки и учёта просмотров.
export const Viewer: React.FC = () => {
  const { id } = useParams()
  const valid = !!id && /^\d+$/.test(id)
  const [missing, setMissing] = useState(false)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    if (!valid) return
    const ac = new AbortController()
    setMissing(false)
    setFailed(false)
    fetchEvent(Number(id), ac.signal).then(event => {
      if (event) window.location.replace(event.siteUrl)
      else setMissing(true)
    }, (e: Error) => { if (e.name !== 'AbortError') setFailed(true) })
    return () => ac.abort()
  }, [id, valid])

  if (!valid || missing) return <NotFound path={`/event/${id}`} kind="event" />
  return <div className="pm-root">{failed ? 'Нет связи с сервером' : 'Открываем сайт...'}</div>
}
