import React, { useEffect } from 'react'
import { Heamoder } from '../../widgets/heamoder.tsx' // @ts-ignore
import '../../widgets/mobile.scss'
import { AdminContentComponent } from '../../features/panel-content/admin-content.tsx'
import { useAdminReFresh } from '../../features/refresh/saga.ts' // Исправить
import { useSelector } from 'react-redux'
import { selectors } from '../../features/refresh/selectors'
import { useNavigate } from 'react-router-dom'

export const Madder: React.FC = () => {
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
      <Heamoder />
      <AdminContentComponent />
    </div>
  );
}