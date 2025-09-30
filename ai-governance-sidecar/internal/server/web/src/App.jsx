import React, { useEffect, useState } from 'react';
import PendingApprovals from './PendingApprovals';
import AuditLog from './AuditLog';
import './App.css';

export default function App() {
  const [ws, setWs] = useState(null);
  const [pending, setPending] = useState([]);
  const [audit, setAudit] = useState([]);

  useEffect(() => {
    // Connect to WebSocket for real-time updates
    const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const socket = new WebSocket(wsProtocol + '//' + window.location.host + '/ws');
    
    socket.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      if (msg.type === 'pending_update') {
        setPending(msg.pending || []);
      }
    };
    
    socket.onopen = () => {
      console.log('WebSocket connected');
    };
    
    socket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
    
    setWs(socket);
    
    // Fetch initial data
    fetchPending();
    fetchAudit();
    
    return () => socket.close();
  }, []);

  const fetchPending = async () => {
    try {
      const response = await fetch('/pending');
      const data = await response.json();
      setPending(data.pending || []);
    } catch (error) {
      console.error('Failed to fetch pending approvals:', error);
    }
  };

  const fetchAudit = async () => {
    try {
      const response = await fetch('/audit');
      const data = await response.json();
      setAudit(data.entries || []);
    } catch (error) {
      console.error('Failed to fetch audit log:', error);
    }
  };

  return (
    <div className="container">
      <h1>AI Governance Dashboard</h1>
      <div className="dashboard">
        <PendingApprovals approvals={pending} onUpdate={fetchPending} />
        <AuditLog logs={audit} />
      </div>
    </div>
  );
}
