import { createSlice, PayloadAction } from '@reduxjs/toolkit'
import { AuthState } from '../login/types'

export const name = 'refresh'

const initialState: AuthState = {
  login: '',
  password: '',
  loading: false,
  error: null,
  isAuthenticated: false,
  role: null,
  username: localStorage.getItem('username'),
  logoutLoading: false,
  logoutCompleted: false,
  refreshReady: false,
  refreshUnauthorized: false,
  navigateToLogin: false,
  navigateTo: null,
  auth: null
};

export const { reducer, actions } = createSlice({
  name,
  initialState,
  reducers: {
    setLogin: (state, action: PayloadAction<string>) => {
      state.login = action.payload;
    },
    setPassword: (state, action: PayloadAction<string>) => {
      state.password = action.payload;
    },
    loginSuccess: (state, action: PayloadAction<{ role: string | null; username: string }>) => {
      state.loading = false;
      state.isAuthenticated = true;
      state.role = action.payload.role;
      state.username = action.payload.username;
      state.error = null;
      state.navigateToLogin = false;
    },
    startRefreshTimer: () => {},
    stopRefreshTimer: () => {},
    refreshRequest: (state) => {
      state.refreshUnauthorized = false;
    },
    refreshSuccess: (state) => {
      state.refreshReady = true;
      state.refreshUnauthorized = false;
      state.navigateToLogin = false;
    },
    refreshUnauthorized: (state) => {
      state.refreshReady = false;
      state.refreshUnauthorized = true;
      state.isAuthenticated = false;
      state.navigateToLogin = true;
    },
    refreshReset: (state) => {
      state.refreshReady = false;
      state.refreshUnauthorized = false;
      state.navigateToLogin = false;
    },
    clearNavigateToLogin: (state) => {
      state.navigateToLogin = false;
      state.navigateTo = '/log';
    },
    setNavigateToLogin: (state) => {
      state.navigateToLogin = true;
      state.navigateTo = '/log';
    },
    forceLogoutAndRedirect: (state) => {
      state.isAuthenticated = false;
      state.role = null;
      state.username = null;
      state.refreshReady = false;
      state.navigateToLogin = true;
      state.navigateTo = '/log';
      localStorage.removeItem('username');
    },
  },
});