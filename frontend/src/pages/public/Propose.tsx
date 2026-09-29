import React, { useEffect, useRef, useState } from 'react'
import { GLYPH, MapView, TYPE_COLOR, type MapHandle } from './MapView.tsx'
import { Presence, useLeaving } from './presence.tsx'
import { geocode, submitProposal, TYPE_LABEL, TYPES, zipHasRootIndex, type EventType, type Place } from './api.ts'
import { Check } from '../../shared/ui/Field.tsx'
import { useNavigate } from 'react-router-dom'
import './propose.css'

// Раскладка по форме создания метки из trudovaya-slava, значения из валидации POST /api/v1/submissions.
const MAX_MB = 20
const MIN_YEAR = 862

interface Draft {
  title: string; date: string; type: EventType | ''; description: string
  lat: number | null; lng: number | null
  telegram: string; file: File | null
  zip: 'none' | 'checking' | 'ok' | string // строка - текст ошибки архива
  consent: boolean; rules: boolean // согласие на обработку и заверение о материалах
}
const EMPTY: Draft = { title: '', date: '', type: '', description: '', lat: null, lng: null, telegram: '', file: null, zip: 'none', consent: false, rules: false }

type Field = 'type' | 'title' | 'date' | 'description' | 'place' | 'archive' | 'telegram' | 'consent' | 'rules'
type Errors = Partial<Record<Field, string>>
const FIELDS: Field[] = ['type', 'title', 'date', 'description', 'place', 'archive', 'telegram', 'consent', 'rules']

// Шаги мастера. icon - путь 24x24 для плитки в шапке карточки.
const STEPS: { fields: Field[]; title: string; sub: string; icon: string }[] = [
  { fields: ['type'], title: 'Какое это событие?', sub: 'От типа зависят цвет и значок метки на карте',
    icon: 'M3.5 12.5V4.5a1 1 0 0 1 1-1h8l8.5 8.5-9 9Z M8 8.5v.01' },
  { fields: ['title', 'date', 'description'], title: 'О событии', sub: 'Название, дата и пара предложений о том, что произошло',
    icon: 'M5 6h14 M5 10h14 M5 14h14 M5 18h9' },
  { fields: ['place'], title: 'Где это было', sub: 'Найдите место поиском или нажмите на карту. Метку можно перетащить',
    icon: 'M12 21s-7-6.2-7-11.5a7 7 0 0 1 14 0C19 14.8 12 21 12 21Z M12 12.2a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Z' },
  { fields: ['archive'], title: 'Сайт события', sub: 'ZIP-архив, index.html лежит в его корне',
    icon: 'M4 7.5 12 3.5l8 4v9l-8 4-8-4Z M4 7.5l8 4 8-4 M12 11.5v9' },
  { fields: ['telegram', 'consent', 'rules'], title: 'Контакты', sub: 'Не публикуется. Статус заявки придёт на почту аккаунта',
    icon: 'M3.5 5.5h17v13h-17Z M3.5 6.5l8.5 7 8.5-7' },
]
const stepOf = (f: Field) => STEPS.findIndex(s => s.fields.includes(f))
const TYPE_SUB: Record<EventType, string> = { military: 'Бородино, оборона Севастополя', political: 'Ништадтский мир, Манифест 1861 года', personality: 'Ломоносов, Гагарин' }
const RESULT_ICON = { ok: 'M5 12.5l4.5 4.5L19 7.5', bad: 'M12 6.5v7 M12 17.5v.01', warn: 'M12 7v5l3 2 M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z' }
const ARROW = 'M5 12h14 M13 6l6 6-6 6'
const SEND = 'M4 12 20 4l-4 16-4-7Z M12 13l8-9'
const PLUS = 'M12 5v14 M5 12h14'
const CROSS = 'M6 6l12 12 M18 6 6 18'
const SERVER_FIELD: Record<string, Field> = {
  title: 'title', eventDate: 'date', eventType: 'type', description: 'description', latitude: 'place', longitude: 'place',
  archive: 'archive', telegramUsername: 'telegram', consent: 'consent', rules: 'rules',
}

type Phase =
  | { k: 'form' }
  | { k: 'sending'; loaded: number; total: number }
  | { k: 'done'; publicId: string }
  | { k: 'fail'; status: number; message?: string; retryAfter?: number }


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
  if (!/^[A-Za-z0-9_]{5,32}$/.test(d.telegram.trim())) e.telegram = 'Без "@", только латиница, цифры и _, от 5 до 32 символов'
  if (!d.consent) e.consent = 'Без согласия заявку не отправить'
  if (!d.rules) e.rules = 'Подтвердите, что соблюдаете правила'
  return e
}

const none = () => false
const noop = () => undefined

// email - почта аккаунта: форму открывают только вошедшим, статус заявки уходит туда.
export const ProposeModal: React.FC<{ open: boolean; email: string; mobile: boolean; onClose: () => void }> = ({ open, email, mobile, onClose }) => {
  const navigate = useNavigate()
  const [d, setD] = useState<Draft>(EMPTY)
  const [ready, setReady] = useState(false) // автор подтвердил, что архив сайта уже есть
  const [phase, setPhase] = useState<Phase>({ k: 'form' })
  const [shown, setShown] = useState<Set<Field>>(new Set())
  const [server, setServer] = useState<Errors>({})
  const [step, setStep] = useState(0)
  const [reached, setReached] = useState(0) // дальний открытый шаг: до него можно прыгать по полоскам
  const [drag, setDrag] = useState(false)
  const [copied, setCopied] = useState(false)
  const [query, setQuery] = useState('')
  const [places, setPlaces] = useState<Place[]>([])
  const fileRef = useRef<HTMLInputElement>(null)
  const pickerRef = useRef<HTMLInputElement>(null)
  const mapRef = useRef<MapHandle>(null)
  const abortRef = useRef<(() => void) | undefined>(undefined)
  const closeRef = useRef<() => void>(noop) // close объявлен ниже раннего return, а хук должен быть до него
  const chosenRef = useRef('') // выбранное место: его текст в поле не ищем повторно
  const sending = phase.k === 'sending'
  const leaving = useLeaving(open)
  // Отправленную заявку сбрасываем после анимации ухода, иначе окно гаснет уже с пустой формой.
  const resetOnHide = useRef(false)
  useEffect(() => {
    if (open || leaving || !resetOnHide.current) return
    resetOnHide.current = false
    setD(EMPTY); setPhase({ k: 'form' }); setShown(new Set()); setServer({}); setStep(0); setReached(0); setQuery('')
  }, [open, leaving])

  useEffect(() => {
    if (!open) return
    // Через close, а не onClose: по Escape окно должно сбрасываться так же, как по кнопке.
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape' && !sending) closeRef.current() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, sending])

  // Поиск места через наш сервер (он ходит в Nominatim): запрос не чаще раза в 500 мс.
  useEffect(() => {
    const q = query.trim()
    if (q.length < 3 || q === chosenRef.current) { setPlaces([]); return }
    const ctrl = new AbortController()
    const t = setTimeout(() => {
      geocode(q, ctrl.signal).then(setPlaces).catch(() => undefined)
    }, 500)
    return () => { clearTimeout(t); ctrl.abort() }
  }, [query])

  if (!open && !leaving) return null

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

  const reset = () => { setReady(false); setD(EMPTY); setPhase({ k: 'form' }); setShown(new Set()); setServer({}); setStep(0); setReached(0); setQuery('') }
  // Защёлку выставляем при каждом закрытии: иначе, если окно успели открыть заново до конца
  // анимации ухода, она осталась бы взведённой и стёрла следующий черновик.
  const close = () => {
    resetOnHide.current = phase.k === 'done'
    if (phase.k !== 'done') setPhase({ k: 'form' })
    onClose()
  }
  closeRef.current = close

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

  const go = (n: number) => { setStep(n); setReached(r => Math.max(r, n)) }
  // Показать ошибки и вернуть на первый шаг, где они есть.
  const complain = (bad: Field[]) => {
    show(...bad)
    setStep(Math.min(...bad.map(stepOf)))
  }
  const next = () => {
    const bad = STEPS[step].fields.filter(f => errors[f])
    if (bad.length) return show(...bad)
    if (step < STEPS.length - 1) go(step + 1)
    else send()
  }

  const send = async () => {
    const bad = FIELDS.filter(f => errors[f])
    if (bad.length) return complain(bad)
    const body = new FormData()
    body.set('title', d.title.trim())
    body.set('description', d.description.trim())
    body.set('eventDate', toIso(d.date)!)
    body.set('eventType', d.type)
    body.set('latitude', String(d.lat))
    body.set('longitude', String(d.lng))
    body.set('telegramUsername', d.telegram.trim())
    body.set('archive', d.file!)
    body.set('consent', String(d.consent))
    body.set('rules', String(d.rules))
    setPhase({ k: 'sending', loaded: 0, total: d.file!.size })
    const req = submitProposal(body, (loaded, total) => setPhase({ k: 'sending', loaded, total }))
    abortRef.current = req.abort
    const res = await req.promise
    if (res.ok) return setPhase({ k: 'done', publicId: res.publicId })
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
    const name = p.display_name.split(',')[0].trim()
    chosenRef.current = name
    setQuery(name)
    setPlaces([])
  }

  let head: { title: string; sub?: string; icon: string; tone?: 'ok' | 'bad' | 'warn' }
  let body: React.ReactNode
  let foot: React.ReactNode
  if (phase.k === 'done') {
    head = { title: 'Заявка отправлена', icon: RESULT_ICON.ok, tone: 'ok',
      sub: `Модератор проверит событие, место и архив. Статус придёт на ${email}: опубликовано со ссылкой или отклонено с причиной.` }
    body = (
      <div className="pm-row-tight pp-number">
        <span className="pm-dim-text">Номер заявки</span>
        <span className="pp-code">{phase.publicId}</span>
        <button type="button" className="pp-copy" onClick={() => navigator.clipboard?.writeText(phase.publicId).then(() => setCopied(true))}>
          {copied ? 'Скопировано' : 'Скопировать'}
        </button>
      </div>
    )
    foot = (
      <>
        <button type="button" className="pp-nav" onClick={() => { reset(); setCopied(false) }}><Chip d={PLUS} />Предложить ещё</button>
        <button type="button" className="pp-nav pp-next" onClick={close}>Вернуться к карте<Chip d={ARROW} /></button>
      </>
    )
  } else if (phase.k === 'fail') {
    const f = phase.status === 413
      ? { tone: 'bad' as const, title: `Архив больше ${MAX_MB} МБ`,
          text: `Сервер не принял файл: ${mb(d.file?.size ?? 0)} МБ при лимите ${MAX_MB}. Уберите видео и крупные изображения, соберите ZIP заново - остальные поля заявки сохранены.`,
          primary: ['Выбрать другой архив', () => { clearFile(); setPhase({ k: 'form' }); setStep(stepOf('archive')); fileRef.current?.click() }] }
      : phase.status === 429
        ? { tone: 'warn' as const, title: 'Слишком много заявок',
            text: `С этого адреса уже отправлено 5 заявок за час. Попробуйте через ${Math.ceil((phase.retryAfter ?? 3600) / 60)} мин - черновик сохранён.`,
            primary: ['Повторить позже', close] }
        : { tone: 'bad' as const, title: phase.status === 0 ? 'Нет связи с сервером' : 'Сервер не принял заявку',
            text: phase.status === 0
              ? 'Заявка не ушла. Проверьте соединение и отправьте снова - поля и архив на месте.'
              : `${phase.message ?? 'Ошибка на сервере'}. Попробуйте отправить ещё раз - поля и архив на месте.`,
            primary: ['Повторить отправку', send] }
    const [pLabel, pRun] = f.primary as [string, () => void]
    head = { title: f.title, sub: f.text, icon: RESULT_ICON[f.tone], tone: f.tone }
    body = <span className="pp-tag" role="alert">Код ответа: {phase.status === 0 ? 'нет сети' : phase.status}</span>
    foot = (
      <>
        <button type="button" className="pp-nav" onClick={() => setPhase({ k: 'form' })}><Chip d={ARROW} back />Назад к заявке</button>
        <button type="button" className="pp-nav pp-next" onClick={pRun}>{pLabel}<Chip d={ARROW} /></button>
      </>
    )
  } else if (!ready && phase.k === 'form') {
    // Без готового сайта заявку не собрать: сначала спрашиваем, при "нет" ведём на инструкцию.
    head = { title: 'Сайт события готов?', sub: 'К заявке прикладывается ZIP-архив сайта о событии. Если сайта ещё нет, инструкция поможет его сделать.', icon: STEPS[stepOf('archive')].icon }
    body = (
      <div className="pp-choices">
        {[
          { title: 'Да, сайт готов', sub: 'Заполнить заявку и приложить архив', run: () => setReady(true) },
          { title: 'Нет, сайта пока нет', sub: 'Инструкция: как сделать сайт за 8 шагов', run: () => navigate('/guide') },
        ].map(c => (
          <button key={c.title} type="button" className="pp-choice" style={{ '--c': 'var(--gold)' } as React.CSSProperties} onClick={c.run}>
            <span className="pm-grow">
              <span className="pp-choice-title">{c.title}</span>
              <span className="pp-choice-sub">{c.sub}</span>
            </span>
            <span className="pp-arrow"><Glyph d={ARROW} /></span>
          </button>
        ))}
      </div>
    )
  } else {
    head = STEPS[step]
    const field = STEPS[step].fields[0]
    body = (
      <fieldset key={step} className="pp-fields" disabled={sending}>
        {field === 'type' && (
          <>
            <div className="pp-choices" role="radiogroup" aria-label="Тип события">
              {TYPES.map(t => (
                <button key={t} type="button" role="radio" aria-checked={d.type === t}
                  className={`pp-choice${d.type === t ? ' on' : ''}`} style={{ '--c': TYPE_COLOR[t] } as React.CSSProperties}
                  onClick={() => { set('type', t, 'type'); go(1) }}>
                  <span className="pp-choice-icon">
                    <svg viewBox="18 9.5 28 28" aria-hidden fill="none" stroke="currentColor" strokeWidth={2.6} strokeLinecap="round" strokeLinejoin="round">
                      {GLYPH[t].map(p => <path key={p} d={p} />)}
                    </svg>
                  </span>
                  <span className="pm-grow">
                    <span className="pp-choice-title">{TYPE_LABEL[t]}</span>
                    <span className="pp-choice-sub">{TYPE_SUB[t]}</span>
                  </span>
                  <span className="pp-arrow"><Glyph d={ARROW} /></span>
                </button>
              ))}
            </div>
            {visible('type') && <span className="pp-error">{visible('type')}</span>}
          </>
        )}

        {field === 'title' && (
          <div className="pp-stack">
            <Text label="Название" error={visible('title')} count={`${d.title.length} / 100`}>
              <input value={d.title} maxLength={100} placeholder="Например, Куликовская битва" autoFocus={!mobile}
                onChange={e => set('title', e.target.value, 'title')} onBlur={() => d.title && show('title')} />
            </Text>
            <Text label="Дата события" error={visible('date')} hint="ДД.ММ.ГГГГ, от 862 года">
              <>
                <input value={d.date} inputMode="numeric" placeholder="ДД.ММ.ГГГГ" aria-label="Дата события"
                  onChange={e => set('date', maskDate(e.target.value), 'date')} onBlur={() => d.date && show('date')} />
                <button type="button" className="pm-link" onClick={openCalendar}>Календарь</button>
                <input ref={pickerRef} type="date" className="pp-picker" tabIndex={-1} aria-hidden min="0862-01-01" max={todayIso()}
                  onChange={e => { if (e.target.value) { set('date', fromIso(e.target.value), 'date'); show('date') } }} />
              </>
            </Text>
            <Text label="Описание" error={visible('description')} count={`${d.description.length} / 450`}>
              <textarea className="pp-area" value={d.description} maxLength={450} placeholder="8 сентября 1380 года войско Дмитрия Донского разбило Мамая у слияния Дона и Непрядвы"
                onChange={e => set('description', e.target.value, 'description')} onBlur={() => d.description && show('description')} />
            </Text>
          </div>
        )}

        {field === 'place' && (
          <>
            <div className="pp-search">
              <div className="pm-field pm-glow">
                <input value={query} maxLength={200} placeholder="Например, Прохоровка" aria-label="Поиск места"
                  onChange={e => setQuery(e.target.value)} />
              </div>
              <Presence>{places.length > 0 && (
                <ul className="pp-places" role="listbox">
                  {places.map(p => (
                    <li key={`${p.lat},${p.lon},${p.display_name}`}>
                      <button type="button" role="option" aria-selected={false} onClick={() => choosePlace(p)}>{p.display_name}</button>
                    </li>
                  ))}
                </ul>
              )}</Presence>
            </div>
            <div className={`pp-map${visible('place') ? ' bad' : ''}`}>
              <MapView ref={mapRef} events={[]} isActive={none} selectedId={null} onSelect={noop} pad={12} labels
                pick={d.lat !== null && d.lng !== null ? { lat: d.lat, lng: d.lng } : null} onPick={sending ? undefined : pick} />
            </div>
            {d.lat !== null && d.lng !== null
              ? <div className="pp-coord"><b>{coord(d.lat, 'с.ш.', 'ю.ш.')}</b> / <b>{coord(d.lng, 'в.д.', 'з.д.')}</b></div>
              : visible('place') && <span className="pp-error">{visible('place')}</span>}
          </>
        )}

        {field === 'archive' && (
          <Text group error={d.file ? undefined : visible('archive')}>
            {d.file ? (
              <FileCard file={d.file} onRemove={clearFile} {...(d.zip === 'checking'
                ? { status: 'Проверяем архив...', progress: 'busy' as const }
                : d.zip === 'ok'
                  ? { status: 'index.html найден', tone: 'ok' as const, value: 'Готово', progress: 1 }
                  : { status: errors.archive, tone: 'bad' as const })} />
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
        )}

        {field === 'telegram' && (
          <div className="pp-stack">
            <Text label="Telegram" error={visible('telegram')} hint="Для вопросов модератора">
              <input value={d.telegram} maxLength={33} placeholder="ivan_petrov" autoCapitalize="none" spellCheck={false} autoFocus={!mobile}
                onChange={e => set('telegram', e.target.value.replace(/^@/, ''), 'telegram')} onBlur={() => d.telegram && show('telegram')} />
            </Text>
            <Check checked={d.consent} error={visible('consent')} disabled={sending} onChange={v => set('consent', v, 'consent')}>
              Даю <a href="/consent/submission" target="_blank" rel="noopener">согласие на обработку персональных данных</a> и
              принимаю <a href="/privacy" target="_blank" rel="noopener">политику</a>
            </Check>
            <Check checked={d.rules} error={visible('rules')} disabled={sending} onChange={v => set('rules', v, 'rules')}>
              Подтверждаю, что соблюдаю <a href="/rules" target="_blank" rel="noopener">правила публикации материалов о третьих лицах</a>
            </Check>
          </div>
        )}
      </fieldset>
    )
    const last = step === STEPS.length - 1
    if (phase.k === 'sending') {
      const part = phase.loaded / (phase.total || 1)
      head = { title: 'Отправляем заявку', sub: 'Не закрывайте окно, пока архив загружается', icon: STEPS[stepOf('archive')].icon }
      body = <FileCard file={d.file!} status="Загрузка архива..." value={`${Math.round(part * 100)} %`} progress={part} />
    }
    foot = phase.k === 'sending'
      ? (
        <>
          <button type="button" className="pp-nav" onClick={() => abortRef.current?.()}><Chip d={CROSS} />Отменить</button>
        </>
      )
      : (
        <>
          {/* С первого шага "Назад" возвращает к вопросу о готовности сайта, вперёд там ведёт выбор типа. */}
          <button type="button" className="pp-nav" onClick={() => step ? setStep(step - 1) : setReady(false)}><Chip d={ARROW} back />Назад</button>
          {step > 0 && <button type="submit" className="pp-nav pp-next">
            {last ? (mobile ? 'Отправить' : 'Отправить на модерацию') : 'Далее'}<Chip d={last ? SEND : ARROW} />
          </button>}
        </>
      )
  }

  const form = phase.k === 'form' || sending
  return (
    <div className={`pm-root pp-page${mobile ? ' mobile' : ''}${open ? '' : ' out'}`} role="dialog" aria-modal="true" aria-labelledby="pp-title">
      <form className={`pp-card${form && STEPS[step].fields[0] === 'place' ? ' wide' : ''}`} noValidate
        onSubmit={e => { e.preventDefault(); if (phase.k === 'form') next() }}>
        <input ref={fileRef} type="file" accept=".zip,application/zip" hidden onChange={e => pickFile(e.target.files?.[0])} />
        <button type="button" className="pp-x pp-close" aria-label="Закрыть" disabled={sending} onClick={close}>×</button>
        <div className="pp-head">
          {(ready || !form) && <div className="pp-steps" aria-label={`Шаг ${step + 1} из ${STEPS.length}`}>
            {STEPS.map((s, i) => (
              <button key={s.title} type="button" aria-label={s.title} disabled={!form || sending || i > reached}
                className={phase.k === 'done' || i < step ? 'done' : i === step ? 'on' : ''} onClick={() => setStep(i)}><i /></button>
            ))}
          </div>}
          <div className={`pp-hero${head.tone ? ` ${head.tone}` : ''}`} key={head.icon}>
            <span className="pp-hero-back" />
            <span className="pp-hero-glass"><Glyph d={head.icon} /></span>
          </div>
          <h1 id="pp-title" className="pp-title">{head.title}</h1>
          {head.sub && <p className="pp-sub">{head.sub}</p>}
        </div>
        <div className="pp-body">{body}</div>
        {ready && <div className="pp-foot">{foot}</div>}
      </form>
    </div>
  )
}

// Карточка архива: документ с плашкой ZIP и панель статуса с полосой. progress: доля 0..1 или 'busy' без процента.
const FileCard: React.FC<{
  file: File; status?: string; tone?: 'ok' | 'bad'; value?: string; progress?: number | 'busy'; onRemove?: () => void
}> = ({ file, status, tone, value, progress, onRemove }) => (
  <div className={`pp-upload${tone ? ` ${tone}` : ''}`}>
    {onRemove && <button type="button" className="pp-x pp-upload-x" aria-label="Убрать архив" onClick={onRemove}>×</button>}
    <div className="pp-upload-file">
      <span className="pp-doc" aria-hidden>
        <svg viewBox="0 0 56 70"><path d="M10 1h26l19 19v39a10 10 0 0 1-10 10H10A10 10 0 0 1 0 59V11A10 10 0 0 1 10 1Z" /><path className="fold" d="M36 1v11a8 8 0 0 0 8 8h11" /></svg>
        <span className="pp-doc-tag">ZIP</span>
      </span>
      <span className="pm-grow">
        <span className="pp-file-name">{file.name}</span>
        <span className="pp-file-size">{mb(file.size)} МБ</span>
      </span>
    </div>
    <div className="pp-upload-panel" role="status">
      <div className="pp-upload-row">
        {tone ? <Glyph d={RESULT_ICON[tone]} /> : <span className="pp-spin" />}
        <span className="pm-grow">{status}</span>
        {value && <span className="pp-upload-value">{value}</span>}
      </div>
      {progress !== undefined && (
        <div className={`pp-track${progress === 'busy' ? ' busy' : ''}`}>
          <i style={progress === 'busy' ? undefined : { width: `${progress * 100}%` }} />
        </div>
      )}
    </div>
  </div>
)

// Иконка в плашке внутри кнопки, как в референсе.
const Chip: React.FC<{ d: string; back?: boolean }> = ({ d, back }) => <span className="pp-chip"><Glyph d={d} back={back} /></span>

const Glyph: React.FC<{ d: string; back?: boolean }> = ({ d, back }) => (
  <svg className={`pp-glyph${back ? ' back' : ''}`} viewBox="0 0 24 24" aria-hidden fill="none" stroke="currentColor"
    strokeWidth={1.7} strokeLinecap="round" strokeLinejoin="round"><path d={d} /></svg>
)

const Text: React.FC<{
  label?: string; error?: string; hint?: string; count?: string; labelBad?: boolean; group?: boolean; children: React.ReactNode
}> = ({ label, error, hint, count, labelBad, group, children }) => {
  const Tag = group ? 'div' : 'label'
  return (
  <Tag className={`pp-field${error ? ' bad' : ''}`}>
    {label && <span className={`pp-label${labelBad ? ' bad' : ''}`}>{label}</span>}
    {group ? children : <div className={`pm-field pm-glow${error ? ' bad' : ''}`}>{children}</div>}
    {error ? <span className="pp-error">{error}</span> : hint ? <span className="pm-dim-text">{hint}</span> : null}
    {count && !error && <span className="pp-count">{count}</span>}
  </Tag>
  )
}
