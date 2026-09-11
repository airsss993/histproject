import { createSelector } from '@reduxjs/toolkit'
import { name } from './slice'
import { AuthState } from '../login/types'

interface State {
    [name]: AuthState | undefined
}

const root = (state: State) => state[name]

const selectRefreshReady = createSelector(root, rootData => rootData?.refreshReady ?? false)
const selectRefreshUnauthorized = createSelector(root, rootData => rootData?.refreshUnauthorized ?? false)
const selectIsAuthenticated = createSelector(root, rootData => rootData?.isAuthenticated ?? false)
const selectNavigateToLogin = createSelector(root, rootData => rootData?.navigateToLogin ?? false)

export const selectors = {
    selectRefreshReady,
    selectRefreshUnauthorized,
    selectIsAuthenticated,
    selectNavigateToLogin
};