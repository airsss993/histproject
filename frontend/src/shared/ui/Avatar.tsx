import React from 'react'

// Аватарок пользователи не загружают: рисуем узор по учётке. Один и тот же id - всегда один узор.
// Сетка 5x5, зеркальная по вертикальной оси, цвета из палитры карты.
const TONES = ['#C7AE7F', '#8194AE', '#5F80AA', '#C46F62', '#7D7151', '#A38FB8']

// FNV-1a: коротко и без зависимостей, криптостойкость тут не нужна.
const hash = (seed: string) => {
  let h = 0x811c9dc5
  for (let i = 0; i < seed.length; i++) h = Math.imul(h ^ seed.charCodeAt(i), 0x01000193)
  return h >>> 0
}

export const Avatar: React.FC<{ seed: string | number; size?: number; className?: string }> = ({ seed, size = 48, className }) => {
  const h = hash(String(seed))
  const tone = TONES[h % TONES.length]
  const accent = TONES[(h >> 5) % TONES.length]
  // Каждый бит хеша - клетка левой половины и центра (3 колонки по 5 строк = 15 клеток).
  const bits = h ^ (h >> 13) ^ Math.imul(h, 2654435761)
  const cells = []
  for (let col = 0; col < 3; col++) {
    for (let row = 0; row < 5; row++) {
      if (!((bits >> (col * 5 + row)) & 1)) continue
      const fill = (col + row) % 3 === 0 ? accent : tone
      cells.push(<rect key={`${col}-${row}`} x={col * 20 + 3} y={row * 20 + 3} width="14" height="14" rx="4" fill={fill} />)
      if (col < 2) cells.push(<rect key={`m${col}-${row}`} x={(4 - col) * 20 + 3} y={row * 20 + 3} width="14" height="14" rx="4" fill={fill} />)
    }
  }
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 100 100" aria-hidden="true">
      <rect width="100" height="100" rx="26" fill="#0C1023" />
      <rect width="100" height="100" rx="26" fill={tone} opacity="0.1" />
      {cells}
    </svg>
  )
}
