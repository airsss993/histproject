import React, { useState } from 'react'
import { BuTG } from '../../../entities/butg'

interface SuccessModalProps {
  isOpen: boolean;
  onClose: () => void;
  onReset?: () => void;
}

export const SuccessModal: React.FC<SuccessModalProps> = ({ isOpen, onClose, onReset }) => {
  const [isClosing, setIsClosing] = useState(false);

  const handleClose = () => {
    if (onReset) {
      setTimeout(() => {
        onReset();
      }, 300)
    }
    setIsClosing(true);
    setTimeout(() => {
      setIsClosing(false);
      onClose();
    }, 300);
  };

  const handleReturnToMap = () => {
    window.open('https://t.me/russia_heroes_bot', '_blank');
    if (onReset) {
      setTimeout(() => {
        onReset();
      }, 300)
    }
    handleClose();
  };

  if (!isOpen && !isClosing) return null;

  return (
    <div
      className={`modal-overlay ${isOpen && !isClosing ? 'open' : isClosing ? 'close' : ''}`}
      style={{ display: (isOpen || isClosing) ? 'flex' : 'none' }}
      onClick={(e) => e.stopPropagation()}
    >
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <button className="modal-close" onClick={handleClose}>×</button>
        <h2 className="modal-title">Заявка отправлена!</h2>
        <p className="modal-description1 modal-descriptions">
          Спасибо за ваше предложение. Заявка успешно передана на модерацию.
        </p>
        <p className="modal-description2 modal-descriptions">
          Мы проверим корректность данных и содержимое загруженных материалов.
        </p>
        <p className="modal-description3 modal-descriptions">
          После рассмотрения вы получите уведомление:<br />
          в <u>Telegram</u> - на указанный вами псевдоним;<br />
          на электронную почту - на указанный адрес.
        </p>
        <p className="modal-description4 modal-descriptions">
          Статусы, о которых мы сообщим:
          <ul>
            <li>Заявка принята в работу.</li>
            <li>Заявка одобрена/не одобрена.</li>
            <li>Опубликовано.</li>
          </ul>
        </p>
        <p className="modal-description5 modal-descriptions">
          Вы можете закрыть это окно и продолжить работу с картой.
        </p>
        <div className="success-actions">
          <BuTG onClick={handleReturnToMap} />
        </div>
      </div>
    </div>
  );
};