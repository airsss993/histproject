export interface AuthState {
  navigateToLogin: any;
  login: string;
  password: string;
  loading: boolean;
  error: string | null;
  isAuthenticated: boolean;
  role: string | null;
  username: string | null;
  logoutLoading: boolean;
  logoutCompleted: boolean;
  refreshReady: boolean;
  refreshUnauthorized: boolean;
  navigateTo: string | null;
  auth: null;
}