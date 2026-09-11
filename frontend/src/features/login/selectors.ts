import { createSelector } from '@reduxjs/toolkit'
import { name } from './slice'
import { AuthState } from './types'

interface State {
    [name]: AuthState | undefined
}

const root = (state: State) => state[name]

const selectLogin = createSelector(root, rootData => rootData?.login ?? '');
const selectPassword = createSelector(root, rootData => rootData?.password ?? '');
const selectLoading = createSelector(root, rootData => rootData?.loading ?? false);
const selectError = createSelector(root, rootData => rootData?.error ?? null);
const selectIsAuthenticated = createSelector(root, rootData => rootData?.isAuthenticated ?? false);
const selectRole = createSelector(root, rootData => rootData?.role ?? null);
const selectUsername = createSelector(root, rootData => rootData?.username ?? null);
const selectLogoutLoading = createSelector(root, rootData => rootData?.logoutLoading ?? false);
const selectLogoutCompleted = createSelector(root, rootData => rootData?.logoutCompleted ?? false);
const selectRefreshReady = createSelector(root, rootData => rootData?.refreshReady ?? false);
const selectRefreshUnauthorized = createSelector(root, rootData => rootData?.refreshUnauthorized ?? false);
const selectNavigateTo = createSelector(root, rootData => rootData?.navigateTo ?? null);

export const selectors = {
    selectLogin,
    selectPassword,
    selectLoading,
    selectError,
    selectIsAuthenticated,
    selectRole,
    selectUsername,
    selectLogoutLoading,
    selectLogoutCompleted,
    selectRefreshReady,
    selectRefreshUnauthorized,
    selectNavigateTo,
};