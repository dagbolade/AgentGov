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

// Log API errors (you already had this)
api.interceptors.response.use(
  (res) => res,
  (err) => {
    console.error('API Error:', err);
    return Promise.reject(err);
  }
);

export const authAPI = {
  login: async (email, password) => {
    // send both keys to cover servers that expect username
    const { data } = await api.post('/login', { email, username: email, password });
    return data; // { token, user }
  },
  me: async () => {
    const { data } = await api.get('/me');
    return data; // current user
  },
};

export const approvalAPI = {
  getPending: async () => {
    try {
      const { data } = await api.get('/approvals', { params: { status: 'pending' } });
      return data;
    } catch (err) {
      if (err.response?.status === 404) {
        // API doesn’t expose /approvals/pending; try query or return empty
        try {
          const { data } = await api.get('/approvals', { params: { status: 'pending' } });
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
      params: { limit, offset },
    });
    return response.data;
  },
};

export const healthAPI = {
  // Check system health
  getHealth: async () => {
    const response = await api.get('/health');
    return response.data;
  },
};

export default api;