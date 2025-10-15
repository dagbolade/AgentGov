import axios from 'axios';

const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add request interceptor for error handling
api.interceptors.response.use(
  response => response,
  error => {
    console.error('API Error:', error);
    return Promise.reject(error);
  }
);

export const approvalAPI = {
  // Get pending approvals
  getPending: async () => {
    const response = await api.get('/approvals/pending');
    return response.data;
  },

  // Approve a request
  approve: async (approvalId, approver, comment) => {
    const response = await api.post(`/approvals/${approvalId}/approve`, {
      approver,
      comment,
    });
    return response.data;
  },

  // Deny a request
  deny: async (approvalId, approver, comment) => {
    const response = await api.post(`/approvals/${approvalId}/deny`, {
      approver,
      comment,
    });
    return response.data;
  },
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