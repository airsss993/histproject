import React, { useEffect } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import { useNavigate } from 'react-router-dom'
import { LogInFeature } from '../login'

export const LogInContentComponent: React.FC = () => {
  // Удаляешь и меняешь на useSelector(selector login frature) || dispatch(action login feature) (useDispatch)
  // const { login, setLogin, password, setPassword, error, loading, handleLogin } = useAdminLogin();
  const dispatch = useDispatch()
  const navigate = useNavigate();

  const login = useSelector(LogInFeature.selectors.selectLogin);
  const password = useSelector(LogInFeature.selectors.selectPassword);
  const error = useSelector(LogInFeature.selectors.selectError);
  const loading = useSelector(LogInFeature.selectors.selectLoading);
  const navigateTo = useSelector(LogInFeature.selectors.selectNavigateTo);
  
  useEffect(() => {
    if (navigateTo) {
      navigate(navigateTo);
      dispatch(LogInFeature.actions.clearNavigateTo());
    }
  }, [navigateTo, navigate, dispatch]);

  const handleLogin = (e: React.FormEvent) => {
    e.preventDefault();
    dispatch(LogInFeature.actions.loginRequest());
  };

  return (
    <form className="AdminLogIn" onSubmit={handleLogin}>
      <h1>Вход</h1>
      <div className="admin-group">
        <div className="admin-login">
          <label htmlFor="LogIn">Логин</label>
          <br />
          <input
            type="text"
            id="LogIn"
            name="login"
            value={login}
            onChange={(e) => dispatch(LogInFeature.actions.setLogin(e.target.value))}
            required
          />
        </div>
        <div className="admin-login">
          <label htmlFor="PassWord">Пароль</label>
          <br />
          <input
            type="password"
            id="PassWord"
            name="password"
            value={password}
            onChange={(e) => dispatch(LogInFeature.actions.setPassword(e.target.value))}
            required
          />
        </div>
      </div>
      {error && <p className="login-error">{error}</p>}
      <button type="submit" disabled={loading}>
        {loading ? 'Вход...' : 'Войти'}
      </button>
    </form>
  );
};