import React, { useState } from 'react'
import { FooterButton } from '../entities/butter'
import { FooterContent } from '../features/map-content/ui/footer-content'

export interface FooterProps {
  collapsible?: boolean;
}

export const Footer: React.FC<FooterProps> = ({ collapsible = false }) => {
  const [isOpen, setIsOpen] = useState(false);

  if (collapsible) {
    return (
      <footer className={`footer--collapsible${isOpen ? ' footer--open' : ''}`}>
        <FooterButton onClick={() => setIsOpen(v => !v)} />
        <FooterContent />
      </footer>
    );
  }
};