// import { useNavigate } from 'react-router-dom'
import { admins, DelAdmins, RolAdmins, DelForm, adminReview, AddForm } from '../../../entities/cons'
import { Moderator, RoleType } from './modcard'

export interface CreateAdminResult {
  login: string
  password: string
}

export const createAdmin = async (email: string, role: RoleType): Promise<CreateAdminResult> => {
  const response = await fetch(admins, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, role }),
  })
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }
  const json = await response.json()
  console.log('[createAdmin] response:', json)
  return { login: json.login, password: json.password }
}

export const deleteAdmin = async (id: string | number): Promise<void> => {
  const response = await fetch(DelAdmins(Number(id)), {
    method: 'DELETE',
    credentials: 'include',
  })
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }
  console.log('[deleteAdmin] response:', await response.json())
}

export const changeAdminRole = async (id: string | number, role: RoleType): Promise<void> => {
  const response = await fetch(RolAdmins(Number(id)), {
    method: 'PATCH',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ role }),
  })
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }
  console.log('[changeAdminRole] response:', await response.json())
}

export const reviewRequest = async (id: string | number): Promise<void> => {
  const response = await fetch(adminReview(Number(id)), {
    method: 'PATCH',
    credentials: 'include',
  })
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }
  console.log('[reviewRequest] response:', await response.json())
}

export const approveRequest = async (id: string | number, eventTypeId: number): Promise<void> => {
  const response = await fetch(AddForm(Number(id)), {
    method: 'PATCH',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ eventTypeId }),
  })
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }
  console.log('[approveRequest] response:', await response.json())
}

export const rejectRequest = async (id: string | number, comment: string): Promise<void> => {
  const response = await fetch(DelForm(Number(id)), {
    method: 'PATCH',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ comment }),
  })
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }
  console.log('[rejectRequest] response:', await response.json())
}

export const fetchModerators = async (): Promise<Moderator[]> => {
  // const navigate = useNavigate();
  try {
    const response = await fetch(admins, {
      method: 'GET',
      credentials: 'include',
    })
    if (!response.ok) {
      // showErrorAlert(response.status)
      throw new Error(`HTTP ${response.status}`)
    }
    const json = await response.json()
    console.log('[fetchModerators] raw response:', json)
    const data: any[] = Array.isArray(json)
      ? json
      : Array.isArray(json?.data)
      ? json.data
      : Array.isArray(json?.admins)
      ? json.admins
      : Array.isArray(json?.results)
      ? json.results
      : Array.isArray(json?.items)
      ? json.items
      : []
    return data.map((item): Moderator => ({
      id: String(item.id),
      email: item.email ?? item.email_address ?? item.user_email ?? item.admin_email ?? '',
      login: item.login ?? item.username ?? item.admin_login ?? '',
      role: (item.role ?? item.role_type ?? item.roleType ?? 'admin') as RoleType,
      createdAt: item.created_at ?? item.createdAt ?? item.created_date ?? '',
    }))
  } catch (err) {
    console.error('Ошибка загрузки модераторов:', err)
    // navigate('/log');
    return []
  }
}
