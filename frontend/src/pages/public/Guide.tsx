import React, { useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Logo, useIsMobile } from './PublicMap.tsx'
import './public.css'
import './about.css'
import './guide.css'

// Инструкция для тех, у кого нет сайта события. Требования к архиву сверены с формой заявки (Propose.tsx)
// и проверкой на сервере (backend/internal/modules/sites/archive.go, CSP sandbox в Caddyfile).
const READ_MIN = 12

type Block =
  | { p: string }
  | { list: string[] }
  | { prompt: string; label?: string }
  | { pair: [string, string]; labels?: [string, string] }
  | { chain: string[] }

const STEPS: { title: string; short: string; blocks: Block[] }[] = [
  {
    title: 'Выберите вопрос, а не тему',
    short: 'Вопрос',
    blocks: [
      { p: 'Событие на карте - это точка и дата, поэтому сайт отвечает на один вопрос о нём и не пересказывает всю эпоху. "Бородино" - тема, про неё пишут книги. "Почему Кутузов решил дать сражение у Бородино?" - вопрос, на который можно ответить на одной странице.' },
      { chain: ['кто или что', 'где', 'когда', 'почему или как'] },
      { p: 'Проверьте свой вопрос по этой формуле. Если в нём нет места или времени, он почти наверняка слишком широкий. Запишите вопрос: ответ на него станет последним разделом сайта.' },
      { pair: ['Отмена крепостного права', 'Как Манифест 1861 года изменил жизнь крестьян одной губернии?'], labels: ['Тема', 'Вопрос'] },
    ],
  },
  {
    title: 'Найдите факты и источники',
    short: 'Источники',
    blocks: [
      { p: 'AI не является историческим источником. Он уверенно ошибается в датах, путает имена и может сослаться на книгу, которой не существует. Используйте его, чтобы придумать поисковые запросы или упростить сложный текст, но каждый факт на сайте подтверждайте тем, что существует отдельно от AI.' },
      { list: [
        'Хорошие источники: музеи, архивы, библиотеки, энциклопедии, университеты, научные статьи, книги, цифровые коллекции документов и газет.',
        'Осторожно: Википедия (как старт, идите по ссылкам внизу статьи), научно-популярные статьи с автором.',
        'Не годятся: ролики из соцсетей, посты без автора, картинки из поиска без происхождения, ответ AI.',
      ] },
      { p: 'Минимум - три хороших источника. Каждую ключевую дату, имя и цифру найдите хотя бы в двух независимых местах. Если у вас есть первичный источник - документ, газета, письмо, фотография эпохи, - сайт сразу становится сильнее.' },
      { p: 'Источник записывайте так, чтобы его мог найти кто угодно: автор или организация, название, ссылка. Государственный Эрмитаж. Коллекция гравюр XVIII века. Адрес страницы.' },
    ],
  },
  {
    title: 'Соберите структуру на бумаге',
    short: 'Структура',
    blocks: [
      { p: 'До первой строчки кода решите, в каком порядке человек узнаёт историю. Рабочая схема из 4-6 разделов:' },
      { chain: ['первый экран: вопрос, место, дата', 'зачем это важно', 'контекст', '2-3 главных блока', 'вывод', 'источники'] },
      { p: 'Для каждого раздела напишите одно предложение: о чём он. Если два раздела получаются про одно и то же, объедините их. Если раздел не помогает ответить на вопрос, уберите его. Вывод начинается с фразы "На основании изученных источников я прихожу к выводу, что..." и отвечает прямо на ваш вопрос.' },
    ],
  },
  {
    title: 'Придумайте, как сайт будет выглядеть',
    short: 'Облик',
    blocks: [
      { p: 'Шаблона не будет, и это нарочно. Если попросить AI "сделай сайт про Бородино", он выдаст то же, что всем: огромный заголовок на градиенте, три одинаковые карточки и подвал. Такие сайты модератор видит десятками, и на карте они сливаются в один.' },
      { p: 'Идите от самого события. Какого цвета мундиры, бумага газеты тех лет, карта кампании? Каким шрифтом набран документ эпохи? Выпишите три-четыре слова о настроении, два-три цвета и один-два шрифта. Это ваше задание на дизайн.' },
      { p: 'AI здесь полезен как собеседник, а не исполнитель. Попросите у него несколько разных вариантов и возьмите тот, что ближе вам, или смешайте их:' },
      { prompt: 'Мой сайт отвечает на вопрос: почему Кутузов решил дать сражение у Бородино. Предложи три непохожих варианта оформления: цвета с HEX-кодами, шрифты из Google Fonts, как устроен первый экран. Не предлагай градиенты и одинаковые карточки. Код пока не пиши.' },
    ],
  },
  {
    title: 'Сделайте первую версию с AI',
    short: 'Первая версия',
    blocks: [
      { p: 'Понадобятся браузер, любой чат с AI и текстовый редактор: Блокнот подойдёт, VS Code удобнее. Создайте папку сайта, а в ней файл index.html. Включите в системе показ расширений файлов, иначе легко получить index.html.txt, который браузер откроет как текст.' },
      { p: 'Чем больше AI знает о задаче, тем меньше он додумывает сам. Дайте ему всё, что вы уже решили:' },
      { prompt: 'Сделай одну страницу index.html. Вопрос сайта: [ваш вопрос]. Разделы: [список из шага 3]. Факты для каждого раздела: [ваши заметки с источниками]. Оформление: [цвета и шрифты из шага 4]. Стили и скрипты внутри файла, без сборщиков и серверного кода. Страница должна открываться двойным щелчком и нормально читаться на телефоне. Не добавляй фактов, которых нет в моём списке.' },
      { p: 'Сохраните ответ в index.html и откройте файл в браузере. Перед каждой крупной правкой копируйте папку сайта рядом, с датой в названии. Если AI что-то сломает, вы вернётесь к рабочей копии за минуту.' },
    ],
  },
  {
    title: 'Правьте точечно и проверяйте за AI',
    short: 'Правки',
    blocks: [
      { p: 'Дальше просите по одной правке за раз. Хороший запрос отвечает на четыре вопроса: что у меня сейчас, что изменить, что не трогать, что показать в ответе.' },
      { pair: ['Сделай первый экран красивее.', 'У меня есть блок первого экрана. Увеличь отступ сверху и снизу. Текст, цвета и шрифты не трогай. Покажи только изменённый кусок кода.'], labels: ['Плохо', 'Хорошо'] },
      { p: 'После каждой правки сверяйте даты, имена и цифры со своими заметками: переписывая страницу, AI иногда меняет и факты. Научитесь сами менять заголовок, дату, цвет и подпись: найдите нужное место поиском по файлу (Ctrl+F) и поправьте руками. Если AI не может починить ошибку с третьей попытки, вернитесь к копии и попросите по-другому.' },
      { p: 'Хорошо работает и обратное: попросите AI покритиковать готовую страницу. Где текст плохо читается, где мало контраста, что ломается на узком экране, какие фразы ничего не сообщают. Решать, что из этого исправлять, будете вы.' },
    ],
  },
  {
    title: 'Напишите текст и подберите изображения',
    short: 'Текст и картинки',
    blocks: [
      { p: 'Первый вариант текста пишите сами. AI может потом сократить его или поправить стиль, но не добавлять новых фактов. Сравните:' },
      { pair: [
        'Важно отметить, что путешествие Петра I в Европу сыграло огромную роль в истории России.',
        'В 1697 году Пётр I поехал в Европу под именем урядника Петра Михайлова и работал плотником на верфи в Голландии.',
      ], labels: ['Вода', 'Конкретика'] },
      { p: 'Изображение должно помогать понять событие, а не украшать страницу. Для каждой картинки ответьте одной фразой: что она объясняет. Нет ответа - картинка не нужна. Берите изображения из Wikimedia Commons, музейных, архивных и библиотечных коллекций, подписывайте каждое: что изображено и откуда взято. Картинку, созданную AI, не выдавайте за историческое фото, подпишите её как иллюстрацию.' },
      { p: 'Уменьшите фотографии примерно до 1600 пикселей по ширине. Видео в архив не кладите: весь сайт должен уложиться в 20 МБ.' },
    ],
  },
  {
    title: 'Проверьте и соберите архив',
    short: 'Архив',
    blocks: [
      { p: 'Откройте сайт в режиме телефона: в Chrome F12, затем Ctrl+Shift+M. Страница не должна прокручиваться вбок, текст читается без увеличения, картинки помещаются в экран. Потом дайте сайт прочитать кому-нибудь и попросите замечания по формуле: где проблема, что непонятно, что попробовать.' },
      { p: 'Сайт публикуется как есть, из вашего архива. Сервер принимает не всё:' },
      { list: [
        'ZIP, не RAR и не 7z, размером до 20 МБ.',
        'index.html лежит в корне архива. Выделите файлы внутри папки сайта и сожмите их, а не саму папку.',
        'Пути к файлам относительные: images/map.jpg, а не C:\\Users\\... и не file:///.',
        'Имена файлов латиницей, без пробелов. Регистр важен: Photo.JPG и photo.jpg для сервера разные файлы.',
        'Только статичные файлы: HTML, CSS, JavaScript, изображения, шрифты. PHP, Python, exe и другие исполняемые файлы сервер отклонит.',
        'Сайт открывается в изолированном режиме: формы обратной связи, счётчики и сохранение данных в браузере работать не будут.',
      ] },
      { p: 'Последняя проверка: распакуйте архив в новую пустую папку и откройте index.html оттуда. Если всё на месте, архив собран правильно.' },
    ],
  },
]

const CHECKS = [
  'Сайт отвечает на один вопрос, в нём есть место и время',
  'Есть раздел "Вывод" с прямым ответом на вопрос',
  'Не меньше трёх хороших источников, AI среди них нет',
  'Даты, имена и цифры сверены с источниками после последней правки AI',
  'У каждого изображения есть подпись с источником',
  'Оформление придумано под событие, а не взято у AI по умолчанию',
  'На телефоне нет прокрутки вбок, текст читается',
  'ZIP до 20 МБ, index.html в корне, имена файлов латиницей',
  'Распакованный архив открывается из новой папки',
]

const WHAT = [
  ['Один вопрос', 'о конкретном событии с местом и датой'],
  ['4-6 разделов', 'от первого экрана до вывода'],
  ['3+ источника', 'проверяемых, без ответов AI'],
  ['Свой облик', 'под эпоху события, без шаблона'],
]

const Tag: React.FC<{ children: string }> = ({ children }) => (
  <div className="pm-about-tag"><span>{children}</span><i /></div>
)

const Check: React.FC = () => (
  <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M5 12.5l4.5 4.5L19 7.5" /></svg>
)

const BlockView: React.FC<{ b: Block }> = ({ b }) => {
  if ('p' in b) return <p>{b.p}</p>
  if ('list' in b) return <ul className="pm-guide-list">{b.list.map(t => <li key={t}>{t}</li>)}</ul>
  if ('prompt' in b) return (
    <div className="pm-guide-prompt">
      <span className="pm-label">{b.label ?? 'Запрос к AI'}</span>
      <p>{b.prompt}</p>
    </div>
  )
  if ('chain' in b) return (
    <div className="pm-guide-chain">
      {b.chain.map((t, i) => <React.Fragment key={t}>{i > 0 && <i aria-hidden="true" />}<span>{t}</span></React.Fragment>)}
    </div>
  )
  const [bad, good] = b.pair
  const [lb, lg] = b.labels ?? ['Было', 'Стало']
  return (
    <div className="pm-guide-pair">
      <div className="bad"><span className="pm-label">{lb}</span><p>{bad}</p></div>
      <div className="good"><span className="pm-label">{lg}</span><p>{good}</p></div>
    </div>
  )
}

export const Guide: React.FC = () => {
  const navigate = useNavigate()
  const mobile = useIsMobile()
  const rootRef = useRef<HTMLDivElement>(null)
  const [progress, setProgress] = useState(0)
  const [active, setActive] = useState(-1)
  const [done, setDone] = useState<boolean[]>(() => CHECKS.map(() => false))
  const propose = () => navigate('/map?propose=1')

  // Прокручивается корень страницы (.pm-about), а не window.
  useEffect(() => {
    const root = rootRef.current
    if (!root) return
    const onScroll = () => {
      const max = root.scrollHeight - root.clientHeight
      setProgress(max > 0 ? Math.min(1, root.scrollTop / max) : 0)
      let cur = -1
      root.querySelectorAll<HTMLElement>('.pm-guide-step').forEach((el, i) => {
        if (el.getBoundingClientRect().top < root.clientHeight * 0.4) cur = i
      })
      setActive(cur)
    }
    onScroll()
    root.addEventListener('scroll', onScroll, { passive: true })
    return () => root.removeEventListener('scroll', onScroll)
  }, [])

  const jump = (e: React.MouseEvent, id: string) => {
    e.preventDefault()
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
  const left = Math.max(1, Math.round(READ_MIN * (1 - progress)))
  const checked = done.filter(Boolean).length

  return (
    <div ref={rootRef} className="pm-root pm-about pm-guide">
      <div className="pm-guide-progress" aria-hidden="true"><i style={{ transform: `scaleX(${progress})` }} /></div>
      <div role="banner" className="pm-header pm-about-header">
        <Logo short={mobile} onClick={() => navigate('/map')} />
        <div className="pm-about-header-end">
          {mobile
            ? <Link to="/map" className="pm-btn-ghost pm-pill pm-nowrap">К карте</Link>
            : <button type="button" className="pm-btn-light pm-pill pm-nowrap" onClick={propose}>Предложить событие</button>}
        </div>
      </div>

      <main className="pm-guide-main">
        <section className="pm-guide-hero">
          <Tag>Инструкция</Tag>
          <h1>Как сделать сайт события</h1>
          <p className="pm-guide-lead">
            Каждое событие на карте открывается своей страницей, и её делает автор заявки. Опыт в программировании не нужен:
            код поможет написать AI. Решать, о чём сайт, проверять факты и выбирать, как он выглядит, будете вы.
          </p>
          <div className="pm-guide-meta">
            <span>{READ_MIN} минут чтения</span>
            <span>8 шагов</span>
            <span>браузер, редактор и чат с AI</span>
          </div>
        </section>

        <section className="pm-guide-what">
          <div className="pm-guide-what-side">
            <h2>Какой сайт мы ждём</h2>
            <p>
              Готового шаблона нет. Если у всех один макет, на карте получается сотня одинаковых страниц, и ни одна
              не запоминается. Вместо шаблона четыре требования, остальное решаете сами.
            </p>
          </div>
          <div className="pm-guide-what-grid">
            {WHAT.map(([b, s]) => (
              <div key={b} className="pm-guide-what-item"><b>{b}</b><span>{s}</span></div>
            ))}
          </div>
        </section>

        <div className="pm-guide-body">
          <nav className="pm-guide-toc" aria-label="Шаги">
            <span className="pm-label">Шаги</span>
            <ol>
              {STEPS.map((s, i) => (
                <li key={s.title}>
                  <a href={`#step-${i + 1}`} className={i === active ? 'on' : ''} onClick={e => jump(e, `step-${i + 1}`)}>
                    <b>{String(i + 1).padStart(2, '0')}</b>{s.short}
                  </a>
                </li>
              ))}
              <li><a href="#check" className={active >= STEPS.length ? 'on' : ''} onClick={e => jump(e, 'check')}><b><Check /></b>Перед отправкой</a></li>
            </ol>
            <span className="pm-guide-left">{progress > 0.98 ? 'Дочитано' : `Осталось около ${left} мин`}</span>
          </nav>

          <div className="pm-guide-steps">
            {STEPS.map((s, i) => (
              <section key={s.title} id={`step-${i + 1}`} className={`pm-guide-step${i <= active ? ' passed' : ''}`}>
                <span className="pm-guide-num" aria-hidden="true">{String(i + 1).padStart(2, '0')}</span>
                <div className="pm-guide-step-body">
                  <span className="pm-label">Шаг {i + 1} из {STEPS.length}</span>
                  <h2>{s.title}</h2>
                  {s.blocks.map((b, j) => <BlockView key={j} b={b} />)}
                </div>
              </section>
            ))}

            <section id="check" className="pm-guide-step pm-guide-check">
              <span className="pm-guide-num" aria-hidden="true"><Check /></span>
              <div className="pm-guide-step-body">
                <span className="pm-label">Перед отправкой · {checked} из {CHECKS.length}</span>
                <h2>Проверьте себя</h2>
                <ul className="pm-guide-checks">
                  {CHECKS.map((c, i) => (
                    <li key={c}>
                      <label>
                        <input type="checkbox" id={`guide-check-${i}`} checked={done[i]}
                          onChange={() => setDone(d => d.map((v, k) => (k === i ? !v : v)))} />
                        <span>{c}</span>
                      </label>
                    </li>
                  ))}
                </ul>
                <p>
                  После отправки заявку проверит модератор. Письмо придёт на почту аккаунта: событие опубликовано со
                  ссылкой или отклонено с комментарием, что исправить. Исправленный сайт можно отправить заново.
                </p>
              </div>
            </section>
          </div>
        </div>

        <section className="pm-guide-cta">
          <h2>Сайт готов?</h2>
          <p>Приложите ZIP-архив к заявке, поставьте точку на карте и отправьте на проверку.</p>
          <button type="button" className="pm-btn-light pm-pill" onClick={propose}>Предложить событие</button>
        </section>
      </main>
    </div>
  )
}
