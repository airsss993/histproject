import React, { useState, useEffect, useRef } from 'react'
import { ModerCard, Moderator, RoleType, ROLE_LABEL } from '../admin-connection/moder-function/modcard'
import { fetchModerators, createAdmin, deleteAdmin, changeAdminRole, CreateAdminResult } from '../admin-connection/moder-function/modreques'
import { ModeRole } from '../admin-connection/moder-function/moderole'
import { useNavigate } from 'react-router-dom'

export const ModerContentComponent: React.FC = () => {
  const [moderators, setModerators] = useState<Moderator[]>([])
  const [copied, setCopied] = useState<string | null>(null)
  const [openMenuId, setOpenMenuId] = useState<string | null>(null)
  const [menuClosing, setMenuClosing] = useState<string | null>(null)
  const [changeRoleMod, setChangeRoleMod] = useState<Moderator | null>(null)
  const [changeRoleClosing, setChangeRoleClosing] = useState(false)
  const [selectedRole, setSelectedRole] = useState<RoleType>('super_admin')
  const [roleDropdownOpen, setRoleDropdownOpen] = useState(false)
  const [addModerOpen, setAddModerOpen] = useState(false)
  const [addModerClosing, setAddModerClosing] = useState(false)
  const [addEmail, setAddEmail] = useState('')
  const [addRole, setAddRole] = useState<RoleType>('super_admin')
  const [addRoleDropdownOpen, setAddRoleDropdownOpen] = useState(false)
  const [addLoading, setAddLoading] = useState(false)
  const [addError, setAddError] = useState<string | null>(null)
  const [addCreated, setAddCreated] = useState<CreateAdminResult | null>(null)
  const menuRef = useRef<HTMLDivElement | null>(null)
  const navigate = useNavigate(); // создаем навигатор

  useEffect(() => {
  fetchModerators()
    .then(setModerators)
    .catch((err) => {
      console.error('Ошибка загрузки модераторов:', err)
      if (err?.message?.includes('401') || err?.message?.includes('403')) {
        navigate('/log')
      }
    })
}, [navigate])

  const closeMenu=(id: string) => {
    setMenuClosing(id)
    setTimeout(() => { setOpenMenuId(null); setMenuClosing(null) }, 300)
  }

  const closeChangeRole=() => {
    setChangeRoleClosing(true)
    setTimeout(() => { setChangeRoleMod(null); setChangeRoleClosing(false)}, 300)
  }

  const closeAddModer = () => {
    setAddModerClosing(true)
    setTimeout(() => {
      setAddModerOpen(false)
      setAddModerClosing(false)
      setAddCreated(null)
      setAddError(null)
    }, 300)
  }

  const handleCreateAdmin = async () => {
    setAddLoading(true)
    setAddError(null)
    
      const handleApiError = (err: any) => {
        // Можно добавить логирование, показывать сообщения и т.п.
        console.error('API Error:', err);
        navigate('/log');
      }
    try {
      const result = await createAdmin(addEmail, addRole)
      setAddCreated(result)
      // Обновляем список модераторов
      fetchModerators()
        .then(setModerators)
        .catch(() => {
          navigate('/log')
        })
    } catch (err: any) {
      handleApiError(err);
    } finally {
      setAddLoading(false)
    }
  }

  const openChangeRole = (mod: Moderator) => {
    setSelectedRole(mod.role)
    setChangeRoleMod(mod)
    if (openMenuId) closeMenu(openMenuId)
  }

  const handleChangeRole = async () => {
    if (!changeRoleMod) return
    try {
      await changeAdminRole(changeRoleMod.id, selectedRole)
      setModerators(prev => prev.map(m => m.id === changeRoleMod.id ? { ...m, role: selectedRole } : m))
      closeChangeRole()
    } catch (err: any) {
      console.error('Ошибка изменения роли:', err?.message)
      // Можно еще редиректить или показывать ошибку
    }
  }

  const handleDelete = async (mod: Moderator) => {
    try {
      await deleteAdmin(mod.id)
      setModerators(prev => prev.filter(m => m.id !== mod.id))
    } catch (err: any) {
      console.error('Ошибка удаления:', err?.message)
      // Можно редирект тут тоже при ошибках
      // navigate('/log');
    }
  }

  const handleCopy = (login: string) => {
    navigator.clipboard.writeText(login)
    setCopied(login)
    setTimeout(() => setCopied(null), 1500)
  }

  return (
      <div className="moder-content">
        <div className="moder-content__top">
          <div className="moder-content__title-block">
            <svg width="26" height="26" viewBox="0 0 26 26" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M17.663 3.79167C18.2236 4.12886 18.6874 4.60537 19.0094 5.17486C19.3313 5.74436 19.5005 6.38747 19.5003 7.04167C19.5005 7.69587 19.3313 8.33898 19.0094 8.90847C18.6874 9.47797 18.2236 9.95447 17.663 10.2917M2.16699 22.75C2.16699 20.3233 2.16699 18.46 2.63933 17.5327C3.05477 16.7173 3.71766 16.0544 4.53299 15.639C5.46033 15.1667 6.67366 15.1667 9.10032 15.1667H11.4837C13.9103 15.1667 15.1237 15.1667 16.051 15.639C16.8663 16.0544 17.5292 16.7173 17.9447 17.5327C18.417 18.46 18.417 20.3233 18.417 22.75M23.8337 22.75V22.1C23.8337 19.6733 23.8337 18.46 23.3613 17.5327C22.9459 16.7173 22.283 16.0544 21.4677 15.639M10.292 10.8333C10.7899 10.8333 11.283 10.7353 11.743 10.5447C12.203 10.3542 12.621 10.0749 12.9731 9.72278C13.3252 9.37069 13.6045 8.9527 13.795 8.49268C13.9856 8.03265 14.0837 7.5396 14.0837 7.04167C14.0837 6.54374 13.9856 6.05068 13.795 5.59066C13.6045 5.13063 13.3252 4.71264 12.9731 4.36055C12.621 4.00846 12.203 3.72917 11.743 3.53862C11.283 3.34807 10.7899 3.25 10.292 3.25C9.28638 3.25 8.32195 3.64948 7.61088 4.36055C6.8998 5.07163 6.50033 6.03605 6.50033 7.04167C6.50033 8.04728 6.8998 9.0117 7.61088 9.72278C8.32195 10.4339 9.28638 10.8333 10.292 10.8333Z" stroke="#555555" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <div>
              <h1>Модераторы</h1>
              <p>Управление учётными записями модераторов</p>
            </div>
          </div>
          <button className="moder-content__add-btn" onClick={() => { setAddEmail(''); setAddRole('super_admin'); setAddModerOpen(true) }}>
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M7 1V13M1 7H13" stroke="white" strokeWidth="2" strokeLinecap="round"/>
            </svg>
            Добавить модератора
          </button>
        </div>

        <div className="moder-table">
          <div className="moder-table__header">
            <span>ID</span>
            <span>Email</span>
            <span>Логин</span>
            <span>Роль</span>
            <span>Создан</span>
            <span>Действие</span>
          </div>

          {moderators.map(mod => (
            <ModerCard
              key={mod.id}
              mod={mod}
              copied={copied}
              openMenuId={openMenuId}
              menuClosing={menuClosing}
              menuRef={menuRef}
              onCopy={handleCopy}
              onMenuToggle={setOpenMenuId}
              onMenuClose={closeMenu}
              onChangeRole={openChangeRole}
              onDelete={handleDelete}
            />
          ))}
        </div>

        {changeRoleMod && (
          <ModeRole
            changeRoleMod={changeRoleMod}
            changeRoleClosing={changeRoleClosing}
            selectedRole={selectedRole}
            roleDropdownOpen={roleDropdownOpen}
            onDropdownToggle={() => setRoleDropdownOpen(o => !o)}
            onRoleSelect={role => { setSelectedRole(role); setRoleDropdownOpen(false) }}
            onCancel={closeChangeRole}
            onSave={handleChangeRole}
          />
        )}

        {addModerOpen && (
          <div className={`role-modal__overlay${addModerClosing ? ' role-modal__overlay--closing' : ''}`}>
            <div className={`role-modal${addModerClosing ? ' role-modal--closing' : ''}`} onMouseDown={e => e.stopPropagation()}>
              <h2 className="role-modal__title">Добавить модератора</h2>
              <p className="role-modal__subtitle">Логин будет сгенерирован автоматически</p>
              <label className="role-modal__label">Email</label>
              <input
                className="role-modal__input"
                type="email"
                placeholder="email@example.com"
                value={addEmail}
                onChange={e => setAddEmail(e.target.value)}
              />
              <label className="role-modal__label">Роль</label>
              <div className="role-modal__select-wrap" onClick={() => setAddRoleDropdownOpen(o => !o)}>
                <svg xmlns="http://www.w3.org/2000/svg" className="role-modal__select-icon" width="14" height="14" viewBox="0 0 14 14" fill="none">
                  <path d="M11.6663 7.58331C11.6663 10.5 9.62467 11.9583 7.19801 12.8041C7.07094 12.8472 6.9329 12.8451 6.80717 12.7983C4.37467 11.9583 2.33301 10.5 2.33301 7.58331V3.49998C2.33301 3.34527 2.39447 3.19689 2.50386 3.0875C2.61326 2.9781 2.76163 2.91664 2.91634 2.91664C4.08301 2.91664 5.54134 2.21664 6.55634 1.32998C6.67992 1.22439 6.83713 1.16638 6.99967 1.16638C7.16222 1.16638 7.31943 1.22439 7.44301 1.32998C8.46384 2.22248 9.91634 2.91664 11.083 2.91664C11.2377 2.91664 11.3861 2.9781 11.4955 3.0875C11.6049 3.19689 11.6663 3.34527 11.6663 3.49998V7.58331Z" stroke="#555" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
                <span className="role-modal__select-label">{ROLE_LABEL[addRole]}</span>
                <svg className={`role-modal__chevron${addRoleDropdownOpen ? ' role-modal__chevron--open' : ''}`} width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M6 9L12 15L18 9" stroke="#555" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
                {addRoleDropdownOpen && (
                  <div className="role-modal__options">
                    {(['admin', 'super_admin'] as RoleType[]).map(role => (
                      <div
                        key={role}
                        className={`role-modal__option${addRole === role ? ' role-modal__option--active' : ''}`}
                        onMouseDown={e => { e.stopPropagation(); setAddRole(role); setAddRoleDropdownOpen(false) }}
                      >
                        {role === 'super_admin' ? (
                          <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none" className="role-modal__option-icon--super">
                            <path d="M11.6663 7.58331C11.6663 10.5 9.62467 11.9583 7.19801 12.8041C7.07094 12.8472 6.9329 12.8451 6.80717 12.7983C4.37467 11.9583 2.33301 10.5 2.33301 7.58331V3.49998C2.33301 3.34527 2.39447 3.19689 2.50386 3.0875C2.61326 2.9781 2.76163 2.91664 2.91634 2.91664C4.08301 2.91664 5.54134 2.21664 6.55634 1.32998C6.67992 1.22439 6.83713 1.16638 6.99967 1.16638C7.16222 1.16638 7.31943 1.22439 7.44301 1.32998C8.46384 2.22248 9.91634 2.91664 11.083 2.91664C11.2377 2.91664 11.3861 2.9781 11.4955 3.0875C11.6049 3.19689 11.6663 3.34527 11.6663 3.49998V7.58331Z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                            <path d="M5.25 6.99998L6.41667 8.16665L8.75 5.83331" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                          </svg>
                        ) : (
                          <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 14 14" fill="none">
                            <path d="M11.6663 7.58331C11.6663 10.5 9.62467 11.9583 7.19801 12.8041C7.07094 12.8472 6.9329 12.8451 6.80717 12.7983C4.37467 11.9583 2.33301 10.5 2.33301 7.58331V3.49998C2.33301 3.34527 2.39447 3.19689 2.50386 3.0875C2.61326 2.9781 2.76163 2.91664 2.91634 2.91664C4.08301 2.91664 5.54134 2.21664 6.55634 1.32998C6.67992 1.22439 6.83713 1.16638 6.99967 1.16638C7.16222 1.16638 7.31943 1.22439 7.44301 1.32998C8.46384 2.22248 9.91634 2.91664 11.083 2.91664C11.2377 2.91664 11.3861 2.9781 11.4955 3.0875C11.6049 3.19689 11.6663 3.34527 11.6663 3.49998V7.58331Z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                          </svg>
                        )}
                        <span>{ROLE_LABEL[role]}</span>
                        {addRole === role && (
                          <svg className="role-modal__check" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 12" fill="none">
                            <path d="M5.28846 8.775L13.7635 0.3C13.9635 0.1 14.1968 0 14.4635 0C14.7301 0 14.9635 0.1 15.1635 0.3C15.3635 0.5 15.4635 0.737667 15.4635 1.013C15.4635 1.28833 15.3635 1.52567 15.1635 1.725L5.98846 10.925C5.78846 11.125 5.55513 11.225 5.28846 11.225C5.02179 11.225 4.78846 11.125 4.58846 10.925L0.288462 6.625C0.0884617 6.425 -0.00753846 6.18767 0.000461538 5.913C0.00846154 5.63833 0.112795 5.40067 0.313461 5.2C0.514128 4.99933 0.751795 4.89933 1.02646 4.9C1.30113 4.90067 1.53846 5.00067 1.73846 5.2L5.28846 8.775Z" fill="white"/>
                          </svg>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
              {addError && (
                <p className="role-modal__error">{addError}</p>
              )}
              {addCreated ? (
                <>
                  <div className="role-modal__created">
                    <p>Логин: <strong>{addCreated.login}</strong></p>
                    <p>Пароль: <strong>{addCreated.password}</strong></p>
                    <p className="role-modal__created-warn">Сохраните данные — повторно получить не получится</p>
                  </div>
                  <button className="role-modal__save" onClick={closeAddModer}>
                    Закрыть
                  </button>
                </>
              ) : (
                <div className="role-modal__actions">
                  <button className="role-modal__cancel" onClick={closeAddModer}>
                    Отмена
                  </button>
                  <button
                    className="role-modal__save"
                    disabled={!addEmail.includes('@') || addLoading}
                    style={{ opacity: addEmail.includes('@') ? 1 : 0.5, cursor: addEmail.includes('@') ? 'pointer' : 'default' }}
                    onClick={handleCreateAdmin}
                  >
                    {addLoading ? '...' : 'Создать'}
                  </button>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    )
  }