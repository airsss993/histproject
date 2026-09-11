import { name, reducer, actions } from './slice'
import { formInit } from './saga'
export { SuggestEventModal } from './ui/modal'
export { ContactModal } from './ui/contac'
export * from './selectors'

export const FormFeature = {
  sagas: { init: formInit },
  actions,
  reducer: { [name]: reducer },
}