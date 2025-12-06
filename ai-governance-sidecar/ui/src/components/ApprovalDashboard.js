import React, { useState, useEffect, useCallback } from 'react';
import { RefreshCw, CheckCircle, Activity, AlertCircle } from 'lucide-react';
import { approvalAPI } from '../services/api';
import { useWebSocket } from '../services/WebSocketProvider';
import ApprovalCard from './ApprovalCard';

const ApprovalDashboard = () => {
  const [approvals, setApprovals] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [refreshing, setRefreshing] = useState(false);
  const [lastUpdated, setLastUpdated] = useState(new Date());
  
  // 1. Get WebSocket connection
  const { isConnected, subscribe } = useWebSocket();

  // Fetch pending approvals
  const fetchApprovals = useCallback(async (isAutoRefresh = false) => {
    try {
      // Don't clear error if just background refreshing
      if (!isAutoRefresh) setError(null);
      
      const data = await approvalAPI.getPending();
      
      // Update state
      setApprovals(data.approvals || []);
      setLastUpdated(new Date());
      
    } catch (err) {
      console.error('Failed to fetch approvals:', err);
      if (!isAutoRefresh) setError('Failed to load pending approvals.');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  // Handle manual refresh
  const handleRefresh = () => {
    setRefreshing(true);
    fetchApprovals();
  };

  // Handle approve action (Optimistic UI)
  const handleApprove = async (approvalId, approver, comment) => {
    try {
      setApprovals(prev => prev.filter(a => a.approval_id !== approvalId));
      await approvalAPI.approve(approvalId, approver, comment);
    } catch (err) {
      console.error('Failed to approve:', err);
      fetchApprovals(); // Revert on error
    }
  };

  // Handle deny action (Optimistic UI)
  const handleDeny = async (approvalId, approver, comment) => {
    try {
      setApprovals(prev => prev.filter(a => a.approval_id !== approvalId));
      await approvalAPI.deny(approvalId, approver, comment);
    } catch (err) {
      console.error('Failed to deny:', err);
      fetchApprovals(); // Revert on error
    }
  };

  // Initial fetch
  useEffect(() => {
    fetchApprovals();
  }, [fetchApprovals]);

  // 2. REAL-TIME LISTENER (PUSH)
  useEffect(() => {
    const unsubscribe = subscribe((message) => {
      // If ANY approval update happens, refresh the list immediately
      if (message.type === 'approval_update') {
        console.log('WS Update received, refreshing list...');
        fetchApprovals(true);
      }
    });
    return unsubscribe;
  }, [subscribe, fetchApprovals]);

  // 3. FAIL-SAFE HEARTBEAT (PULL)
  // Polls every 5 seconds to ensure UI never drifts from reality
  useEffect(() => {
    const interval = setInterval(() => {
      if (isConnected) {
        fetchApprovals(true);
      }
    }, 5000); 

    return () => clearInterval(interval);
  }, [isConnected, fetchApprovals]);

  if (loading && approvals.length === 0) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <RefreshCw className="w-8 h-8 animate-spin text-blue-500 mx-auto mb-4" />
          <p className="text-gray-600">Loading approvals...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header with stats */}
      <div className="bg-white rounded-lg shadow p-6">
        <div className="flex items-center justify-between">
          <div>
            <div className="flex items-center space-x-3">
              <h2 className="text-2xl font-bold text-gray-900">
                Pending Approvals
              </h2>
              {/* Live Status Indicator */}
              {isConnected ? (
                <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 animate-pulse">
                  <Activity className="w-3 h-3 mr-1" />
                  Live
                </span>
              ) : (
                 <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-600">
                  Offline
                </span>
              )}
            </div>
            <p className="text-sm text-gray-500 mt-1">
              {approvals.length} request{approvals.length !== 1 ? 's' : ''} awaiting review
              <span className="text-xs text-gray-400 ml-2">
                (Last sync: {lastUpdated.toLocaleTimeString()})
              </span>
            </p>
          </div>
          <div className="flex items-center space-x-4">
            <button
              onClick={handleRefresh}
              disabled={refreshing}
              className="flex items-center space-x-2 px-4 py-2 bg-blue-50 text-blue-600 rounded-lg hover:bg-blue-100 transition-colors disabled:opacity-50"
            >
              <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
              <span>Refresh</span>
            </button>
          </div>
        </div>
      </div>

      {/* Error message */}
      {error && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 flex items-start space-x-3">
          <AlertCircle className="w-5 h-5 text-red-500 mt-0.5" />
          <div>
            <p className="text-red-800 font-medium">Error</p>
            <p className="text-red-600 text-sm mt-1">{error}</p>
          </div>
        </div>
      )}

      {/* Approvals list */}
      {approvals.length === 0 ? (
        <div className="bg-white rounded-lg shadow p-12 text-center">
          <CheckCircle className="w-16 h-16 text-green-500 mx-auto mb-4" />
          <h3 className="text-xl font-semibold text-gray-900 mb-2">
            All caught up!
          </h3>
          <p className="text-gray-600">
            No pending approvals at the moment.
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {approvals.map((approval) => (
            <ApprovalCard
              key={approval.approval_id}
              approval={approval}
              onApprove={handleApprove}
              onDeny={handleDeny}
            />
          ))}
        </div>
      )}
    </div>
  );
};

export default ApprovalDashboard;