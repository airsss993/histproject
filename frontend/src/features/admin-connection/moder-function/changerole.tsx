import React from 'react'
import { Moderator } from './modcard'

interface ChangeRoleMenuProps {
  mod: Moderator
  menuClosing: string | null
  onChangeRole: (mod: Moderator) => void
  onDelete: (mod: Moderator) => void
}

export const rolechange: React.FC<ChangeRoleMenuProps> = ({ mod, menuClosing, onChangeRole, onDelete }) => (
  <div className={`moder-table__menu${menuClosing === mod.id ? ' moder-table__menu--closing' : ''}`}>
    <button className="moder-table__menu-item" onClick={() => onChangeRole(mod)}>
      <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none">
        <path d="M10 15H6C4.93913 15 3.92172 15.4214 3.17157 16.1716C2.42143 16.9217 2 17.9391 2 19V21M14.305 16.53L15.228 16.148M15.228 13.852L14.305 13.469M16.852 12.228L16.469 11.305M16.852 17.772L16.469 18.696M19.148 12.228L19.531 11.305M19.53 18.696L19.148 17.772M20.772 13.852L21.696 13.469M20.772 16.148L21.696 16.531" stroke="#555555" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
        <path d="M18 18C19.6569 18 21 16.6569 21 15C21 13.3431 19.6569 12 18 12C16.3431 12 15 13.3431 15 15C15 16.6569 16.3431 18 18 18Z" stroke="#555555" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
        <path d="M9 11C11.2091 11 13 9.20914 13 7C13 4.79086 11.2091 3 9 3C6.79086 3 5 4.79086 5 7C5 9.20914 6.79086 11 9 11Z" stroke="#555555" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
      </svg>
      Изменить роль
    </button>
    <button className="moder-table__menu-item moder-table__menu-item--danger" onClick={() => onDelete(mod)}>
      <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none">
        <path d="M14 11V17M10 11V17M6 7V19C6 19.5304 6.21071 20.0391 6.58579 20.4142C6.96086 20.7893 7.46957 21 8 21H16C16.5304 21 17.0391 20.7893 17.4142 20.4142C17.7893 20.0391 18 19.5304 18 19V7M4 7H20M7 7L9 3H15L17 7" stroke="#555555" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
      </svg>
      Удалить
    </button>
  </div>
)
