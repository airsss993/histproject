import React, { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Logo, useIsMobile } from './PublicMap.tsx'
import { NotFound } from './NotFound.tsx'
import { fetchMe, profilePath, type User } from './api.ts'
import privacy from './legal/privacy.md?raw'
import rules from './legal/rules.md?raw'
import registration from './legal/registration.md?raw'
import submission from './legal/submission.md?raw'
import distribution from './legal/distribution.md?raw'
import './public.css'
import './about.css'
import './guide.css'
import './legal.css'

// Публичные документы по персональным данным. Тексты в legal/*.md, внутренние документы - в docs/legal.
// Поменялся текст согласия - новая дата здесь и версия согласия на бэкенде (users/service.go, submissions).
const EDITION = '28.09.2026'
const DOCS: Record<string, { title: string; tag: string; text: string }> = {
  '/privacy': { title: 'Политика обработки персональных данных', tag: 'Политика', text: privacy },
  '/rules': { title: 'Правила публикации материалов о третьих лицах', tag: 'Правила', text: rules },
  '/consent/registration': { title: 'Согласие на обработку данных при регистрации', tag: 'Согласие', text: registration },
  '/consent/submission': { title: 'Согласие на обработку данных при отправке заявки', tag: 'Согласие', text: submission },
  '/consent/distribution': { title: 'Согласие на распространение персональных данных', tag: 'Согласие', text: distribution },
}

// Разметка текстов - узкое подмножество Markdown: ## и ### заголовки, абзацы, списки "- ", таблицы, **жирный**, [ссылка](адрес).
const inline = (text: string): React.ReactNode[] =>
  text.split(/(\*\*[^*]+\*\*|\[[^\]]+\]\([^)]+\))/).map((part, i) => {
    const bold = part.match(/^\*\*(.+)\*\*$/)
    if (bold) return <b key={i}>{bold[1]}</b>
    const link = part.match(/^\[(.+)\]\((.+)\)$/)
    if (link) return link[2].startsWith('/') ? <Link key={i} to={link[2]}>{link[1]}</Link> : <a key={i} href={link[2]}>{link[1]}</a>
    return part
  })

const cells = (row: string) => row.trim().replace(/^\||\|$/g, '').split('|').map(c => c.trim())
const anchor = (title: string) => `s-${title.match(/^[\d.]+/)?.[0].replace(/\.$/, '') ?? title}`

const Blocks: React.FC<{ text: string }> = ({ text }) => (
  <>
    {text.trim().split(/\n\s*\n/).map((block, i) => {
      const lines = block.split('\n')
      if (block.startsWith('### ')) return <h3 key={i}>{block.slice(4)}</h3>
      if (block.startsWith('## ')) return <h2 key={i} id={anchor(block.slice(3))}>{block.slice(3)}</h2>
      if (block.startsWith('|')) {
        const [head, , ...rows] = lines.map(cells)
        return (
          <table key={i} className="pm-legal-table">
            <thead><tr>{head.map(c => <th key={c}>{c}</th>)}</tr></thead>
            <tbody>
              {rows.map((row, r) => (
                <tr key={r}>{row.map((c, k) => <td key={k} data-label={head[k]}>{inline(c)}</td>)}</tr>
              ))}
            </tbody>
          </table>
        )
      }
      if (block.startsWith('- ')) return <ul key={i} className="pm-guide-list">{lines.map(l => <li key={l}>{inline(l.slice(2))}</li>)}</ul>
      return <p key={i}>{inline(lines.map(l => l.trim()).join(' '))}</p>
    })}
  </>
)

export const Legal: React.FC<{ path: string }> = ({ path }) => {
  const navigate = useNavigate()
  const mobile = useIsMobile()
  const [me, setMe] = useState<User | null>(null)
  const doc = DOCS[path]

  // В согласие на распространение подставляются данные из учётной записи, гостю - пояснение.
  useEffect(() => {
    if (path !== '/consent/distribution') return
    const ac = new AbortController()
    fetchMe(ac.signal).then(setMe, () => {})
    return () => ac.abort()
  }, [path])

  if (!doc) return <NotFound />
  const text = doc.text
    .replace('{name}', me ? `${me.firstName} ${me.lastName}` : '(имя и фамилия из учётной записи)')
    .replace('{email}', me ? me.email : '(email учётной записи)')
    .replace('{profile}', me ? `https://russia-heroes.ru${profilePath(me.publicId)}` : '(адрес профиля на Сайте)')
  const toc = [...text.matchAll(/^## (.+)$/gm)].map(m => m[1])

  return (
    <div className="pm-root pm-about pm-guide pm-legal">
      <div role="banner" className="pm-header pm-about-header">
        <Logo short={mobile} onClick={() => navigate('/map')} />
        <div className="pm-about-header-end">
          <Link to="/map" className="pm-btn-ghost pm-pill pm-nowrap">К карте</Link>
        </div>
      </div>

      <main className="pm-guide-main pm-legal-main">
        <section className="pm-guide-hero">
          <div className="pm-about-tag"><span>{doc.tag}</span><i /></div>
          <h1>{doc.title}</h1>
          <div className="pm-guide-meta"><span>Редакция от {EDITION}</span></div>
        </section>

        <div className={toc.length > 2 ? 'pm-guide-body' : 'pm-legal-single'}>
          {toc.length > 2 && (
            <nav className="pm-guide-toc" aria-label="Разделы">
              <span className="pm-label">Разделы</span>
              <ol>
                {toc.map(t => (
                  <li key={t}>
                    <a href={`#${anchor(t)}`} onClick={e => { e.preventDefault(); document.getElementById(anchor(t))?.scrollIntoView({ behavior: 'smooth' }) }}>
                      {t.replace(/^[\d.]+\s*/, '')}
                    </a>
                  </li>
                ))}
              </ol>
            </nav>
          )}
          <article className="pm-legal-text"><Blocks text={text} /></article>
        </div>

        <nav className="pm-legal-docs" aria-label="Документы">
          <span className="pm-label">Другие документы</span>
          {Object.entries(DOCS).filter(([p]) => p !== path).map(([p, d]) => <Link key={p} to={p}>{d.title}</Link>)}
        </nav>
      </main>
    </div>
  )
}
