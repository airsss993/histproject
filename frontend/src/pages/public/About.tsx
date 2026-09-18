import React, { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Logo, useIsMobile } from './PublicMap.tsx'
import { MapView } from './MapView.tsx'
import { fetchAllEvents, yearOf, type HistEvent } from './api.ts'
import './public.css'
import './about.css'

// Команда и контакты - со страницы "О нас" trudovaya-slava.
// photo - путь от public/, например '/team/sklyarova.jpg'. Без него в портрете буква.
const TEAM: { initial: string; role: string; name: string; photo?: string }[] = [
  {
    initial: 'Д',
    role: 'Менеджер проекта',
    name: 'Дарья Склярова',
  },
  {
    initial: 'А',
    role: 'Разработчик',
    name: 'Артём Джапаридзе',
  },
]

// Ответы сверены с формой заявки (Propose.tsx) и письмами автору (backend notifications).
const FAQ = [
  {
    q: 'Какие события есть на карте?',
    a: 'Военные и политические события истории России с IX по XXI век. У каждого события есть точка на карте, дата, короткое описание и отдельная страница.',
  },
  {
    q: 'Как добавить событие?',
    a: 'Нажмите "Предложить событие" на карте. В форме нужны тип события, название, дата не раньше 862 года, описание до 450 символов и точка на карте. Страницу события прикладывают ZIP-архивом сайта до 50 МБ с index.html в корне. Для связи укажите email и Telegram.',
  },
  {
    q: 'Что происходит после отправки заявки?',
    a: 'Заявку проверяет модератор. На email приходят письма: когда заявку взяли на проверку, когда событие опубликовано (со ссылкой на него) и когда заявка отклонена (с комментарием модератора).',
  },
  {
    q: 'Заявку отклонили. Что делать?',
    a: 'Прочитайте комментарий в письме, исправьте то, что в нём указано, и отправьте заявку заново через ту же форму.',
  },
  {
    q: 'Я нашёл ошибку в событии. Как её поправить?',
    a: 'Напишите нам в Telegram или на почту: ссылки внизу страницы. Укажите событие и приложите источник, если он есть.',
  },
]

// Связь с проектом - контакты менеджера.
const MAIL = 'mailto:doctorin7@gmail.com'
const TELEGRAM = 'https://t.me/dsxwwx_z'

const plural = (n: number) => {
  const d = n % 10, h = n % 100
  return d === 1 && h !== 11 ? 'событие' : d >= 2 && d <= 4 && (h < 12 || h > 14) ? 'события' : 'событий'
}
const always = () => true

const Tag: React.FC<{ children: string }> = ({ children }) => (
  <div className="pm-about-tag"><span>{children}</span><i /></div>
)

// Превью события, пока его нет - штриховка.
const Preview: React.FC<{ e?: HistEvent; className: string }> = ({ e, className }) => (
  e?.previewImageUrl
    ? <img className={className} src={e.previewImageUrl} alt="" loading="lazy" />
    : <span className={`${className} pm-hatch`} />
)

const Star: React.FC = () => (
  <svg className="pm-about-star" viewBox="0 0 24 24" aria-hidden="true">
    <path d="M12 0C12.6 7.2 16.8 11.4 24 12C16.8 12.6 12.6 16.8 12 24C11.4 16.8 7.2 12.6 0 12C7.2 11.4 11.4 7.2 12 0Z" />
  </svg>
)

// Смена animation-duration в CSS пересчитывает угол и кольцо прыгает; playbackRate сохраняет позицию.
// Скорость плавно меняется за 400ms ease-in-out, синхронно с transition логотипа в about.css.
let spinRaf = 0
const spinRate = (el: HTMLElement, to: number) => {
  const anim = el.querySelector('svg')?.getAnimations()[0]
  if (!anim) return
  cancelAnimationFrame(spinRaf)
  const from = anim.playbackRate
  const t0 = performance.now()
  const step = (t: number) => {
    const k = Math.min((t - t0) / 400, 1)
    anim.playbackRate = from + (to - from) * (k < 0.5 ? 4 * k ** 3 : 1 - (2 - 2 * k) ** 3 / 2)
    if (k < 1) spinRaf = requestAnimationFrame(step)
  }
  spinRaf = requestAnimationFrame(step)
}

const BADGE_TEXT = 'Предложить событие · Предложить событие · '

export const About: React.FC = () => {
  const navigate = useNavigate()
  const mobile = useIsMobile()
  const [openFaq, setOpenFaq] = useState<number | null>(0)
  const [events, setEvents] = useState<HistEvent[]>([])
  useEffect(() => {
    const ac = new AbortController()
    fetchAllEvents(ac.signal).then(setEvents).catch(() => {}) // без событий страница остаётся со штриховкой
    return () => ac.abort()
  }, [])
  const previews = events.filter(e => e.previewImageUrl).sort((a, b) => b.id - a.id)
  const latest = previews[0]
  const propose = () => navigate('/map?propose=1')

  return (
    <div className="pm-root pm-about">
      <div role="banner" className="pm-header pm-about-header">
        <Logo short={mobile} onClick={() => navigate('/map')} />
        <div className="pm-about-header-end">
          {mobile
            ? <Link to="/map" className="pm-btn-ghost pm-pill pm-nowrap">К карте</Link>
            : <button type="button" className="pm-btn pm-pill pm-nowrap" onClick={propose}>Предложить событие</button>}
        </div>
      </div>

      <main className="pm-about-main">
        <section className="pm-about-hero">
          <h1>О проекте</h1>
          <div className="pm-about-hero-grid">
            <div className="pm-about-hero-lead">
              <div className="pm-about-range">IX-XXI</div>
              <div className="pm-label">века на карте</div>
              <p>Интерактивная карта истории России: военные и политические события, у каждого есть место, дата и своя страница.</p>
            </div>

            <div className="pm-about-hero-map">
              <MapView events={events} isActive={always} selectedId={null} onSelect={id => navigate(`/map?event=${id}`)}
                pad={24} labels={false} interactive={false} />
              <div className="pm-about-hero-map-bar">
                <span className="pm-about-chip pm-glass">{events.length} {plural(events.length)} на карте</span>
                <Link to="/map" className="pm-btn pm-pill pm-nowrap">Открыть карту</Link>
              </div>
            </div>

            <div className="pm-about-hero-idea">
              {latest
                ? (
                  <Link to={`/map?event=${latest.id}`} className="pm-about-shot">
                    <Preview e={latest} className="pm-about-shot-img" />
                    <span className="pm-about-chip pm-glass"><b>{yearOf(latest)}</b> {latest.title}</span>
                  </Link>
                )
                : <div className="pm-about-shot"><Preview className="pm-about-shot-img" /></div>}
              <h2>Наша идея</h2>
              <p>Показать историю страны через места, где она происходила. События присылают читатели, модератор проверяет каждое перед публикацией.</p>
            </div>
          </div>
        </section>

        <section className="pm-about-team">
          <div className="pm-about-team-panel">
            {TEAM.map((p, i) => (
              <div key={p.name} className={`pm-about-portrait pm-about-portrait-${i}`}>
                {p.photo
                  ? <img src={p.photo} alt={p.name} />
                  : <span className="pm-about-portrait-initial" aria-hidden="true">{p.initial}</span>}
              </div>
            ))}
            <div className="pm-about-team-center">
              <Tag>Команда</Tag>
              <h2>Люди<br />за картой</h2>
              <div className="pm-about-strip" aria-hidden="true">
                {[0, 1, 2].map(i => <Preview key={i} e={previews[i]} className="pm-about-strip-img" />)}
              </div>
              <p>Карту делают два человека. Менеджер ведёт проект и общается с сообществом, разработчик строит платформу.</p>
            </div>
          </div>
          <div className="pm-about-team-names">
            {TEAM.map((p, i) => (
              <div key={p.name} className={`pm-about-name pm-about-name-${i}`}>
                <h3>{p.name}</h3>
                <div className="pm-about-role">{p.role}</div>
              </div>
            ))}
          </div>
        </section>

        <section className="pm-about-faq">
          <div className="pm-about-faq-side">
            <Tag>FAQ</Tag>
            <h2>Частые вопросы</h2>
            <p>Как устроена карта, как предложить событие и что происходит с заявкой после отправки.</p>
          </div>
          <div className="pm-about-faq-list">
            {FAQ.map((item, i) => {
              const open = openFaq === i
              return (
                <div key={item.q} className={`pm-about-faq-item${open ? ' open' : ''}`}>
                  <button type="button" id={`faq-q-${i}`} aria-expanded={open} aria-controls={`faq-a-${i}`}
                    onClick={() => setOpenFaq(open ? null : i)}>
                    <span className="pm-grow">{item.q}</span>
                    <span className="pm-about-faq-toggle" aria-hidden="true" />
                  </button>
                  {/* Высоту анимирует grid-template-rows 0fr -> 1fr: ответ в DOM всегда, закрытый недоступен через inert.
                      inert через spread: в @types/react 18 его нет, рантайм React 19 его знает. */}
                  <div className="pm-about-faq-body" id={`faq-a-${i}`} role="region" aria-labelledby={`faq-q-${i}`} {...{ inert: !open }}>
                    <div><p>{item.a}</p></div>
                  </div>
                </div>
              )
            })}
          </div>
        </section>
      </main>

      <footer className="pm-about-foot">
        <div className="pm-about-foot-top">
          <nav className="pm-about-socials" aria-label="Связь">
            <a href={TELEGRAM} className="pm-about-sticker fill" style={{ '--r': '-22deg', left: '44%', top: '6%' } as React.CSSProperties}>
              <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M21.4 3.6 2.9 10.7c-1.3.5-1.2 2.3.1 2.6l4.6 1.3 1.8 5.5c.3 1 1.6 1.3 2.3.5l2.6-2.6 4.6 3.4c.8.6 2 .1 2.2-.9L24 5.2c.3-1.2-1.3-2.2-2.6-1.6ZM9.6 14.4l8.6-7.6-6.9 9.1-.4 2.9-1.3-4.4Z" /></svg>
              Telegram
            </a>
            <a href={MAIL} className="pm-about-sticker" style={{ '--r': '5deg', left: '4%', top: '38%' } as React.CSSProperties}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" aria-hidden="true"><rect x="2.5" y="4.5" width="19" height="15" rx="2.5" /><path d="m3.5 6.5 8.5 7 8.5-7" /></svg>
              Почта
            </a>
            <Link to="/map" className="pm-about-sticker fill" style={{ '--r': '-4deg', left: '30%', top: '66%' } as React.CSSProperties}>Карта</Link>
          </nav>
          <div className="pm-about-foot-side">
            <button type="button" className="pm-about-badge" onClick={propose} aria-label="Предложить событие"
              onMouseEnter={e => spinRate(e.currentTarget, 22 / 8)} onMouseLeave={e => spinRate(e.currentTarget, 1)}>
              <svg viewBox="0 0 148 148" aria-hidden="true">
                <defs><path id="pm-badge-path" d="M74 74m-56 0a56 56 0 1 1 112 0a56 56 0 1 1 -112 0" /></defs>
                <text><textPath href="#pm-badge-path" textLength={351} lengthAdjust="spacing">{BADGE_TEXT}</textPath></text>
              </svg>
              <img src="/logo.svg" alt="" />
            </button>
            <span>© 2026<br />Россия - страна Героев</span>
          </div>
        </div>
        <div className="pm-about-bigword" aria-hidden="true">Страна<Star />Героев</div>
      </footer>
    </div>
  )
}
