import React from 'react'

interface FooterButtonProps {
  onClick?: () => void;
}

export const FooterButton: React.FC<FooterButtonProps> = ({ onClick }) => {
  return (
    <button className="footer__toggle" onClick={onClick}>
      <span className="footer__toggle-label">Информация о проекте</span>
    </button>
  );
};