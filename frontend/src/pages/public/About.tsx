import React from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Logo, useIsMobile } from './PublicMap.tsx'
import './public.css'
import './about.css'

// Команда и контакты - со страницы "О нас" trudovaya-slava.
const TEAM = [
  {
    initial: 'Д',
    role: 'Менеджер проекта',
    name: 'Дарья Склярова',
    bio: 'Отвечает за развитие проекта, координацию команды и работу с сообществом.',
    contacts: [
      { label: '@dsxwwx_z', href: 'https://t.me/dsxwwx_z' },
      { label: 'doctorin7@gmail.com', href: 'mailto:doctorin7@gmail.com' },
    ],
  },
  {
    initial: 'А',
    role: 'Разработчик',
    name: 'Артём Джапаридзе',
    bio: 'Проектирует и разрабатывает платформу.',
    contacts: [],
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
    a: 'Напишите нам в Telegram или на почту из блока "Команда проекта". Укажите событие и приложите источник, если он есть.',
  },
]

export const About: React.FC = () => {
  const navigate = useNavigate()
  const mobile = useIsMobile()
  return (
    <div className="pm-root pm-about">
      <div role="banner" className="pm-header pm-about-header">
        <Logo short={mobile} onClick={() => navigate('/map')} />
        <div className="pm-grow" />
        <Link to="/map" className="pm-btn-ghost pm-pill pm-nowrap">К карте</Link>
      </div>

      <main className="pm-about-main">
        <section className="pm-about-hero">
          <div className="pm-about-intro">
            <div className="pm-label">Открытый исторический проект · 2026</div>
            <h1>О проекте</h1>
            <p>
              "Россия - страна Героев" - интерактивная карта событий истории России. Карту ведёт
              независимая команда из двух человек, события присылают читатели.
            </p>
            <p>
              Если знаете событие, которого нет на карте, - предложите его. Нашли ошибку - напишите нам.
            </p>
            <div className="pm-row">
              <button type="button" className="pm-btn" onClick={() => navigate('/map?propose=1')}>Предложить событие</button>
              <a href="#faq" className="pm-btn-ghost">Частые вопросы</a>
            </div>
          </div>

          <aside className="pm-about-team">
            <div className="pm-label">Над проектом работают</div>
            <h2>Команда проекта</h2>
            {TEAM.map(p => (
              <div key={p.name} className="pm-about-person">
                <div className="pm-about-avatar" aria-hidden="true">{p.initial}</div>
                <div className="pm-grow">
                  <div className="pm-about-role">{p.role}</div>
                  <h3>{p.name}</h3>
                  <p>{p.bio}</p>
                  {p.contacts.length > 0 && (
                    <div className="pm-about-contacts">
                      {p.contacts.map(c => <a key={c.href} className="pm-link" href={c.href}>{c.label}</a>)}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </aside>
        </section>

        <section className="pm-about-faq" id="faq">
          <div className="pm-about-faq-side">
            <div className="pm-about-faq-tag"><span>FAQ</span><i /></div>
            <h2>Частые вопросы</h2>
            <p>Как устроена карта, как предложить событие и что происходит с заявкой после отправки.</p>
          </div>
          <div className="pm-about-faq-list">
            {FAQ.map((item, i) => (
              <details key={item.q} name="faq" open={i === 0}>
                <summary>
                  <span className="pm-grow">{item.q}</span>
                  <span className="pm-about-faq-toggle" aria-hidden="true" />
                </summary>
                <p>{item.a}</p>
              </details>
            ))}
          </div>
        </section>
      </main>

      <footer className="pm-about-foot">
        <span>© 2026 · Россия - страна Героев</span>
        <nav>
          <Link to="/map" className="pm-link">Карта</Link>
          <Link to="/map?propose=1" className="pm-link">Предложить событие</Link>
        </nav>
      </footer>
    </div>
  )
}
