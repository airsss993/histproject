import React from 'react'
import { Moderator, RoleType, ROLE_LABEL } from './modcard'
import { DropDownRole } from './dropdownrole'

interface ModeRoleProps {
  changeRoleMod: Moderator
  changeRoleClosing: boolean
  selectedRole: RoleType
  roleDropdownOpen: boolean
  onDropdownToggle: () => void
  onRoleSelect: (role: RoleType) => void
  onCancel: () => void
  onSave: () => void
}

export const ModeRole: React.FC<ModeRoleProps> = ({
  changeRoleMod,
  changeRoleClosing,
  selectedRole,
  roleDropdownOpen,
  onDropdownToggle,
  onRoleSelect,
  onCancel,
  onSave,
}) => {
  return (
    <div className={`role-modal__overlay${changeRoleClosing ? ' role-modal__overlay--closing' : ''}`} onMouseDown={e => e.stopPropagation()}>
      <div className={`role-modal${changeRoleClosing ? ' role-modal--closing' : ''}`} onMouseDown={e => e.stopPropagation()}>
        <h2 className="role-modal__title">Изменить роль</h2>
        <p className="role-modal__subtitle">
          Изменить роль модератора <strong>{changeRoleMod.login}</strong>
        </p>
        <label className="role-modal__label">Новая роль</label>
        <div className="role-modal__select-wrap" onClick={onDropdownToggle}>
          <svg xmlns="http://www.w3.org/2000/svg" className="role-modal__select-icon" width="14" height="14" viewBox="0 0 14 14" fill="none">
            <path d="M11.6663 7.58331C11.6663 10.5 9.62467 11.9583 7.19801 12.8041C7.07094 12.8472 6.9329 12.8451 6.80717 12.7983C4.37467 11.9583 2.33301 10.5 2.33301 7.58331V3.49998C2.33301 3.34527 2.39447 3.19689 2.50386 3.0875C2.61326 2.9781 2.76163 2.91664 2.91634 2.91664C4.08301 2.91664 5.54134 2.21664 6.55634 1.32998C6.67992 1.22439 6.83713 1.16638 6.99967 1.16638C7.16222 1.16638 7.31943 1.22439 7.44301 1.32998C8.46384 2.22248 9.91634 2.91664 11.083 2.91664C11.2377 2.91664 11.3861 2.9781 11.4955 3.0875C11.6049 3.19689 11.6663 3.44527 11.6663 3.49998V7.58331Z" stroke="black" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
          <span className="role-modal__select-label">{ROLE_LABEL[selectedRole]}</span>
          <svg className={`role-modal__chevron${roleDropdownOpen ? ' role-modal__chevron--open' : ''}`} width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M6 9L12 15L18 9" stroke="#555" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
          {roleDropdownOpen && (
            <DropDownRole selectedRole={selectedRole} onRoleSelect={onRoleSelect} />
          )}
        </div>
        <div className="role-modal__actions">
          <button className="role-modal__cancel" onClick={onCancel}>
            Отмена
          </button>
          <button className="role-modal__save" onClick={onSave}>
            Сохранить
          </button>
        </div>
      </div>
    </div>
  )
}
