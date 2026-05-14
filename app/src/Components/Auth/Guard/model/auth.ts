// Interfaces for the authentication response structure

import {AUTH_TOKEN_KEY} from '../AuthConstants.ts';

export interface AuthUser {
  id: string;
}

export interface AuthToken {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_at: number;
  user: AuthUser;
}

export interface AuthResponse {
  message: AuthToken;
  success: boolean;
}

export const findAuthToken = (): AuthToken | null => {
    const raw = localStorage.getItem(AUTH_TOKEN_KEY);
    if (!raw) return null;
    try {
        return JSON.parse(raw) as AuthToken;
    } catch {
        return null;
    }
}

export const saveAuthToken = (token: AuthToken | null): void => {
  if (token) {
    localStorage.setItem(AUTH_TOKEN_KEY, JSON.stringify(token));
  } else {
    localStorage.removeItem(AUTH_TOKEN_KEY);
  }
};

export function logoutUser() {
  localStorage.removeItem(AUTH_TOKEN_KEY);
  window.location.href = '/login';
}
