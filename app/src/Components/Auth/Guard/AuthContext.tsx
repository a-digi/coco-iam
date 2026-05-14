import { useState, type ReactNode } from 'react';
import { AUTH_TOKEN_KEY } from './AuthConstants';
import { AuthContext } from './AuthContextContext';
import  { type AuthToken, findAuthToken} from './model/auth';

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [authenticated, setAuthenticated] = useState<boolean>(() => !!localStorage.getItem(AUTH_TOKEN_KEY));
  const [authToken, setAuthToken] = useState<AuthToken | null>(findAuthToken);

  const login = (token: AuthToken) => {
    localStorage.setItem(AUTH_TOKEN_KEY, JSON.stringify(token));
    setAuthToken(token);
    setAuthenticated(true);
  };

  const logout = () => {
    setAuthenticated(false);
    setAuthToken(null);
    localStorage.removeItem(AUTH_TOKEN_KEY);
  };

  return (
    <AuthContext.Provider value={{ authenticated, setAuthenticated, authToken, setAuthToken, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
};
