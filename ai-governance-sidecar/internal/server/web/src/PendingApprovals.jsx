import React, { useState } from 'react';
import './PendingApprovals.css';

export default function PendingApprovals({ approvals, onUpdate }) {
  const [reason, setReason] = useState({});
  const [loading, setLoading] = useState({});

  const handleAction = async (id, approved) => {
    const reasonText = reason[id] || '';
    if (!reasonText.trim()) {
      alert('Please provide a reason for your decision');
      return;
    }

    setLoading(prev => ({ ...prev, [id]: true }));

    try {
      const response = await fetch(`/approve/${id}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          approved,
          reason: reasonText,
          decided_by: 'UI User', // Could be made configurable
        }),
      });

      if (response.ok) {
        setReason(r => ({ ...r, [id]: '' }));
        onUpdate(); // Refresh the pending list
      } else {
        const error = await response.json();
        alert(`Error: ${error.error}`);
      }
    } catch (error) {
      console.error('Failed to process approval:', error);
      alert('Failed to process approval');
    } finally {
      setLoading(prev => ({ ...prev, [id]: false }));
    }
  };

  return (
    <div className="pending-approvals">
      <h2>Pending Approvals</h2>
      {approvals.length === 0 ? <p>No pending approvals.</p> : (
        <ul>
          {approvals.map(item => (
            <li key={item.id} className="approval-item">
              <div className="approval-info">
                <strong>{item.tool_name || item.id}</strong>
                <p><strong>Request:</strong> {item.request || 'No details available'}</p>
                <p><strong>Risk Level:</strong> {item.risk_level || 'Unknown'}</p>
                {item.timestamp && <p><strong>Requested:</strong> {new Date(item.timestamp).toLocaleString()}</p>}
              </div>
              <div className="approval-actions">
                <input
                  type="text"
                  placeholder="Reason (required)"
                  value={reason[item.id] || ''}
                  onChange={e => setReason(r => ({ ...r, [item.id]: e.target.value }))}
                  disabled={loading[item.id]}
                />
                <div className="buttons">
                  <button 
                    className="approve" 
                    onClick={() => handleAction(item.id, true)}
                    disabled={loading[item.id]}
                  >
                    {loading[item.id] ? 'Processing...' : 'Approve'}
                  </button>
                  <button 
                    className="deny" 
                    onClick={() => handleAction(item.id, false)}
                    disabled={loading[item.id]}
                  >
                    {loading[item.id] ? 'Processing...' : 'Deny'}
                  </button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
