import React, { type ReactElement } from 'react';
import { useAuth } from '../../../Components/Auth/Guard/useAuth';
import { parseJwt } from '../../../config/security/jtw';
import { AppScopes } from '../../../config/security/scopes';

export interface ScopeAccessAware {
  accessMe: boolean;
}

export interface ScopeBasedComponentAccessProps {
  requiredScopes: string[];
  children: ReactElement<Partial<ScopeAccessAware>>;
}

export const ScopeBasedComponentAccess: React.FC<ScopeBasedComponentAccessProps> = ({
  requiredScopes,
  children,
}) => {
  const { authToken } = useAuth();

  if (!authToken || !authToken.access_token) {
    return null;
  }

  const payload = parseJwt(authToken.access_token);

  if (!payload) {
    return null;
  }

  const userScopes: string[] = Array.isArray(payload.scopes) ? payload.scopes : [];

  if (requiredScopes.length === 0) {
    return React.cloneElement(children, { accessMe: false });
  }

  const hasFullAccess = requiredScopes
    .filter((scope) => scope !== AppScopes.UserMe)
    .some((scope) => userScopes.includes(scope));

  const allowsUserMe = requiredScopes.includes(AppScopes.UserMe);

  if (hasFullAccess) {
    return React.cloneElement(children, { accessMe: false });
  }

  if (allowsUserMe) {
    return React.cloneElement(children, { accessMe: true });
  }

  return null;
};

export default ScopeBasedComponentAccess;
