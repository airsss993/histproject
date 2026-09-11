import { createSlice, PayloadAction } from '@reduxjs/toolkit'

interface ContactEventPayload {
  name: string;
  date: string;
  description: string;
  eventType: string;
  zipFile?: File | null;
}

interface FormDesignState {
  name: string
  date: string
  description: string
  eventType: string | null
  eventTypeLabel: string | null
  zipFile: File | null
  eventName: string
  eventPayload: ContactEventPayload | null
  eventTypeId: number | null
  showContactModal: boolean
  email: string
  telegram: string
  isSubmitting: boolean
  submitDone: boolean
  error: string | null
  isSuccessOpen: boolean
}

const initialState: FormDesignState = {
  name: '',
  date: '',
  description: '',
  eventType: null,
  eventTypeLabel: null,
  zipFile: null,
  eventName: '',
  eventPayload: null,
  eventTypeId: null,
  showContactModal: false,
  email: '',
  telegram: '',
  isSubmitting: false,
  submitDone: false,
  error: null,
  isSuccessOpen: false,
}

export const { name, reducer, actions } = createSlice({
  name: 'formDesign',
  initialState,
  reducers: {
    setName: (state, action: PayloadAction<string>) => { state.name = action.payload },
    setDate: (state, action: PayloadAction<string>) => { state.date = action.payload },
    setDescription: (state, action: PayloadAction<string>) => { state.description = action.payload },
    setEventType: (state, action: PayloadAction<{ value: string; label: string }>) => {
      state.eventType = action.payload.value
      state.eventTypeLabel = action.payload.label
    },
    setZipFile: (state, action: PayloadAction<File | null>) => { state.zipFile = action.payload },
    setEventName: (state, action: PayloadAction<string>) => { state.eventName = action.payload },
    setEventPayload: (state, action: PayloadAction<ContactEventPayload>) => {  // Добавлен этот редюсер
      state.eventPayload = action.payload
    },
    setEventTypeId: (state, action: PayloadAction<number | null>) => { state.eventTypeId = action.payload },
    setShowContactModal: (state, action: PayloadAction<boolean>) => { state.showContactModal = action.payload },
    setEmail: (state, action: PayloadAction<string>) => { state.email = action.payload },
    setTelegram: (state, action: PayloadAction<string>) => { state.telegram = action.payload },
    submitRequest: (state) => {
      state.isSubmitting = true
      state.submitDone = false
      state.error = null
    },
    submitSuccess: (state) => {
      state.isSubmitting = false
      state.submitDone = true
    },
    submitFailure: (state, action: PayloadAction<string>) => {
      state.isSubmitting = false
      state.error = action.payload
    },
    resetSubmitDone: (state) => { state.submitDone = false },
    openSuccessModal: (state) => { state.isSuccessOpen = true },
    closeSuccessModal: (state) => { state.isSuccessOpen = false },
    resetMainForm: (state) => {
      state.name = ''
      state.date = ''
      state.description = ''
      state.eventType = null
      state.eventTypeLabel = null
      state.zipFile = null
      state.eventTypeId = null
      state.eventPayload = null
      state.eventName = ''
    },
    resetContactForm: (state) => {
      state.email = ''
      state.telegram = ''
    },
  },
})