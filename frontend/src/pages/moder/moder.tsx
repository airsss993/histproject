import React, { useEffect } from 'react'
import { Headmer } from '../../widgets/headmer.tsx'
import { ModerContentComponent } from '../../features/panel-content/moder-content.tsx' // @ts-ignore
import '../admin/admin.scss' // @ts-ignore
import './moder.scss'
import { useAdminReFresh } from '../../features/refresh/saga.ts' // Исправить
import { useSelector } from 'react-redux'
import { selectors } from '../../features/refresh/selectors'
import { useNavigate } from 'react-router-dom'

export const Moder: React.FC = () => {
  const navigate = useNavigate();
  const navigateToLogin = useSelector(selectors.selectNavigateToLogin);
  useAdminReFresh();
  useEffect(() => {
    if (navigateToLogin) {
      navigate('/log');
    }
  }, [navigateToLogin, navigate]);

  return (
    <div className="AdminPage">
      <Headmer />
      <ModerContentComponent />
    </div>
  )
}