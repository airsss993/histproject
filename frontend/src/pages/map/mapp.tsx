import React, { useState } from 'react'
import { Header } from '../../widgets/header.tsx'
import { SuggestEventModal } from '../../features/form/ui/modal.tsx'
import { Map } from '../../features/map-content/ui/map.tsx'
import { Footer } from '../../widgets/footer.tsx' // @ts-ignore
import './mapp.scss' // @ts-ignore
import '../../widgets/mobile.scss'

export const Mapp: React.FC = () => {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const openModal = () => setIsModalOpen(true);
  const closeModal = () => setIsModalOpen(false);
  
  return (
    <div className="MappPage">
      <Header onOpenModal={openModal} />
      <SuggestEventModal 
        isOpen={isModalOpen}
        onClose={closeModal}
      />
      <Map />
      <Footer collapsible />
    </div>
  );
};