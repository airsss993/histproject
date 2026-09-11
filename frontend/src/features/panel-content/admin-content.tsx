import React, { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { RequestModal, Request } from './admreq'
import { ReqCard } from '../admin-connection/admin-function/reqard'
import { fetchRequests } from '../admin-connection/admin-function/reques'
import { fetchRequestCard } from '../admin-connection/admin-function/cardreques'
import { ReqFilter } from '../admin-connection/admin-function/reqfilter'

const PAGE_SIZE = 5

export const AdminContentComponent: React.FC = () => {
  const [selectedRequest, setSelectedRequest] = useState<Request | null>(null)
  const [requests, setRequests] = useState<Request[]>([])
  const navigate = useNavigate()
  const [currentPage, setCurrentPage] = useState(1)
  const [activeStatus, setActiveStatus] = useState('')
  const [searchQ, setSearchQ] = useState('')
  const [debouncedQ, setDebouncedQ] = useState('')
  const [statusCounts, setStatusCounts] = useState<Record<string, number>>({})
  const [allTotal, setAllTotal] = useState(0)

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQ(searchQ), 400)
    return () => clearTimeout(timer)
  }, [searchQ])

  useEffect(() => {
    setCurrentPage(1)
    const loadRequests = async () => {
      try {
        const result = await fetchRequests({
          status: activeStatus || undefined,
          q: debouncedQ || undefined,
          limit: 100,
        })
        if (result && result.requests) {
          setRequests(result.requests)
          if (!activeStatus && !debouncedQ) {
            const counts: Record<string, number> = {}
            result.requests.forEach((req: Request) => { 
              counts[req.status] = (counts[req.status] || 0) + 1 
            })
            setStatusCounts(counts)
            setAllTotal(result.total || result.requests.length)
          }
        }
      } catch (error) {
        console.error('Ошибка загрузки заявок:', error)
        navigate('/log')
      }
    }
    
    loadRequests()
  }, [activeStatus, debouncedQ])

  const totalPages = Math.max(1, Math.ceil(requests.length / PAGE_SIZE))
  const pagedRequests = requests.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE)
  const displayTotal = allTotal

  const handleRowClick = async (request: Request) => {
    const detailed = await fetchRequestCard(request.id) // @ts-ignore
    setSelectedRequest(detailed ?? request)
  }

  return (
    <div className="AdminRequests">
      {selectedRequest && (
        <RequestModal request={selectedRequest} onClose={() => setSelectedRequest(null)} />
      )}
      <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 26 26" fill="none">
        <path d="M4.3335 4.33335C4.3335 3.75872 4.56177 3.20762 4.9681 2.80129C5.37443 2.39496 5.92553 2.16669 6.50016 2.16669H15.1668C15.4541 2.16675 15.7296 2.28093 15.9327 2.4841L21.3494 7.90077C21.5526 8.10389 21.6668 8.37939 21.6668 8.66669V21.6667C21.6668 22.2413 21.4386 22.7924 21.0322 23.1987C20.6259 23.6051 20.0748 23.8334 19.5002 23.8334H6.50016C5.92553 23.8334 5.37443 23.6051 4.9681 23.1987C4.56177 22.7924 4.3335 22.2413 4.3335 21.6667V4.33335ZM19.0517 8.66669L15.1668 4.78185V8.66669H19.0517ZM13.0002 4.33335H6.50016V21.6667H19.5002V10.8334H14.0835C13.7962 10.8334 13.5206 10.7192 13.3175 10.5161C13.1143 10.3129 13.0002 10.0373 13.0002 9.75002V4.33335ZM8.66683 14.0834C8.66683 13.796 8.78097 13.5205 8.98413 13.3173C9.1873 13.1142 9.46285 13 9.75016 13H16.2502C16.5375 13 16.813 13.1142 17.0162 13.3173C17.2194 13.5205 17.3335 13.796 17.3335 14.0834C17.3335 14.3707 17.2194 14.6462 17.0162 14.8494C16.813 15.0525 16.5375 15.1667 16.2502 15.1667H9.75016C9.46285 15.1667 9.1873 15.0525 8.98413 14.8494C8.78097 14.6462 8.66683 14.3707 8.66683 14.0834ZM8.66683 18.4167C8.66683 18.1294 8.78097 17.8538 8.98413 17.6507C9.1873 17.4475 9.46285 17.3334 9.75016 17.3334H16.2502C16.5375 17.3334 16.813 17.4475 17.0162 17.6507C17.2194 17.8538 17.3335 18.1294 17.3335 18.4167C17.3335 18.704 17.2194 18.9796 17.0162 19.1827C16.813 19.3859 16.5375 19.5 16.2502 19.5H9.75016C9.46285 19.5 9.1873 19.3859 8.98413 19.1827C8.78097 18.9796 8.66683 18.704 8.66683 18.4167Z" fill="#555555"/>
      </svg>
      <h1>
        Заявки
      </h1>
      <p>
        Управление заявками на добавление событий
      </p>
      <div className="admin-buttons">
        <ReqFilter
          total={displayTotal}
          activeStatus={activeStatus}
          statusCounts={statusCounts}
          onStatusChange={setActiveStatus}
          onSearchChange={setSearchQ}
        />
        <div className="found-count">
          Найдено заявок: <span className="countRequests">{displayTotal}</span>
        </div>
      </div>

      <div className="requests-table">
        <div className="requests-table__header">
          <span>ID</span>
          <span>Название события</span>
          <span>Дата подачи</span>
          <span>Статус</span>
        </div>
        {pagedRequests.map((req) => (
          <ReqCard
            key={req.id} // @ts-ignore
            request={req} // @ts-ignore
            onClick={handleRowClick}
          />
        ))}
      </div>

      <div className="requests-pagination">
        <span className="requests-pagination__info">
          Показано {pagedRequests.length} из {requests.length} заявок
        </span>
        <div className="requests-pagination__controls">
          <button
            className="requests-pagination__nav"
            disabled={currentPage === 1}
            onClick={() => setCurrentPage(p => p - 1)}
          >
            <svg width="7" height="12" viewBox="0 0 7 12" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M6 1L1 6L6 11" stroke={currentPage === 1 ? '#aaa' : '#555'} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
            &nbsp;Назад
          </button>

          {(() => {
            const pages: (number | '...')[] = []
            if (totalPages <= 7) {
              for (let i = 1; i <= totalPages; i++) pages.push(i)
            } else {
              pages.push(1)
              if (currentPage > 3) pages.push('...')
              const start = Math.max(2, currentPage - 1)
              const end = Math.min(totalPages - 1, currentPage + 1)
              for (let i = start; i <= end; i++) pages.push(i)
              if (currentPage < totalPages - 2) pages.push('...')
              pages.push(totalPages)
            }
            return pages.map((page, idx) =>
              page === '...'
                ? <span key={`ellipsis-${idx}`} className="requests-pagination__ellipsis">…</span>
                : <button
                    key={page}
                    className={`requests-pagination__page${currentPage === page ? ' active' : ''}`}
                    onClick={() => setCurrentPage(page)}
                  >
                    {page}
                  </button>
            )
          })()}

          <button
            className="requests-pagination__nav"
            disabled={currentPage === totalPages}
            onClick={() => setCurrentPage(p => p + 1)}
          >
            Вперёд&nbsp;
            <svg width="7" height="12" viewBox="0 0 7 12" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M1 1L6 6L1 11" stroke={currentPage === totalPages ? '#aaa' : '#555'} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
};