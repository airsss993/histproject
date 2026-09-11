import React from 'react'
import CITPanda from '../../../assets/CITPandaFonot.png'
import { CITButton } from '../../../entities/butci'
import { PMTGButton } from '../../../entities/butopm'
import { CDTGButton } from '../../../entities/butocd'
import { FETGButton } from '../../../entities/butofe'
import { BETGButton } from '../../../entities/butobe'

export const FooterContent: React.FC = () => {
  return (
    <div className="footer__content">
      <div className="footer__inner">
        <div className="footer-left">
          <img src={CITPanda} alt="CIT_Panda_Logo" className="footer-logo" />
        </div>
        <div className="footer-right">
          <p className="footer-title">
            Проект студентов{' '}
            <CITButton />
          </p>
          <span className="footer-copy">© 2026</span>
          <p className="footer-names">
            <PMTGButton />,&nbsp; 
            <CDTGButton />,&nbsp;
            <FETGButton />,&nbsp;
            <BETGButton />
          </p>
        </div>
      </div>
    </div>
  );
};