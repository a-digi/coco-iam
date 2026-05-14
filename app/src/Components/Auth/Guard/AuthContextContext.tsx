import { createContext } from 'react';
import type { AuthToken } from './model/auth';

interface AuthContextType {
  authenticated: boolean;
  setAuthenticated: (auth: boolean) => void;
  authToken: AuthToken | null;
  setAuthToken: (token: AuthToken | null) => void;
  login: (token: AuthToken) => void; // updated to accept AuthToken
  logout: () => void;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);
