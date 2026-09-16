import React, { useEffect, useRef, useState } from 'react'
import { MapView, type MapHandle } from './MapView.tsx'
import { submitProposal, TYPE_LABEL, TYPES, zipHasRootIndex, type EventType } from './api.ts'
import './propose.css'

// Раскладка по форме создания метки из trudovaya-slava, значения из валидации POST /api/v1/submissions.
const MAX_MB = 50
const MIN_YEAR = 862

interface Draft {
  title: string; date: string; type: EventType | ''; description: string
  lat: number | null; lng: number | null
  email: string; telegram: string; file: File | null
  zip: 'none' | 'checking' | 'ok' | string // строка - текст ошибки архива
}
const EMPTY: Draft = { title: '', date: '', type: '', description: '', lat: null, lng: null, email: '', telegram: '', file: null, zip: 'none' }

type Field = 'type' | 'title' | 'date' | 'description' | 'place' | 'archive' | 'email' | 'telegram'
type Errors = Partial<Record<Field, string>>
const FIELDS: Field[] = ['type', 'title', 'date', 'description', 'place', 'archive', 'email', 'telegram']
const LABEL: Record<Field, string> = {
  type: 'Тип события', title: 'Название', date: 'Дата', description: 'Описание',
  place: 'Место на карте', archive: 'ZIP-архив сайта', email: 'Email', telegram: 'Telegram',
}
const SECTION: Record<Field, string> = {
  type: 'type', title: 'about', date: 'about', description: 'about', place: 'place', archive: 'archive', email: 'contacts', telegram: 'contacts',
}
const SERVER_FIELD: Record<string, Field> = {
  title: 'title', eventDate: 'date', eventType: 'type', description: 'description', latitude: 'place', longitude: 'place',
  archive: 'archive', email: 'email', telegramUsername: 'telegram',
}

type Phase =
  | { k: 'form' }
  | { k: 'sending'; loaded: number; total: number }
  | { k: 'done'; id: number }
  | { k: 'fail'; status: number; message?: string; retryAfter?: number }

interface Place { place_id: number; display_name: string; lat: string; lon: string }

// Мелкие файлы не показываем как "0 МБ".
const mb = (bytes: number) => (bytes > 0 && bytes < 10486 ? 0.01 : bytes / 1048576)
  .toLocaleString('ru-RU', { maximumFractionDigits: bytes < 1048576 ? 2 : 1 })
const pad = (n: number) => String(n).padStart(2, '0')
const todayIso = () => { const d = new Date(); return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` }
const coord = (v: number, pos: string, neg: string) => `${Math.abs(v).toFixed(5)} ${v < 0 ? neg : pos}`

// ДД.ММ.ГГГГ -> YYYY-MM-DD, null если такой даты нет.
function toIso(date: string): string | null {
  const m = /^(\d{2})\.(\d{2})\.(\d{3,4})$/.exec(date)
  if (!m) return null
  const [, dd, mm, yy] = m
  const d = new Date(0)
  d.setFullYear(+yy, +mm - 1, +dd)
  if (d.getFullYear() !== +yy || d.getMonth() !== +mm - 1 || d.getDate() !== +dd) return null
  return `${yy.padStart(4, '0')}-${mm}-${dd}`
}
const fromIso = (iso: string) => iso.split('-').map((p, i) => (i === 0 ? String(+p) : p)).reverse().join('.')
const maskDate = (raw: string) => {
  const digits = raw.replace(/\D/g, '').slice(0, 8)
  return [digits.slice(0, 2), digits.slice(2, 4), digits.slice(4)].filter(Boolean).join('.')
}

function validate(d: Draft): Errors {
  const e: Errors = {}
  if (!d.type) e.type = 'Выберите тип события'
  const title = d.title.trim().length
  if (title < 3 || title > 100) e.title = 'Укажите название - от 3 до 100 символов'
  const iso = toIso(d.date)
  if (!iso || +iso.slice(0, 4) < MIN_YEAR) e.date = 'Неверная дата. Формат ДД.ММ.ГГГГ, от 862 года'
  else if (iso > todayIso()) e.date = 'Дата не может быть в будущем'
  const desc = d.description.trim().length
  if (desc === 0 || desc > 450) e.description = 'Опишите событие - до 450 символов'
  if (d.lat === null || d.lng === null) e.place = 'Поставьте точку на карте'
  if (!d.file) e.archive = 'Прикрепите ZIP-архив сайта'
  else if (d.zip === 'checking') e.archive = 'Архив ещё проверяется'
  else if (d.zip !== 'ok') e.archive = d.zip
  const email = d.email.trim()
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/.test(email)) e.email = 'Проверьте адрес: нужен домен вида example.ru'
  else if (email.length > 70) e.email = 'Не больше 70 символов'
  if (!/^[A-Za-z0-9_]{5,32}$/.test(d.telegram.trim())) e.telegram = 'Без "@", только латиница, цифры и _, от 5 до 32 символов'
  return e
}

const none = () => false
const noop = () => undefined

export const ProposeModal: React.FC<{ open: boolean; mobile: boolean; onClose: () => void }> = ({ open, mobile, onClose }) => {
  const [d, setD] = useState<Draft>(EMPTY)
  const [phase, setPhase] = useState<Phase>({ k: 'form' })
  const [shown, setShown] = useState<Set<Field>>(new Set())
  const [server, setServer] = useState<Errors>({})
  const [toast, setToast] = useState<Field[]>([])
  const [drag, setDrag] = useState(false)
  const [copied, setCopied] = useState(false)
  const [query, setQuery] = useState('')
  const [places, setPlaces] = useState<Place[]>([])
  const fileRef = useRef<HTMLInputElement>(null)
  const pickerRef = useRef<HTMLInputElement>(null)
  const mapRef = useRef<MapHandle>(null)
  const abortRef = useRef<() => void>()
  const sending = phase.k === 'sending'

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape' && !sending) onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, sending, onClose])

  useEffect(() => {
    if (!toast.length) return
    const t = setTimeout(() => setToast([]), 6000)
    return () => clearTimeout(t)
  }, [toast])

  // Поиск места через Nominatim: не чаще раза в 500 мс, как требуют правила сервиса.
  useEffect(() => {
    const q = query.trim()
    if (q.length < 3) { setPlaces([]); return }
    const ctrl = new AbortController()
    const t = setTimeout(() => {
      fetch(`https://nominatim.openstreetmap.org/search?format=json&limit=5&accept-language=ru&q=${encodeURIComponent(q)}`, { signal: ctrl.signal })
        .then(r => r.json() as Promise<Place[]>).then(setPlaces).catch(() => undefined)
    }, 500)
    return () => { clearTimeout(t); ctrl.abort() }
  }, [query])

  if (!open) return null

  const local = validate(d)
  const errors: Errors = { ...local, ...server }
  const visible = (f: Field) => (shown.has(f) ? errors[f] : undefined)
  const show = (...f: Field[]) => setShown(s => new Set([...s, ...f]))
  const set = <K extends keyof Draft>(k: K, v: Draft[K], f?: Field) => {
    setD(prev => ({ ...prev, [k]: v }))
    if (f) setServer(({ [f]: _, ...rest }) => rest)
  }
  const pick = (lat: number, lng: number) => {
    setD(prev => ({ ...prev, lat, lng }))
    setServer(({ place: _, ...rest }) => rest)
    show('place')
  }

  const reset = () => { setD(EMPTY); setPhase({ k: 'form' }); setShown(new Set()); setServer({}); setToast([]); setQuery('') }
  const close = () => { if (phase.k === 'done') reset(); else setPhase({ k: 'form' }); onClose() }

  const pickFile = async (file: File | undefined) => {
    if (!file) return
    setServer(({ archive: _, ...rest }) => rest)
    show('archive')
    if (!/\.zip$/i.test(file.name)) return setD(p => ({ ...p, file, zip: 'Нужен ZIP с index.html в корне - не RAR и не папка' }))
    if (file.size > MAX_MB * 1048576) return setD(p => ({ ...p, file, zip: `${mb(file.size)} МБ - архив больше ${MAX_MB} МБ. Уберите видео и соберите заново` }))
    setD(p => ({ ...p, file, zip: 'checking' }))
    const zip = await zipHasRootIndex(file).then(
      ok => (ok ? 'ok' : 'В корне архива нет index.html. Заархивируйте содержимое папки сайта, а не саму папку'),
      () => 'Не удалось прочитать архив. Соберите ZIP заново',
    )
    setD(p => (p.file === file ? { ...p, zip } : p))
  }
  const clearFile = () => { set('file', null, 'archive'); set('zip', 'none'); if (fileRef.current) fileRef.current.value = '' }

  const complain = (bad: Field[]) => {
    show(...bad)
    setToast(bad)
    document.getElementById(`pp-s-${SECTION[bad[0]]}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  const send = async () => {
    const bad = FIELDS.filter(f => errors[f])
    if (bad.length) return complain(bad)
    setToast([])
    const body = new FormData()
    body.set('title', d.title.trim())
    body.set('description', d.description.trim())
    body.set('eventDate', toIso(d.date)!)
    body.set('eventType', d.type)
    body.set('latitude', String(d.lat))
    body.set('longitude', String(d.lng))
    body.set('email', d.email.trim())
    body.set('telegramUsername', d.telegram.trim())
    body.set('archive', d.file!)
    setPhase({ k: 'sending', loaded: 0, total: d.file!.size })
    const req = submitProposal(body, (loaded, total) => setPhase({ k: 'sending', loaded, total }))
    abortRef.current = req.abort
    const res = await req.promise
    if (res.ok) return setPhase({ k: 'done', id: res.id })
    if (res.status === -1) return setPhase({ k: 'form' })
    if (res.status === 400 && res.fields) {
      const mapped: Errors = {}
      for (const [k, v] of Object.entries(res.fields)) mapped[SERVER_FIELD[k] ?? 'title'] = v
      setServer(mapped)
      setPhase({ k: 'form' })
      return complain(FIELDS.filter(f => mapped[f]))
    }
    setPhase({ k: 'fail', status: res.status, message: res.message, retryAfter: res.retryAfter })
  }

  const openCalendar = () => {
    const el = pickerRef.current
    if (!el) return
    el.value = toIso(d.date) ?? ''
    try { el.showPicker() } catch { el.focus() }
  }

  const choosePlace = (p: Place) => {
    const lat = +p.lat, lng = +p.lon
    pick(lat, lng)
    mapRef.current?.flyTo(lat, lng)
    setQuery(p.display_name.split(',')[0])
    setPlaces([])
  }

  const section = (id: string, n: number, title: string, fields: Field[], children: React.ReactNode, hint?: string) => {
    const bad = fields.some(f => visible(f))
    return (
      <section id={`pp-s-${id}`} className="pp-section">
        <div className="pp-section-head">
          <span className={`pp-section-num${bad ? ' bad' : ''}`}>{pad(n)}</span>
          <h2 className="pp-section-title">{title}</h2>
          <span className="pp-badge">{hint ?? 'обязательно'}</span>
        </div>
        {children}
      </section>
    )
  }

  let body: React.ReactNode
  let foot: React.ReactNode
  if (phase.k === 'done') {
    body = (
      <div className="pp-result">
        <div className="pp-result-icon ok">✓</div>
        <h2 className="pp-result-title">Заявка отправлена</h2>
        <div className="pm-row-tight pp-number">
          {!mobile && <span className="pm-dim-text">Номер заявки</span>}
          <span className="pp-code">#{phase.id}</span>
          {!mobile && (
            <button type="button" className="pp-copy" onClick={() => navigator.clipboard?.writeText(String(phase.id)).then(() => setCopied(true))}>
              {copied ? 'Скопировано' : 'Скопировать'}
            </button>
          )}
        </div>
        <p className="pp-result-text">
          Модератор проверит событие, место на карте и архив. Статус придёт на {d.email.trim()}: опубликовано со ссылкой или отклонено с причиной.
        </p>
      </div>
    )
    foot = (
      <>
        <div className="pm-grow" />
        <button type="button" className="pm-btn-ghost" onClick={() => { reset(); setCopied(false) }}>Предложить ещё</button>
        <button type="button" className="pm-btn" onClick={close}>Вернуться к карте</button>
      </>
    )
  } else if (phase.k === 'fail') {
    const f = phase.status === 413
      ? { tone: 'bad', glyph: '!', title: `Архив больше ${MAX_MB} МБ`, code: '413',
          text: `Сервер не принял файл: ${mb(d.file?.size ?? 0)} МБ при лимите ${MAX_MB}. Уберите видео и крупные изображения, соберите ZIP заново - остальные поля заявки сохранены.`,
          primary: ['Выбрать другой архив', () => { clearFile(); setPhase({ k: 'form' }); fileRef.current?.click() }],
          secondary: ['Назад к заявке', () => setPhase({ k: 'form' })] }
      : phase.status === 429
        ? { tone: 'warn', glyph: '⏲', title: 'Слишком много заявок', code: '429',
            text: `С этого адреса уже отправлено 5 заявок за час. Попробуйте через ${Math.ceil((phase.retryAfter ?? 3600) / 60)} мин - черновик сохранён.`,
            primary: ['Повторить позже', close], secondary: ['Назад к заявке', () => setPhase({ k: 'form' })] }
        : { tone: 'bad', glyph: '×', title: phase.status === 0 ? 'Нет связи с сервером' : 'Сервер не принял заявку',
            code: phase.status === 0 ? 'network' : String(phase.status),
            text: phase.status === 0
              ? 'Заявка не ушла. Проверьте соединение и отправьте снова - поля и архив на месте.'
              : `${phase.message ?? 'Ошибка на сервере'}. Попробуйте отправить ещё раз - поля и архив на месте.`,
            primary: ['Повторить отправку', send], secondary: ['Назад к заявке', () => setPhase({ k: 'form' })] }
    const [pLabel, pRun] = f.primary as [string, () => void]
    const [sLabel, sRun] = f.secondary as [string, () => void]
    body = (
      <div className="pp-fail" role="alert">
        <div className={`pp-result-icon ${f.tone}`}>{f.glyph}</div>
        <div className="pm-grow">
          <div className="pm-row-tight"><h2 className={`pp-fail-title ${f.tone}`}>{f.title}</h2><span className="pp-tag">{f.code}</span></div>
          <p className="pp-fail-text">{f.text}</p>
        </div>
      </div>
    )
    foot = (
      <>
        <div className="pm-grow" />
        <button type="button" className="pm-btn-text" onClick={sRun}>{sLabel}</button>
        <button type="button" className="pm-btn" onClick={pRun}>{pLabel}</button>
      </>
    )
  } else {
    body = (
      <fieldset className="pp-fields" disabled={sending}>
        {section('type', 1, 'Тип события', ['type'], (
          <>
          <div className="pp-types" role="radiogroup" aria-label="Тип события">
            {TYPES.map(t => (
              <button key={t} type="button" role="radio" aria-checked={d.type === t}
                className={`pp-type ${t}${d.type === t ? ' on' : ''}${visible('type') ? ' bad' : ''}`}
                onClick={() => { set('type', t, 'type'); show('type') }}>
                <span className="pp-dot" />{TYPE_LABEL[t]}
              </button>
            ))}
          </div>
          {visible('type') && <span className="pp-error">{visible('type')}</span>}
          </>
        ))}

        {section('about', 2, 'О событии', ['title', 'date', 'description'], (
          <div className="pp-stack">
            <Text label="Название" error={visible('title')} count={`${d.title.length} / 100`}>
              <input className="pp-input" value={d.title} maxLength={100} placeholder="Например, Куликовская битва"
                onChange={e => set('title', e.target.value, 'title')} onBlur={() => d.title && show('title')} />
            </Text>
            <Text label="Дата события" error={visible('date')} hint="ДД.ММ.ГГГГ, от 862 года">
              <div className={`pp-input pp-date${visible('date') ? ' bad' : ''}`}>
                <input value={d.date} inputMode="numeric" placeholder="ДД.ММ.ГГГГ" aria-label="Дата события"
                  onChange={e => set('date', maskDate(e.target.value), 'date')} onBlur={() => d.date && show('date')} />
                <button type="button" className="pm-link" onClick={openCalendar}>Календарь</button>
                <input ref={pickerRef} type="date" className="pp-picker" tabIndex={-1} aria-hidden min="0862-01-01" max={todayIso()}
                  onChange={e => { if (e.target.value) { set('date', fromIso(e.target.value), 'date'); show('date') } }} />
              </div>
            </Text>
            <Text label="Описание" error={visible('description')} count={`${d.description.length} / 450`}>
              <textarea className="pp-input pp-area" value={d.description} maxLength={450} placeholder="Что произошло и почему это важно"
                onChange={e => set('description', e.target.value, 'description')} onBlur={() => d.description && show('description')} />
            </Text>
          </div>
        ))}

        {section('place', 3, 'Место на карте', ['place'], (
          <>
            <p className="pp-lead">Найдите место через поиск или нажмите на карту. Метку можно перетащить.</p>
            <div className="pp-search">
              <input className="pp-input" value={query} placeholder="Город, село, урочище" aria-label="Поиск места"
                onChange={e => setQuery(e.target.value)} />
              {places.length > 0 && (
                <ul className="pp-places" role="listbox">
                  {places.map(p => (
                    <li key={p.place_id}>
                      <button type="button" role="option" aria-selected={false} onClick={() => choosePlace(p)}>{p.display_name}</button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
            <div className={`pp-map${visible('place') ? ' bad' : ''}`}>
              <MapView ref={mapRef} events={[]} isActive={none} selectedId={null} onSelect={noop} pad={12} labels
                pick={d.lat !== null && d.lng !== null ? { lat: d.lat, lng: d.lng } : null} onPick={sending ? undefined : pick} />
            </div>
            {d.lat !== null && d.lng !== null
              ? <div className="pp-coord"><b>{coord(d.lat, 'с.ш.', 'ю.ш.')}</b> / <b>{coord(d.lng, 'в.д.', 'з.д.')}</b></div>
              : visible('place') && <span className="pp-error">{visible('place')}</span>}
          </>
        ))}

        {section('archive', 4, 'Сайт события', ['archive'], (
          <Text error={d.file ? undefined : visible('archive')}>
            <input ref={fileRef} type="file" accept=".zip,application/zip" hidden onChange={e => pickFile(e.target.files?.[0])} />
            {d.file ? (
              <div className={`pp-file${d.zip !== 'ok' && d.zip !== 'checking' ? ' bad' : ''}`}>
                <div className="pp-zip">ZIP</div>
                <div className="pm-grow">
                  <div className="pp-file-name">{d.file.name}</div>
                  <div className={`pp-file-note ${d.zip === 'ok' ? 'ok' : d.zip === 'checking' ? '' : 'bad'}`}>
                    {d.zip === 'ok' ? `${mb(d.file.size)} МБ · index.html найден` : d.zip === 'checking' ? 'Проверяем архив...' : errors.archive}
                  </div>
                </div>
                <button type="button" className="pp-x" aria-label="Убрать архив" onClick={clearFile}>×</button>
              </div>
            ) : (
              <button type="button" className={`pp-drop${drag ? ' over' : ''}${visible('archive') ? ' bad' : ''}`}
                onClick={() => fileRef.current?.click()}
                onDragOver={e => { e.preventDefault(); setDrag(true) }}
                onDragLeave={() => setDrag(false)}
                onDrop={e => { e.preventDefault(); setDrag(false); pickFile(e.dataTransfer.files[0]) }}>
                <span className="pp-drop-title">{drag ? 'Отпустите файл' : mobile ? 'Выбрать ZIP-архив сайта' : 'Перетащите архив или выберите файл'}</span>
                <span className="pm-dim-text">ZIP до {MAX_MB} МБ · index.html в корне архива</span>
              </button>
            )}
          </Text>
        ), 'ZIP до 50 МБ')}

        {section('contacts', 5, 'Контакты', ['email', 'telegram'], (
          <div className="pp-pair">
            <Text label="Email" error={visible('email')} hint="Сюда придёт статус заявки">
              <input className="pp-input" type="email" autoComplete="email" value={d.email} maxLength={70} placeholder="ivan@example.ru"
                onChange={e => set('email', e.target.value, 'email')} onBlur={() => d.email && show('email')} />
            </Text>
            <Text label="Telegram" error={visible('telegram')} hint='Без "@", для вопросов модератора'>
              <input className="pp-input" value={d.telegram} maxLength={32} placeholder="ivan_petrov" autoCapitalize="none" spellCheck={false}
                onChange={e => set('telegram', e.target.value, 'telegram')} onBlur={() => d.telegram && show('telegram')} />
            </Text>
          </div>
        ), 'не публикуются')}
      </fieldset>
    )
    foot = phase.k === 'sending'
      ? (
        <>
          <div className="pp-progress pm-grow" role="status">
            <div className="pp-progress-row">
              <span className="pm-mono">Загрузка архива</span>
              <span className="pp-muted">{mb(phase.loaded)} из {mb(phase.total)} МБ · {Math.round((phase.loaded / (phase.total || 1)) * 100)} %</span>
            </div>
            <div className="pp-bar"><div style={{ width: `${(phase.loaded / (phase.total || 1)) * 100}%` }} /></div>
          </div>
          <button type="button" className="pm-btn-ghost" onClick={() => abortRef.current?.()}>Отменить</button>
        </>
      )
      : (
        <>
          {!mobile && <span className="pm-dim-text pm-grow">Модератор проверит заявку перед публикацией</span>}
          <button type="button" className="pm-btn-text" onClick={close}>Отмена</button>
          <button type="button" className="pm-btn" onClick={send}>{mobile ? 'Отправить' : 'Отправить на модерацию'}</button>
        </>
      )
  }

  return (
    <div className={`pm-root pp-page${mobile ? ' mobile' : ''}`} role="dialog" aria-modal="true" aria-labelledby="pp-title">
      <div className="pp-top">
        <button type="button" className="pp-back" disabled={sending} onClick={close}>{mobile ? '<' : '<- Карта'}</button>
        <span className="pp-divider" />
        <h1 id="pp-title" className="pp-top-title">Предложить событие</h1>
      </div>
      <div className="pp-scroll">
        <div className={`pp-inner${phase.k === 'form' || sending ? '' : ' result'}`}>
          {phase.k === 'form' || sending
            ? <p className="pp-intro">Расскажите о событии, отметьте место на карте и приложите сайт. После проверки метка появится на карте.</p>
            : null}
          {body}
        </div>
      </div>
      <div className="pp-actbar">{foot}</div>
      {toast.length > 0 && (
        <div className="pp-toast" role="alert">
          <div className="pp-toast-head">
            <span className="pp-banner-icon">!</span>
            <span className="pm-grow">Не всё заполнено</span>
            <button type="button" className="pp-x small" aria-label="Скрыть" onClick={() => setToast([])}>×</button>
          </div>
          <ul className="pp-toast-list">{toast.map(f => <li key={f}>{LABEL[f]}</li>)}</ul>
        </div>
      )}
    </div>
  )
}

const Text: React.FC<{
  label?: string; error?: string; hint?: string; count?: string; labelBad?: boolean; group?: boolean; children: React.ReactNode
}> = ({ label, error, hint, count, labelBad, group, children }) => {
  const Tag = group ? 'div' : 'label'
  return (
  <Tag className={`pp-field${error ? ' bad' : ''}`}>
    {label && <span className={`pp-label${labelBad ? ' bad' : ''}`}>{label}</span>}
    {children}
    {error ? <span className="pp-error">{error}</span> : hint ? <span className="pm-dim-text">{hint}</span> : null}
    {count && !error && <span className="pp-count">{count}</span>}
  </Tag>
  )
}
