import React, { useState, useEffect, useCallback } from 'react';
import { format } from 'date-fns';
import { 
  RefreshCw, 
  CheckCircle, 
  XCircle, 
  AlertCircle,
  ChevronDown,
  ChevronUp,
  Activity // Added Activity icon for "Live" status
} from 'lucide-react';
import { auditAPI } from '../services/api';
import { useWebSocket } from '../services/WebSocketProvider'; // <--- 1. Import WebSocket

const AuditLog = () => {
  const [entries, setEntries] = useState([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [expandedEntry, setExpandedEntry] = useState(null);
  
  // 2. Get the subscribe function and connection status
  const { subscribe, isConnected } = useWebSocket();

  // Define fetch as useCallback so we can use it inside useEffect
  const fetchAuditLog = useCallback(async (isAutoRefresh = false) => {
    try {
      if (!isAutoRefresh) setLoading(true); // Don't show full spinner on auto-update
      const data = await auditAPI.getAuditLog(50, 0);
      setEntries(data.entries || []);
    } catch (err) {
      console.error('Failed to fetch audit log:', err);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  // Initial load
  useEffect(() => {
    fetchAuditLog();
  }, [fetchAuditLog]);

  // 3. LISTEN FOR REAL-TIME UPDATES
  useEffect(() => {
    // Subscribe to the websocket
    const unsubscribe = subscribe((message) => {
      // Whenever the backend announces a change (approval_update), refresh the log
      if (message.type === 'approval_update') {
        fetchAuditLog(true); // Pass true to skip the loading spinner
      }
    });

    // Cleanup listener on unmount
    return unsubscribe;
  }, [subscribe, fetchAuditLog]);

  const handleRefresh = () => {
    setRefreshing(true);
    fetchAuditLog();
  };

  const getStatusIcon = (status) => {
    const normalized = status?.toLowerCase();
    if (normalized === 'allow' || normalized === 'approved') return <CheckCircle className="w-5 h-5 text-green-500" />;
    if (normalized === 'deny' || normalized === 'denied') return <XCircle className="w-5 h-5 text-red-500" />;
    return <AlertCircle className="w-5 h-5 text-gray-500" />;
  };

  const getStatusColor = (status) => {
    const normalized = status?.toLowerCase();
    if (normalized === 'allow' || normalized === 'approved') return 'bg-green-100 text-green-800';
    if (normalized === 'deny' || normalized === 'denied') return 'bg-red-100 text-red-800';
    return 'bg-gray-100 text-gray-800';
  };

  if (loading && entries.length === 0) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <RefreshCw className="w-8 h-8 animate-spin text-blue-500 mx-auto mb-4" />
          <p className="text-gray-600">Loading audit log...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="bg-white rounded-lg shadow p-6 flex items-center justify-between">
          <div>
            <div className="flex items-center space-x-3">
              <h2 className="text-2xl font-bold text-gray-900">Audit Log</h2>
              {/* Live Status Indicator */}
              {isConnected && (
                <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 animate-pulse">
                  <Activity className="w-3 h-3 mr-1" />
                  Live
                </span>
              )}
            </div>
            <p className="text-sm text-gray-500 mt-1">
              {entries.length} total entr{entries.length !== 1 ? 'ies' : 'y'}
            </p>
          </div>
          <button
            onClick={handleRefresh}
            disabled={refreshing}
            className="flex items-center space-x-2 px-4 py-2 bg-blue-50 text-blue-600 rounded-lg hover:bg-blue-100 transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
            <span>Refresh</span>
          </button>
      </div>

      {entries.length === 0 ? (
        <div className="bg-white rounded-lg shadow p-12 text-center">
          <AlertCircle className="w-16 h-16 text-gray-400 mx-auto mb-4" />
          <h3 className="text-xl font-semibold text-gray-900 mb-2">No audit entries</h3>
          <p className="text-gray-500">Waiting for agent activity...</p>
        </div>
      ) : (
        <div className="bg-white rounded-lg shadow overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Tool/Action</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Timestamp</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Approver</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Details</th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {entries.map((entry, index) => {
                const isExpanded = expandedEntry === index;
                // Robust access to tool name
                const toolName = entry.tool_input?.tool_name || entry.request?.tool || 'Unknown';
                const toolAction = entry.tool_input?.args?.action || entry.request?.action || 'Called';
                
                return (
                  <React.Fragment key={entry.id || index}>
                    <tr className="hover:bg-gray-50 transition-colors duration-150">
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="flex items-center space-x-2">
                          {getStatusIcon(entry.decision)}
                          <span className={`px-2 py-1 text-xs font-medium rounded-full ${getStatusColor(entry.decision)}`}>
                            {entry.decision?.toUpperCase()}
                          </span>
                        </div>
                      </td>
                      <td className="px-6 py-4">
                        <div className="text-sm font-medium text-gray-900">{toolName}</div>
                        <div className="text-sm text-gray-500">{toolAction}</div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm text-gray-900">
                          {format(new Date(entry.timestamp), 'MMM d, yyyy')}
                        </div>
                        <div className="text-sm text-gray-500">
                          {format(new Date(entry.timestamp), 'h:mm:ss a')}
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm text-gray-900">
                          {entry.approver || 'System'}
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <button
                          onClick={() => setExpandedEntry(isExpanded ? null : index)}
                          className="text-blue-600 hover:text-blue-800 flex items-center space-x-1"
                        >
                          {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                          <span className="text-sm">{isExpanded ? 'Hide' : 'Show'}</span>
                        </button>
                      </td>
                    </tr>
                    
                    {isExpanded && (
                      <tr>
                        <td colSpan="5" className="px-6 py-4 bg-gray-50 animate-fadeIn">
                          <div className="space-y-4">
                            {entry.reason && (
                              <div>
                                <h4 className="text-sm font-semibold text-gray-700 mb-1">Reason</h4>
                                <p className="text-sm text-gray-600">{entry.reason}</p>
                              </div>
                            )}
                            <div>
                              <h4 className="text-sm font-semibold text-gray-700 mb-2">Full Input</h4>
                              <pre className="text-xs text-gray-600 bg-white p-4 rounded border border-gray-200 overflow-auto">
                                {JSON.stringify(entry.tool_input || entry.request, null, 2)}
                              </pre>
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </React.Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

export default AuditLog;