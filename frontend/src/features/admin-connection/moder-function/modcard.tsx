import React from 'react'
import { rolechange as ChangeRoleMenu } from './changerole'
import { svgrolog } from './svgrolog'

export type RoleType = 'super_admin' | 'admin'

export interface Moderator {
  id: string
  email: string
  login: string
  role: RoleType
  createdAt: string
}

export const ROLE_LABEL: Record<RoleType, string> = {
  super_admin: 'Супер админ',
  admin: 'Модератор',
}

interface ModerCardProps {
  mod: Moderator
  copied: string | null
  openMenuId: string | null
  menuClosing: string | null
  menuRef: React.RefObject<HTMLDivElement | null>
  onCopy: (login: string) => void
  onMenuToggle: (id: string) => void
  onMenuClose: (id: string) => void
  onChangeRole: (mod: Moderator) => void
  onDelete: (mod: Moderator) => void
}

export const forMoDate = (dateString: string): string => {
  const date = new Date(dateString)
  if (isNaN(date.getTime())) return dateString
  const datePart = date.toLocaleDateString('ru-RU', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  }).replace(' г.', '')
  return `${datePart}`
}

export const ModerCard: React.FC<ModerCardProps> = ({
  mod,
  copied,
  openMenuId,
  menuClosing,
  menuRef,
  onCopy,
  onMenuToggle,
  onMenuClose,
  onChangeRole,
  onDelete,
}) => (
  <div className="moder-table__row">
    <span className="moder-table__id">{mod.id}</span>
    <span className="moder-table__email">{mod.email}</span>
    <div className="moder-table__login">
      <span>{mod.login}</span>
      <button
        className="moder-table__copy-btn"
        onClick={() => onCopy(mod.login)}
        title="Скопировать логин"
      >
        {svgrolog[copied === mod.login ? 1 : 0]}
      </button>
    </div>
    <div className={`moder-table__role moder-table__role--${mod.role}`}>
      {mod.role === 'super_admin' ? (
        <svg xmlns="http://www.w3.org/2000/svg" width="11" height="14" viewBox="0 0 11 14" fill="none">
          <path d="M10.0833 7.16693C10.0833 10.0836 8.04167 11.5419 5.615 12.3878C5.48793 12.4308 5.3499 12.4288 5.22417 12.3819C2.79167 11.5419 0.75 10.0836 0.75 7.16693V3.0836C0.75 2.92889 0.811458 2.78051 0.920854 2.67112C1.03025 2.56172 1.17862 2.50026 1.33333 2.50026C2.5 2.50026 3.95833 1.80026 4.97333 0.913596C5.09692 0.808012 5.25412 0.75 5.41667 0.75C5.57921 0.75 5.73642 0.808012 5.86 0.913596C6.88083 1.8061 8.33333 2.50026 9.5 2.50026C9.65471 2.50026 9.80308 2.56172 9.91248 2.67112C10.0219 2.78051 10.0833 2.92889 10.0833 3.0836V7.16693Z" stroke="#218ED3" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
          <path d="M3.66699 6.5836L4.83366 7.75026L7.16699 5.41693" stroke="#218ED3" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
      ) : (
        <svg xmlns="http://www.w3.org/2000/svg" width="11" height="14" viewBox="0 0 11 14" fill="none">
          <path d="M10.0833 7.16693C10.0833 10.0836 8.04167 11.5419 5.615 12.3878C5.48793 12.4308 5.3499 12.4288 5.22417 12.3819C2.79167 11.5419 0.75 10.0836 0.75 7.16693V3.0836C0.75 2.92889 0.811458 2.78051 0.920854 2.67112C1.03025 2.56172 1.17862 2.50026 1.33333 2.50026C2.5 2.50026 3.95833 1.80026 4.97333 0.913596C5.09692 0.808012 5.25412 0.75 5.41667 0.75C5.57921 0.75 5.73642 0.808012 5.86 0.913596C6.88083 1.8061 8.33333 2.50026 9.5 2.50026C9.65471 2.50026 9.80308 2.56172 9.91248 2.67112C10.0219 2.78051 10.0833 2.92889 10.0833 3.0836V7.16693Z" stroke="#008D00" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
      )}
      {ROLE_LABEL[mod.role]}
    </div>
    <span className="moder-table__date">{forMoDate(mod.createdAt)}</span>
    <div
      className="moder-table__actions" // @ts-ignore
      ref={openMenuId === mod.id ? menuRef : null}
    >
      <button
        className="moder-table__dots-btn"
        onClick={() => openMenuId === mod.id ? onMenuClose(mod.id) : onMenuToggle(mod.id)}
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <circle cx="5" cy="12" r="1.5" fill="#555"/>
          <circle cx="12" cy="12" r="1.5" fill="#555"/>
          <circle cx="19" cy="12" r="1.5" fill="#555"/>
        </svg>
      </button>
      {(openMenuId === mod.id || menuClosing === mod.id) && (
        <ChangeRoleMenu mod={mod} menuClosing={menuClosing} onChangeRole={onChangeRole} onDelete={onDelete} />
      )}
    </div>
  </div>
)
