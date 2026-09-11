import { name, reducer, actions } from './slice'
import { LogOutInit } from './saga'
import { selectors } from './selectors'

export const LogOutFeature = {
    sagas: {
        init: LogOutInit
    },
    actions,
    reducer: {
        [name]: reducer
    },
    selectors
}