import React, { useState } from 'react'
import { Header } from '../../widgets/header.tsx'
import { SuggestEventModal } from '../../features/form/ui/modal.tsx'
import { IndexContentComponent } from '../../features/main-content/ui/index-content.tsx' //@ts-ignore
import './index.scss' //@ts-ignore
import '../../widgets/mobile.scss'

export const Index: React.FC = () => {
  const [isModalOpen, setIsModalOpen] = useState(false);

  const openModal = () => setIsModalOpen(true);
  const closeModal = () => setIsModalOpen(false);
  return (
    <div>
      <Header onOpenModal={openModal} />
      <SuggestEventModal 
        isOpen={isModalOpen}
        onClose={closeModal}
      />
      <IndexContentComponent />
    </div>
  );
};