import React from 'react';
import './AuditLog.css';

export default function AuditLog({ logs }) {
  return (
    <div className="audit-log">
      <h2>Audit Log</h2>
      {logs.length === 0 ? <p>No audit entries.</p> : (
        <table>
          <thead>
            <tr>
              <th>Time</th>
              <th>Tool</th>
              <th>Action</th>
              <th>Status</th>
              <th>Risk</th>
            </tr>
          </thead>
          <tbody>
            {logs.map((log, i) => (
              <tr key={i}>
                <td>{log.timestamp ? new Date(log.timestamp).toLocaleString() : 'N/A'}</td>
                <td>{log.tool_name || 'Unknown'}</td>
                <td>{log.action || log.request || 'N/A'}</td>
                <td className={`status-${log.status || 'unknown'}`}>
                  {log.status || 'Unknown'}
                </td>
                <td className={`risk-${log.risk_level || 'unknown'}`}>
                  {log.risk_level || 'Unknown'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
