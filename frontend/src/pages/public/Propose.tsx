import React, { useEffect, useRef, useState } from 'react'
import { submitProposal, TYPE_LABEL, TYPES, zipHasRootIndex, type EventType } from './api.ts'
import './propose.css'

// Значения из "Предложить событие.dc.html" и валидации POST /api/v1/submissions.
const MAX_MB = 50
const MIN_YEAR = 862

interface Draft {
  title: string; date: string; type: EventType | ''; description: string
  email: string; telegram: string; file: File | null
  zip: 'none' | 'checking' | 'ok' | string // строка - текст ошибки архива
}
const EMPTY: Draft = { title: '', date: '', type: '', description: '', email: '', telegram: '', file: null, zip: 'none' }

type Field = 'title' | 'date' | 'type' | 'description' | 'archive' | 'email' | 'telegram'
type Errors = Partial<Record<Field, string>>
const STEP1: Field[] = ['title', 'date', 'type', 'description', 'archive']
const SERVER_FIELD: Record<string, Field> = {
  title: 'title', eventDate: 'date', eventType: 'type', description: 'description',
  archive: 'archive', email: 'email', telegramUsername: 'telegram',
}

type Phase =
  | { k: 'form' }
  | { k: 'sending'; loaded: number; total: number }
  | { k: 'done'; id: number }
  | { k: 'fail'; status: number; message?: string; retryAfter?: number }

// Мелкие файлы не показываем как "0 МБ".
const mb = (bytes: number) => (bytes > 0 && bytes < 10486 ? 0.01 : bytes / 1048576)
  .toLocaleString('ru-RU', { maximumFractionDigits: bytes < 1048576 ? 2 : 1 })
const pad = (n: number) => String(n).padStart(2, '0')
const todayIso = () => { const d = new Date(); return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` }

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
  const title = d.title.trim().length
  if (title < 3 || title > 100) e.title = 'Укажите название - от 3 до 100 символов'
  const iso = toIso(d.date)
  if (!iso || +iso.slice(0, 4) < MIN_YEAR) e.date = 'Неверная дата. Формат ДД.ММ.ГГГГ, от 862 года'
  else if (iso > todayIso()) e.date = 'Дата не может быть в будущем'
  if (!d.type) e.type = 'Выберите тип события'
  const desc = d.description.trim().length
  if (desc === 0 || desc > 450) e.description = 'Опишите событие - до 450 символов'
  if (!d.file) e.archive = 'Прикрепите ZIP-архив сайта'
  else if (d.zip !== 'ok' && d.zip !== 'checking') e.archive = d.zip
  const email = d.email.trim()
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/.test(email)) e.email = 'Проверьте адрес: нужен домен вида example.ru'
  else if (email.length > 70) e.email = 'Не больше 70 символов'
  if (!/^[A-Za-z0-9_]{5,32}$/.test(d.telegram.trim())) e.telegram = 'Без "@", только латиница, цифры и _, от 5 до 32 символов'
  return e
}

const fieldsWord = (n: number) => (n % 10 === 1 && n % 100 !== 11 ? 'поле' : n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 12 || n % 100 > 14) ? 'поля' : 'полей')

export const ProposeModal: React.FC<{ open: boolean; mobile: boolean; onClose: () => void }> = ({ open, mobile, onClose }) => {
  const [d, setD] = useState<Draft>(EMPTY)
  const [step, setStep] = useState<1 | 2>(1)
  const [phase, setPhase] = useState<Phase>({ k: 'form' })
  const [shown, setShown] = useState<Set<Field>>(new Set())
  const [server, setServer] = useState<Errors>({})
  const [banner, setBanner] = useState(0)
  const [drag, setDrag] = useState(false)
  const [copied, setCopied] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)
  const pickerRef = useRef<HTMLInputElement>(null)
  const abortRef = useRef<() => void>()
  const sending = phase.k === 'sending'

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape' && !sending) onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, sending, onClose])

  if (!open) return null

  const local = validate(d)
  const errors: Errors = { ...local, ...server }
  const visible = (f: Field) => (shown.has(f) ? errors[f] : undefined)
  const show = (...f: Field[]) => setShown(s => new Set([...s, ...f]))
  const set = <K extends keyof Draft>(k: K, v: Draft[K], f?: Field) => {
    setD(prev => ({ ...prev, [k]: v }))
    if (f) setServer(({ [f]: _, ...rest }) => rest)
  }

  const reset = () => { setD(EMPTY); setStep(1); setPhase({ k: 'form' }); setShown(new Set()); setServer({}); setBanner(0) }
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

  const step1Filled = d.title.trim() && d.date && d.type && d.description.trim() && d.file && d.zip !== 'checking'
  const step2Filled = d.email.trim() && d.telegram.trim()
  const next = () => {
    const bad = STEP1.filter(f => errors[f])
    show(...STEP1)
    setBanner(bad.length)
    if (!bad.length) setStep(2)
  }

  const send = async () => {
    const bad = (Object.keys(errors) as Field[]).filter(f => errors[f])
    show('email', 'telegram')
    if (bad.some(f => STEP1.includes(f))) { setStep(1); show(...STEP1); setBanner(bad.filter(f => STEP1.includes(f)).length); return }
    if (bad.length) return setBanner(bad.length)
    const body = new FormData()
    body.set('title', d.title.trim())
    body.set('description', d.description.trim())
    body.set('eventDate', toIso(d.date)!)
    body.set('eventType', d.type)
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
      const keys = Object.keys(mapped) as Field[]
      setServer(mapped)
      show(...keys)
      const first = keys.filter(f => STEP1.includes(f))
      setBanner(first.length || keys.length)
      if (first.length) setStep(1)
      return setPhase({ k: 'form' })
    }
    setPhase({ k: 'fail', status: res.status, message: res.message, retryAfter: res.retryAfter })
  }

  const openCalendar = () => {
    const el = pickerRef.current
    if (!el) return
    el.value = toIso(d.date) ?? ''
    try { el.showPicker() } catch { el.focus() }
  }

  const summary = (
    <div className="pp-summary">
      <div className="pp-thumb pm-hatch" />
      <div className="pm-grow">
        <div className="pp-summary-title">{[d.title.trim(), toIso(d.date)?.slice(0, 4).replace(/^0/, ''), mobile ? '' : d.type && TYPE_LABEL[d.type]].filter(Boolean).join(' · ')}</div>
        <div className="pm-dim-text">{mobile && d.type ? `${TYPE_LABEL[d.type]} · ` : ''}{d.file?.name}{!mobile && d.file ? ` · ${mb(d.file.size)} МБ` : ''}</div>
      </div>
    </div>
  )

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
          {mobile ? 'Статус придёт на ' : 'Модератор проверит событие и архив. Статус придёт на '}
          {d.email.trim()}{mobile ? ' - ' : ': '}опубликовано со ссылкой или отклонено с причиной.
        </p>
      </div>
    )
    foot = (
      <>
        <button type="button" className="pm-btn" onClick={close}>Вернуться к карте</button>
        <button type="button" className="pm-btn-ghost" onClick={() => { reset(); setCopied(false) }}>Предложить ещё</button>
      </>
    )
  } else if (phase.k === 'fail') {
    const f = phase.status === 413
      ? { tone: 'bad', glyph: '!', title: `Архив больше ${MAX_MB} МБ`, code: '413',
          text: `Сервер не принял файл: ${mb(d.file?.size ?? 0)} МБ при лимите ${MAX_MB}. Уберите видео и крупные изображения, соберите ZIP заново - остальные поля заявки сохранены.`,
          primary: ['Выбрать другой архив', () => { clearFile(); setStep(1); setPhase({ k: 'form' }); fileRef.current?.click() }],
          secondary: ['Назад к заявке', () => setPhase({ k: 'form' })] }
      : phase.status === 429
        ? { tone: 'warn', glyph: '⏲', title: 'Слишком много заявок', code: '429',
            text: `С этого адреса уже отправлено 5 заявок за час. Попробуйте через ${Math.ceil((phase.retryAfter ?? 3600) / 60)} мин - черновик сохранён.`,
            primary: ['Повторить позже', close], secondary: ['Вернуться к карте', close] }
        : { tone: 'bad', glyph: '×', title: phase.status === 0 ? 'Нет связи с сервером' : 'Сервер не принял заявку',
            code: phase.status === 0 ? 'network' : String(phase.status),
            text: phase.status === 0
              ? 'Заявка не ушла. Проверьте соединение и отправьте снова - поля и архив на месте.'
              : `${phase.message ?? 'Ошибка на сервере'}. Попробуйте отправить ещё раз - поля и архив на месте.`,
            primary: ['Повторить отправку', send], secondary: ['Сохранить и закрыть', close] }
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
        <button type="button" className="pm-btn-text" onClick={sRun}>{sLabel}</button>
        <div className="pm-grow" />
        <button type="button" className="pm-btn" onClick={pRun}>{pLabel}</button>
      </>
    )
  } else if (step === 1) {
    body = (
      <>
        <Text label={mobile ? 'Название' : 'Название события'} error={visible('title')} count={mobile ? undefined : `${d.title.length} / 100`}>
          <input className="pp-input" value={d.title} maxLength={100} placeholder="Например, Куликовская битва"
            onChange={e => set('title', e.target.value, 'title')} onBlur={() => d.title && show('title')} />
        </Text>
        <div className="pp-pair">
          <Text label="Дата события" error={visible('date')}>
            <div className={`pp-input pp-date${visible('date') ? ' bad' : ''}`}>
              <input value={d.date} inputMode="numeric" placeholder="ДД.ММ.ГГГГ" aria-label="Дата события"
                onChange={e => set('date', maskDate(e.target.value), 'date')} onBlur={() => d.date && show('date')} />
              <button type="button" className="pm-link" onClick={openCalendar}>Календарь</button>
              <input ref={pickerRef} type="date" className="pp-picker" tabIndex={-1} aria-hidden min="0862-01-01" max={todayIso()}
                onChange={e => { if (e.target.value) { set('date', fromIso(e.target.value), 'date'); show('date') } }} />
            </div>
          </Text>
          <Text group label={mobile ? 'Тип' : 'Тип события'} error={visible('type')} labelBad={!!visible('type')}>
            <div className="pp-types" role="radiogroup" aria-label="Тип события">
              {TYPES.map(t => (
                <button key={t} type="button" role="radio" aria-checked={d.type === t}
                  className={`pp-type${d.type === t ? ' on' : ''}${visible('type') ? ' bad' : ''}`}
                  onClick={() => { set('type', t, 'type'); show('type') }}>
                  {!mobile && <span className="pp-radio" />}{TYPE_LABEL[t]}
                </button>
              ))}
            </div>
          </Text>
        </div>
        <Text label="Описание" error={visible('description')} count={`${d.description.length} / 450`}>
          <textarea className="pp-input pp-area" value={d.description} maxLength={450} placeholder="Что произошло, где и почему это важно"
            onChange={e => set('description', e.target.value, 'description')} onBlur={() => d.description && show('description')} />
        </Text>
        <Text label={mobile ? undefined : 'Сайт события · ZIP-архив'} error={d.file ? undefined : visible('archive')}>
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
      </>
    )
    foot = mobile
      ? <button type="button" className="pm-btn pp-wide" disabled={!step1Filled} onClick={next}>Далее · контакты</button>
      : (
        <>
          <span className="pm-dim-text">Шаг 1 из 2</span>
          <div className="pm-grow" />
          <button type="button" className="pm-btn-text" onClick={close}>Отмена</button>
          <button type="button" className="pm-btn" disabled={!step1Filled} onClick={next}>Далее</button>
        </>
      )
  } else {
    body = (
      <>
        <Text label="Email" error={visible('email')} hint={mobile ? 'Статус заявки придёт сюда' : 'На этот адрес придёт статус заявки. До 70 символов'}>
          <input className="pp-input" type="email" autoComplete="email" value={d.email} maxLength={70} placeholder="ivan@example.ru" disabled={sending}
            onChange={e => set('email', e.target.value, 'email')} onBlur={() => d.email && show('email')} />
        </Text>
        <Text label="Telegram" error={visible('telegram')} hint={mobile ? 'Без "@", латиница, цифры и _, 5-32 символа' : 'Без "@", 5-32 символа: латиница, цифры, _. Нужен модератору для вопросов по заявке'}>
          <input className="pp-input" value={d.telegram} maxLength={32} placeholder="ivan_petrov" autoCapitalize="none" spellCheck={false} disabled={sending}
            onChange={e => set('telegram', e.target.value, 'telegram')} onBlur={() => d.telegram && show('telegram')} />
        </Text>
        {summary}
        {phase.k === 'sending' && (
          <div className="pp-progress" role="status">
            <div className="pp-progress-row">
              <span className="pm-mono">{d.file?.name}</span>
              <span className="pp-muted">{mb(phase.loaded)} из {mb(phase.total)} МБ · {Math.round((phase.loaded / (phase.total || 1)) * 100)} %</span>
            </div>
            <div className="pp-bar"><div style={{ width: `${(phase.loaded / (phase.total || 1)) * 100}%` }} /></div>
            <div className="pm-dim-text">Не закрывайте окно - архив ещё загружается</div>
          </div>
        )}
      </>
    )
    foot = sending
      ? (
        <>
          <button type="button" className="pm-btn pp-busy" disabled><span className="pp-spin" />Отправляем</button>
          <button type="button" className="pm-btn-ghost" onClick={() => abortRef.current?.()}>Отменить загрузку</button>
        </>
      )
      : mobile
        ? (
          <>
            <button type="button" className="pm-btn pp-wide" disabled={!step2Filled} onClick={send}>Отправить заявку</button>
            {d.file && <div className="pm-dim-text pp-center">Архив {mb(d.file.size)} МБ загрузится при отправке</div>}
          </>
        )
        : (
          <>
            <button type="button" className="pm-btn-text" onClick={() => { setStep(1); setBanner(0) }}>&lt;- Назад</button>
            <div className="pm-grow" />
            <button type="button" className="pm-btn" disabled={!step2Filled} onClick={send}>Отправить заявку</button>
          </>
        )
  }

  const inForm = phase.k === 'form' || phase.k === 'sending'
  const back = mobile && step === 2 && inForm
  return (
    <div className={`pm-root pp-overlay${mobile ? ' mobile' : ''}`}>
      <div className="pp-modal" role="dialog" aria-modal="true" aria-labelledby="pp-title">
        {mobile ? (
          <div className="pp-head">
            {phase.k === 'done'
              ? <img src="/logo.svg" alt="" className="pp-logo" />
              : (
                <button type="button" className="pp-x" disabled={sending} aria-label={back ? 'Назад' : 'Закрыть'}
                  onClick={back ? () => setStep(1) : close}>{back ? '<' : '×'}</button>
              )}
            <h2 id="pp-title" className="pp-head-title">{phase.k === 'done' ? 'Страна Героев' : back ? 'Контакты' : 'Предложить событие'}</h2>
          </div>
        ) : (
          <div className="pp-head">
            <div className="pm-grow">
              <h2 id="pp-title" className="pp-title">Предложить событие</h2>
              <div className="pm-dim-text">Модератор проверит заявку и поставит точку на карте</div>
            </div>
            <button type="button" className="pp-x" aria-label="Закрыть" disabled={sending} onClick={close}>×</button>
          </div>
        )}
        {inForm && !(mobile && step === 2) && (
          <div className="pp-steps">
            <span className={`pp-step-n ${step === 1 ? 'on' : 'done'}`}>{step === 1 ? '1' : '✓'}</span>
            <span className={step === 1 ? '' : 'pp-gold'}>Событие</span>
            <span className={`pp-step-line${step === 2 ? ' done' : ''}`} />
            <span className={`pp-step-n ${step === 2 ? 'on' : ''}`}>2</span>
            <span className={step === 2 ? '' : 'pp-dim'}>Контакты</span>
          </div>
        )}
        {inForm && banner > 0 && Object.keys(errors).some(f => (step === 1 ? STEP1 : ['email', 'telegram']).includes(f as Field) && shown.has(f as Field)) && (
          <div className="pp-banner" role="alert">
            <span className="pp-banner-icon">!</span>
            Исправьте {banner} {fieldsWord(banner)} - заявка не отправлена
          </div>
        )}
        <div className={`pp-body${inForm ? '' : ' result'}`}>{body}</div>
        <div className={`pp-foot${mobile ? ' col' : ''}`}>{foot}</div>
      </div>
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
