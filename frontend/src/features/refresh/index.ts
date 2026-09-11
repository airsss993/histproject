import { name, reducer, actions } from './slice'
import { ReFreshInit } from './saga'

export const ReFreshFeature = {
    sagas: {
        init: ReFreshInit
    },
    actions,
    reducer: {
        [name]: reducer
    }
}