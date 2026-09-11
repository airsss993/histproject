import React from 'react'
import { RoleType, ROLE_LABEL } from './modcard'
import { rolesvg } from './svgrole'
import { rolesvg as checksvg } from './svgserole'

interface DropDownRoleProps {
  selectedRole: RoleType
  onRoleSelect: (role: RoleType) => void
}

export const DropDownRole: React.FC<DropDownRoleProps> = ({ selectedRole, onRoleSelect }) => (
  <div className="role-modal__options">
    {(['admin', 'super_admin'] as RoleType[]).map(role => (
      <div
        key={role}
        className={`role-modal__option${selectedRole === role ? ' role-modal__option--active' : ''}`}
        onMouseDown={e => { e.stopPropagation(); onRoleSelect(role) }}
      >
        {rolesvg[role]}
        <span>{ROLE_LABEL[role]}</span>
        {selectedRole === role && checksvg}
      </div>
    ))}
  </div>
)
