import React, { useState, useEffect } from 'react'
import '../../pages/hislog/hislog.scss'
import { useNavigate } from 'react-router-dom'
import { HisLogCard } from '../admin-connection/hislog-function/hiscard'
import { fetchHistory, LogEntry } from '../admin-connection/hislog-function/hisreques'
import { HisSearch } from '../admin-connection/hislog-function/hisearch'

const PAGE_SIZE = 9

export const HisLogContentComponent: React.FC = () => {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const navigate = useNavigate()

  useEffect(() => {
    setLoading(true)
    fetchHistory({ q: search || undefined, page, limit: PAGE_SIZE })
      .then(({ entries, total }) => {
        setEntries(entries)
        setTotal(total)
        setLoading(false)
      })
      .catch(() => {
        navigate('/log')
      })
  }, [search, page, navigate])

  const totalPages = Math.ceil(total / PAGE_SIZE)

  const handleSearch = (v: string) => { setSearch(v); setPage(1) }

  return (
    <div className="hislog">
      <div className="hislog__header">
        <svg width="28" height="28" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <circle cx="12" cy="12" r="9.5" stroke="#1a1a1a" strokeWidth="1.6" />
          <path d="M12 7V12.5L15.5 15" stroke="#1a1a1a" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        <h1>История изменений</h1>
        <p>Все действия модераторов по заявкам</p>
      </div>
      <HisSearch value={search} onChange={handleSearch} />
      <p className="hislog__count">
        Всего записей:&nbsp;<strong>{total}</strong>
      </p>
      {loading ? (
        <div className="hislog__loading">Загрузка...</div>
      ) : (
        <div className="hislog__timeline">
          {entries.map(entry => (
            <div key={entry.id} className="hislog__entry">
              <div className="hislog__dot" />
              <HisLogCard entry={entry} />
            </div>
          ))}
        </div>
      )}

      <div className="hislog__pagination">
        <span>Страница {page} из {totalPages || 1}</span>
        <div className="hislog__pagination-btns">
          <button
            className="hislog__page-btn"
            disabled={page <= 1}
            onClick={() => setPage(p => p - 1)}
          >
            {'<'} Назад
          </button>
          <button
            className="hislog__page-btn"
            disabled={page >= totalPages}
            onClick={() => setPage(p => p + 1)}
          >
            Вперёд {'>'}
          </button>
        </div>
      </div>
    </div>
  )
}