import { createSlice, PayloadAction } from '@reduxjs/toolkit'
import { AuthState } from './types'

export const name = 'auth'

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
  navigateTo: null,
  navigateToLogin: undefined,
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
    loginRequest: (state) => {
      state.loading = true;
      state.error = null;
    },
    loginSuccess: (state, action: PayloadAction<{ role: string | null; username: string }>) => {
      state.loading = false;
      state.isAuthenticated = true;
      state.role = action.payload.role;
      state.username = action.payload.username;
      state.error = null;
      state.navigateTo = 
        action.payload.role === 'moderator' ? '/mad' : 
        action.payload.role === 'super_admin' ? '/adm' : 
        '/log';
    },
    loginFailure: (state, action: PayloadAction<string>) => {
      state.loading = false;
      state.error = action.payload;
      state.isAuthenticated = false;
      state.navigateTo = '/log';
    },
    clearError: (state) => {
      state.error = null;
    },
    clearNavigateTo: (state) => {
      state.navigateTo = null;
    },
    logoutRequest: (state) => {
      state.logoutLoading = true;
      state.logoutCompleted = false;
    },
    logoutSuccess: (state) => {
      state.logoutLoading = false;
      state.logoutCompleted = true;
    },
    logoutReset: (state) => {
      state.logoutCompleted = false;
    },
    refreshRequest: (state) => {
      state.refreshUnauthorized = false;
    },
    refreshSuccess: (state) => {
      state.refreshReady = true;
      state.refreshUnauthorized = false;
    },
    refreshUnauthorized: (state) => {
      state.refreshReady = false;
      state.refreshUnauthorized = true;
      state.navigateTo = '/log';
    },
    refreshReset: (state) => {
      state.refreshReady = false;
      state.refreshUnauthorized = false;
    },
  },
});