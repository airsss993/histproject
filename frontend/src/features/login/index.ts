import { name, reducer, actions } from './slice'
import { LogInInit } from './saga'
import { selectors } from './selectors'

export const LogInFeature = {
    sagas: {
        init: LogInInit
    },
    actions,
    reducer: {
        [name]: reducer 
    },
    selectors
}