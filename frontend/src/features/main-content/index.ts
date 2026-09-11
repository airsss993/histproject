import { selectors } from "./selectors"
import { name, reducer, actions } from "./slice"

export const MainContent = {
    reducer: { [name]: reducer },
    selectors,
    actions
}