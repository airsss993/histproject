import { RootState } from '../../app/store'

export const selectName = (state: RootState) => state.formDesign.name
export const selectDate = (state: RootState) => state.formDesign.date
export const selectDescription = (state: RootState) => state.formDesign.description
export const selectEventType = (state: RootState) => state.formDesign.eventType
export const selectEventTypeLabel = (state: RootState) => state.formDesign.eventTypeLabel
export const selectZipFile = (state: RootState) => state.formDesign.zipFile
export const selectEventName = (state: RootState) => state.formDesign.eventName
export const selectEventTypeId = (state: RootState) => state.formDesign.eventTypeId
export const selectEventPayload = (state: RootState) => state.formDesign.eventPayload
export const selectShowContactModal = (state: RootState) => state.formDesign.showContactModal
export const selectEmail = (state: RootState) => state.formDesign.email
export const selectTelegram = (state: RootState) => state.formDesign.telegram
export const selectIsSubmitting = (state: RootState) => state.formDesign.isSubmitting
export const selectSubmitDone = (state: RootState) => state.formDesign.submitDone
export const selectError = (state: RootState) => state.formDesign.error
export const selectIsSuccessOpen = (state: RootState) => state.formDesign.isSuccessOpen

export const selectIsoDate = (state: RootState): string | null => {
  const { date } = state.formDesign
  const parts = date.split('.')
  if (parts.length !== 3) return null
  const [ddStr, mmStr, yyyyStr] = parts
  const day = Number(ddStr)
  const month = Number(mmStr)
  const year = Number(yyyyStr)
  if (
    !Number.isInteger(day) ||
    !Number.isInteger(month) ||
    !Number.isInteger(year)
  ) return null
  if (year < 862 || year > 2026) return null
  if (month < 1 || month > 12) return null
  const daysInMonth = new Date(year, month, 0).getDate()
  if (day < 1 || day > daysInMonth) return null
  const yyyy = String(year).padStart(4, '0')
  const mm = String(month).padStart(2, '0')
  const dd = String(day).padStart(2, '0')
  return `${yyyy}-${mm}-${dd}`
}

export const selectIsFileSizeValid = (state: RootState): boolean => {
  const MAX_FILE_SIZE = 350 * 1024 * 1024 // 350 МБ
  const { zipFile } = state.formDesign
  return !zipFile || zipFile.size <= MAX_FILE_SIZE
}

export const selectIsMainFormValid = (state: RootState): boolean => {
  const { name, date, description, eventType, zipFile } = state.formDesign
  return (
    name.trim() !== '' &&
    date.trim() !== '' &&
    description.trim() !== '' &&
    eventType !== null &&
    zipFile !== null
  )
}

export const selectIsContactFormValid = (state: RootState): boolean => {
  const { email, telegram } = state.formDesign
  return email.includes('@') && email.trim() !== '' && telegram.trim() !== ''
}