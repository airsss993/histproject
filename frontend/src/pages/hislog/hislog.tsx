import React, { useEffect } from 'react'
import { Headmer } from '../../widgets/headmer.tsx' // @ts-ignore
import '../../widgets/mobile.scss'
import { HisLogContentComponent } from '../../features/panel-content/hislog-content.tsx' // @ts-ignore
import './hislog.scss'
import { useAdminReFresh } from '../../features/refresh/saga.ts' // Исправить
import { useSelector } from 'react-redux'
import { selectors } from '../../features/refresh/selectors'
import { useNavigate } from 'react-router-dom'

export const HisLog: React.FC = () => {
  const navigate = useNavigate();
  const navigateToLogin = useSelector(selectors.selectNavigateToLogin);
  useAdminReFresh();
  useEffect(() => {
    if (navigateToLogin) {
      navigate('/log');
    }
  }, [navigateToLogin, navigate]);

  return (
    <div className="HislogPage">
      <Headmer />
      <HisLogContentComponent />
    </div>
  )
}
