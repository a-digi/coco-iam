import React, { useEffect } from 'react';
import { useAuth } from '../Guard/useAuth';
import { useNavigate } from 'react-router-dom';

const Logout: React.FC = () => {
  const { logout } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    logout();
    navigate('/login', { replace: true });
  }, [logout, navigate]);

  return (
    <div className="flex items-center justify-center min-h-screen">
      <div className="text-lg text-gray-700">Logging out...</div>
    </div>
  );
};

export default Logout;
