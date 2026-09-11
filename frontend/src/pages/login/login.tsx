import React from 'react'
import { Hepander } from '../../widgets/hepander.tsx' // @ts-ignore
import '../../widgets/mobile.scss'
import { LogInContentComponent } from '../../features/panel-content/login-content.tsx' // @ts-ignore
import './login.scss'

export const LogIn: React.FC = () => { 
  return (
    <div className="LoginPage">
      <Hepander />
      <LogInContentComponent />
    </div>
  );
}
