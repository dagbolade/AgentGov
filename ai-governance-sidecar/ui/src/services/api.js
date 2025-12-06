import axios from 'axios';

const API_BASE_URL =
  process.env.REACT_APP_API_URL ||
  process.env.REACT_APP_API_BASE_URL ||
  '/api';      

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: { 'Content-Type': 'application/json' },
});

// Attach JWT on every request
api.interceptors.request.use((config) => {
  const tok = localStorage.getItem('auth_token');
  if (tok) {
    config.headers.Authorization = `Bearer ${tok}`;
  }
  return config;
});

// Log API errors
api.interceptors.response.use(
  (res) => res,
  (err) => {
    console.error('API Error:', err);
    return Promise.reject(err);
  }
);

export const authAPI = {
  login: async (email, password) => {
    const { data } = await api.post('/login', { email, username: email, password });
    return data; // { token, user }
  },
  me: async () => {
    // Add cache buster to ensure fresh user data
    const { data } = await api.get('/me', { params: { _t: Date.now() } });
    return data; // current user
  },
};

export const approvalAPI = {
  getPending: async () => {
    try {
      // FIX: Add timestamp (_t) to prevent browser caching
      const { data } = await api.get('/approvals', { 
        params: { 
          status: 'pending',
          _t: Date.now() 
        } 
      });
      return data;
    } catch (err) {
      if (err.response?.status === 404) {
        // Fallback for some proxies
        try {
          const { data } = await api.get('/approvals', { 
            params: { 
              status: 'pending',
              _t: Date.now() 
            } 
          });
          return data;
        } catch (e2) {
          if (e2.response?.status === 404) return [];
          throw e2;
        }
      }
      throw err;
    }
  },
  // Approve a request
  approve: async (id, approver, comment) =>
    (await api.post(`/approvals/${id}/approve`, { approver, comment })).data,
  // Deny a request
  deny: async (id, approver, comment) =>
    (await api.post(`/approvals/${id}/deny`, { approver, comment })).data,
};

export const auditAPI = {
  // Get audit log entries
  getAuditLog: async (limit = 50, offset = 0) => {
    const response = await api.get('/audit', {
      params: { 
        limit, 
        offset,
        _t: Date.now() // Prevent caching for audit log too
      },
    });
    return response.data;
  },
};

export const healthAPI = {
  // Check system health
  getHealth: async () => {
    const response = await api.get('/health', { params: { _t: Date.now() } });
    return response.data;
  },
};

export default api;