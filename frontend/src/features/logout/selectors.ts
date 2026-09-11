import { createSelector } from '@reduxjs/toolkit'
import { name } from './slice'
import { AuthState } from '../login/types'

interface State {
    [name]: AuthState | undefined
}

const root = (state: State) => state[name]

const selectLoading = createSelector(root, rootData => rootData?.loading ?? false);
const selectError = createSelector(root, rootData => rootData?.error ?? null);
const selectIsAuthenticated = createSelector(root, rootData => rootData?.isAuthenticated ?? false);
const selectRole = createSelector(root, rootData => rootData?.role ?? null);
const selectUsername = createSelector(root, rootData => rootData?.username ?? null);
const selectAdminLoginState = createSelector(root, rootData => rootData?.auth ?? null);
const selectLogoutLoading = createSelector(root, rootData => rootData?.logoutLoading ?? false);
const selectLogoutCompleted = createSelector(root, rootData => rootData?.logoutCompleted ?? false);
const selectNavigateTo = createSelector(root, rootData => rootData?.navigateTo ?? null);

export const selectors = {
    selectLoading,
    selectError,
    selectIsAuthenticated,
    selectRole,
    selectUsername,
    selectAdminLoginState,
    selectLogoutLoading,
    selectLogoutCompleted,
    selectNavigateTo,
};
