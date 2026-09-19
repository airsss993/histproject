import React from 'react'

// Поле формы из дизайн-системы (ui.css): подпись, рамка со светом фокуса, ошибка или подсказка.
// children - голый input, textarea или select; id и aria-атрибуты Field проставляет сам.
export const Field: React.FC<{
  label?: string; error?: string; hint?: React.ReactNode; after?: React.ReactNode; className?: string; children: React.ReactElement
}> = ({ label, error, hint, after, className, children }) => {
  const id = React.useId()
  return (
    <div className={`pm-form-field${className ? ` ${className}` : ''}`}>
      {label && <label className="pm-label" htmlFor={id}>{label}</label>}
      <div className={`pm-field pm-glow${error ? ' bad' : ''}`}>
        {React.cloneElement(children as React.ReactElement<Record<string, unknown>>, {
          id, 'aria-invalid': !!error, 'aria-describedby': error ? `${id}-e` : undefined,
        })}
        {after}
      </div>
      {error ? <p className="pm-field-error" id={`${id}-e`}>{error}</p> : hint}
    </div>
  )
}
