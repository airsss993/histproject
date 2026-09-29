import React, { useEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Logo, useIsMobile } from './PublicMap.tsx'
import { registerUser, requestLoginCode, verifyLoginCode, type ApiResult } from './api.ts'
import { Check } from '../../shared/ui/Field.tsx'
import './public.css'
import './auth.css'

// Правая половина экрана. Картинку сгенерируем позже: путь от public/, например '/auth-art.jpg'.
const ART: string | null = null

const CODE_LEN = 6
const NO_ACCOUNT = 'Аккаунта с этой почтой нет'
const TAKEN = 'Эта почта уже зарегистрирована'
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

type Mode = 'login' | 'register'
type Field = 'firstName' | 'lastName' | 'email' | 'code' | 'consent'
type Errors = Partial<Record<Field, string>>

const COPY: Record<Mode, { title: string; sub: string; backSub: string; alt: string; altLink: string; altTo: string }> = {
  login: { title: 'С возвращением', sub: 'Войдите по коду из письма', backSub: 'Чтобы предложить событие, войдите по коду из письма', alt: 'Нет аккаунта?', altLink: 'Зарегистрироваться', altTo: '/register' },
  register: { title: 'Создать аккаунт', sub: 'Код для входа придёт на почту', backSub: 'Чтобы предложить событие, создайте аккаунт. Код для входа придёт на почту', alt: 'Уже есть аккаунт?', altLink: 'Войти', altTo: '/login' },
}

const Arrow = () => (
  <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d="M5 12h14M13 6l6 6-6 6" />
  </svg>
)

// Общая ошибка по ответу API. Поля и известные коды разбирает вызывающий.
function problem(res: Extract<ApiResult, { ok: false }>) {
  if (res.status === 0) return 'Нет связи с сервером. Проверьте интернет и попробуйте ещё раз.'
  if (res.status === 429) return `Код уже отправлен недавно. Новый можно запросить через ${res.retryAfter ?? 60} с.`
  return res.message ?? 'Что-то пошло не так. Попробуйте ещё раз.'
}

export const AuthPage: React.FC<{ mode: Mode }> = ({ mode }) => {
  const navigate = useNavigate()
  const mobile = useIsMobile()
  // back - куда вернуть после входа; переходы вход <-> регистрация его сохраняют.
  const { email: prefill = '', back } = (useLocation().state as { email?: string; back?: string } | null) ?? {}
  const copy = COPY[mode]

  const [form, setForm] = useState({ firstName: '', lastName: '', email: prefill })
  const [remember, setRemember] = useState(false)
  const [consent, setConsent] = useState(false)
  const [errors, setErrors] = useState<Errors>({})
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [sentTo, setSentTo] = useState<string | null>(null)
  const [code, setCode] = useState('')
  const [cooldown, setCooldown] = useState(0)

  useEffect(() => {
    if (cooldown <= 0) return
    const t = setTimeout(() => setCooldown(c => c - 1), 1000)
    return () => clearTimeout(t)
  }, [cooldown])

  const set = (key: 'firstName' | 'lastName' | 'email') => (e: React.ChangeEvent<HTMLInputElement>) => {
    setForm(f => ({ ...f, [key]: e.target.value }))
    setErrors(er => ({ ...er, [key]: undefined }))
  }

  const send = async (e?: React.FormEvent) => {
    e?.preventDefault()
    if (busy) return
    const email = form.email.trim()
    const next: Errors = {}
    if (mode === 'register') {
      if (!form.firstName.trim()) next.firstName = 'Введите имя'
      if (!form.lastName.trim()) next.lastName = 'Введите фамилию'
      if (!consent) next.consent = 'Без согласия зарегистрироваться нельзя'
    }
    if (!email) next.email = 'Введите почту'
    else if (!EMAIL_RE.test(email)) next.email = 'Похоже, в адресе опечатка'
    setErrors(next)
    setNote('')
    if (Object.keys(next).length) return

    setBusy(true)
    const res = mode === 'register'
      ? await registerUser({ firstName: form.firstName, lastName: form.lastName, email, consent })
      : await requestLoginCode(email)
    setBusy(false)
    if (res.ok) {
      setSentTo(email)
      setCode('')
      setCooldown(60)
      return
    }
    if (res.fields) setErrors(res.fields)
    else if (res.code === 'email_taken') setErrors({ email: TAKEN })
    else if (res.code === 'user_not_found') setErrors({ email: NO_ACCOUNT })
    else setNote(problem(res))
  }

  const verify = async (value: string) => {
    if (!sentTo || busy) return
    setBusy(true)
    setNote('')
    const res = await verifyLoginCode(sentTo, value, remember)
    setBusy(false)
    if (res.ok) {
      navigate(back ?? '/map', { replace: true })
      return
    }
    setCode('')
    if (res.code === 'invalid_code') setErrors({ code: 'Неверный код. Проверьте письмо и введите ещё раз' })
    else if (res.code === 'code_expired') setErrors({ code: 'Код устарел или кончились попытки. Запросите новый' })
    else setNote(problem(res))
  }

  const resend = async () => {
    setErrors({})
    await send()
  }

  return (
    <div className={`pm-root au-page${mobile ? ' mobile' : ''}`}>
      <main className="au-side">
        <div className="au-header">
          <Logo short={mobile} onClick={() => navigate('/map')} />
          <Link to="/map" className="pm-btn-text">На карту</Link>
        </div>

        <div className="au-center">
          <section className="au-card" aria-labelledby="au-title">
            {sentTo ? (
              <div className="au-step" key="code">
                <h1 id="au-title" className="au-title">Проверьте почту</h1>
                <p className="au-sub">Отправили код на <b>{sentTo}</b></p>
                <CodeInput value={code} bad={!!errors.code} disabled={busy}
                  onChange={v => { setCode(v); setErrors({}); if (v.length === CODE_LEN) verify(v) }} />
                <p className="au-error" role="alert">{errors.code ?? note}</p>
                <div className="au-row">
                  <button type="button" className="pm-btn-text au-small" onClick={() => { setSentTo(null); setErrors({}); setNote('') }}>
                    Изменить почту
                  </button>
                  {cooldown > 0
                    ? <span className="au-muted">Новый код через {cooldown} с</span>
                    : <button type="button" className="pm-btn-text au-small" onClick={resend} disabled={busy}>Отправить ещё раз</button>}
                </div>
              </div>
            ) : (
              <form className="au-step" key={mode} onSubmit={send} noValidate>
                <h1 id="au-title" className="au-title">{copy.title}</h1>
                <p className="au-sub">{back === '/admin'
                  ? mode === 'login' ? 'Войдите по коду, чтобы открыть панель' : 'Создайте аккаунт по почте, которой выдана роль модератора'
                  : back ? copy.backSub : copy.sub}</p>

                <div className="au-fields">
                  {mode === 'register' && (
                    <div className="au-pair">
                      <Input label="Имя" value={form.firstName} onChange={set('firstName')} error={errors.firstName}
                        autoComplete="given-name" maxLength={50} autoFocus />
                      <Input label="Фамилия" value={form.lastName} onChange={set('lastName')} error={errors.lastName}
                        autoComplete="family-name" maxLength={50} />
                    </div>
                  )}
                  <Input label="Почта" type="email" value={form.email} onChange={set('email')} error={errors.email}
                    autoComplete="email" inputMode="email" maxLength={254} autoFocus={mode === 'login'}
                    placeholder="name@mail.ru"
                    action={
                      <button type="submit" className="au-go" disabled={busy} aria-label="Получить код">
                        {busy ? <i className="au-spin" /> : <Arrow />}
                      </button>
                    } />
                </div>

                {errors.email === NO_ACCOUNT && (
                  <p className="au-hint">
                    <Link to="/register" state={{ email: form.email.trim(), back }}>Зарегистрироваться с этой почтой</Link>
                  </p>
                )}
                {errors.email === TAKEN && (
                  <p className="au-hint">
                    <Link to="/login" state={{ email: form.email.trim(), back }}>Войти с этой почтой</Link>
                  </p>
                )}

                {mode === 'register' && (
                  <Check className="au-consent" checked={consent} error={errors.consent}
                    onChange={v => { setConsent(v); setErrors(er => ({ ...er, consent: undefined })) }}>
                    Даю <a href="/consent/registration" target="_blank" rel="noopener">согласие на обработку персональных данных</a> и
                    принимаю <a href="/privacy" target="_blank" rel="noopener">политику</a>
                  </Check>
                )}
                <div className="au-row">
                  <Check checked={remember} onChange={setRemember}>Запомнить меня</Check>
                </div>
                {note && <p className="au-error" role="alert">{note}</p>}

                <p className="au-alt">
                  {copy.alt} <Link to={copy.altTo} state={{ email: form.email.trim(), back }}>{copy.altLink}</Link>
                </p>
              </form>
            )}
          </section>
        </div>
      </main>

      {!mobile && (
        <aside className="au-art" aria-hidden="true">
          {ART
            ? <img src={ART} alt="" />
            : <div className="au-art-stub"><img src="/logo.svg" alt="" /></div>}
        </aside>
      )}
    </div>
  )
}

type InputProps = React.InputHTMLAttributes<HTMLInputElement> & { label: string; error?: string; action?: React.ReactNode }

const Input: React.FC<InputProps> = ({ label, error, action, ...input }) => {
  const id = React.useId()
  return (
    <div className="au-cell-wrap">
      <label className={`pm-field pm-glow au-field${error ? ' bad' : ''}`} htmlFor={id}>
        <span className="au-field-body">
          <span className="au-label">{label}</span>
          <input id={id} {...input} aria-invalid={!!error} aria-describedby={error ? `${id}-e` : undefined} />
        </span>
        {action}
      </label>
      {error && <p className="au-error" id={`${id}-e`}>{error}</p>}
    </div>
  )
}

// Одно настоящее поле поверх шести клеток: работают вставка, автоподстановка кода из SMS/почты и стирание.
const CodeInput: React.FC<{ value: string; bad: boolean; disabled: boolean; onChange: (v: string) => void }> = ({ value, bad, disabled, onChange }) => {
  const [focused, setFocused] = useState(false)
  const ref = useRef<HTMLInputElement>(null)
  useEffect(() => { if (!disabled) ref.current?.focus() }, [disabled])
  const active = Math.min(value.length, CODE_LEN - 1)
  return (
    <div className={`au-code${bad ? ' bad' : ''}`}>
      <input ref={ref} value={value} disabled={disabled} inputMode="numeric" autoComplete="one-time-code"
        maxLength={CODE_LEN} aria-label="Код из письма" aria-invalid={bad}
        onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}
        onChange={e => onChange(e.target.value.replace(/\D/g, '').slice(0, CODE_LEN))} />
      {Array.from({ length: CODE_LEN }, (_, i) => (
        <span key={i} className={`au-digit pm-glow${focused && i === active ? ' on' : ''}${value[i] ? ' full' : ''}`}>{value[i]}</span>
      ))}
    </div>
  )
}
