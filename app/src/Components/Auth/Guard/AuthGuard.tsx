import React from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from './useAuth';
import { parseJwt } from '../../../config/security/jtw';
import { AppScopes } from '../../../config/security/scopes';

interface AuthGuardProps {
  children: React.ReactNode;
  accessScopes?: string[];
}

const AuthGuard: React.FC<AuthGuardProps> = ({ children, accessScopes }) => {
  const location = useLocation();
  const { authenticated, authToken } = useAuth();

  if (!authenticated || !authToken?.access_token) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  if (accessScopes && accessScopes.length > 0) {
    const payload = parseJwt(authToken.access_token);
    const userScopes: string[] = payload && Array.isArray(payload.scopes) ? payload.scopes : [];

    const allowsUserMe = accessScopes.includes(AppScopes.UserMe);
    const hasStandardAccess = accessScopes.some(scope => userScopes.includes(scope));
    const isSuperAdmin = userScopes.includes(AppScopes.SuperAdmin);

    if (!isSuperAdmin && !allowsUserMe && !hasStandardAccess) {
      return <Navigate to="/" replace />;
    }
  }

  return <>{children}</>;
};

export default AuthGuard;
