import React, { createContext, useContext, useEffect, useState } from 'react';
import api, { authAPI } from '../services/api';

const AuthContext = createContext(null);

export const useAuth = () => {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
};

export const AuthProvider = ({ children }) => {
  const [token, setToken] = useState(() => localStorage.getItem('auth_token') || '');
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // Keep axios instance in sync with the token (helps even before interceptor runs)
  useEffect(() => {
    if (token) {
      localStorage.setItem('auth_token', token);
      api.defaults.headers.common.Authorization = `Bearer ${token}`;
    } else {
      localStorage.removeItem('auth_token');
      delete api.defaults.headers.common.Authorization;
    }
  }, [token]);

  // On mount (or token change), load current user if we have a token
  useEffect(() => {
    const load = async () => {
      if (!token) {
        setUser(null);
        setLoading(false);
        return;
      }
      try {
        const me = await authAPI.me(); // GET /me with Authorization header
        setUser(me);
      } catch (e) {
        // bad/expired token — clear it
        setToken('');
        setUser(null);
      } finally {
        setLoading(false);
      }
    };
    load();
  }, [token]);

  const login = async (email, password) => {
    setError(null);
    try {
      // POST /login — returns { token, user }
      const { token: newToken, user: userInfo } = await authAPI.login(email, password);

      // set token first so subsequent calls include it
      setToken(newToken);
      setUser(userInfo); // we already have the user; no need to refetch immediately

      return { success: true };
    } catch (e) {
      const msg = e?.response?.data?.error || 'Login failed';
      setError(msg);
      return { success: false, error: msg };
    }
  };

  const logout = () => {
    setToken('');
    setUser(null);
    setError(null);
  };

  const hasRole = (role) => Boolean(user?.roles?.includes(role));
  const canApprove = () => hasRole('admin') || hasRole('approver');

  return (
    <AuthContext.Provider
      value={{
        token,
        user,
        loading,
        error,
        login,
        logout,
        hasRole,
        canApprove,
        isAuthenticated: !!user,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};