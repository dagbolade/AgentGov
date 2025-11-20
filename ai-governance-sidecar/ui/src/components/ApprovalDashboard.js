import React, { useState, useEffect, useCallback } from 'react';
import { RefreshCw, AlertCircle, CheckCircle } from 'lucide-react';
import { approvalAPI } from '../services/api';
import { useWebSocket } from '../services/WebSocketProvider';
import ApprovalCard from './ApprovalCard';

const ApprovalDashboard = () => {
  const [approvals, setApprovals] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [refreshing, setRefreshing] = useState(false);
  const { isConnected, subscribe } = useWebSocket();

  // Fetch pending approvals
  const fetchApprovals = useCallback(async () => {
    try {
      setError(null);
      const data = await approvalAPI.getPending();
      setApprovals(data.approvals || []);
    } catch (err) {
      console.error('Failed to fetch approvals:', err);
      setError('Failed to load pending approvals. Please try again.');
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

  // Handle approve action
  const handleApprove = async (approvalId, approver, comment) => {
    try {
      await approvalAPI.approve(approvalId, approver, comment);
      // Remove from list immediately (optimistic update)
      setApprovals(prev => prev.filter(a => a.approval_id !== approvalId));
    } catch (err) {
      console.error('Failed to approve:', err);
      alert('Failed to approve request. Please try again.');
      // Refresh to get current state
      fetchApprovals();
    }
  };

  // Handle deny action
  const handleDeny = async (approvalId, approver, comment) => {
    try {
      await approvalAPI.deny(approvalId, approver, comment);
      // Remove from list immediately (optimistic update)
      setApprovals(prev => prev.filter(a => a.approval_id !== approvalId));
    } catch (err) {
      console.error('Failed to deny:', err);
      alert('Failed to deny request. Please try again.');
      // Refresh to get current state
      fetchApprovals();
    }
  };

  // Initial fetch
  useEffect(() => {
    fetchApprovals();
  }, [fetchApprovals]);

  // Subscribe to WebSocket updates
  useEffect(() => {
    const unsubscribe = subscribe((message) => {
      console.log('Received WebSocket update:', message);
      
      // Handle snapshot (initial state or full refresh)
      if (message.type === 'snapshot' && Array.isArray(message.approvals)) {
        setApprovals(message.approvals);
        return;
      }
      
      // Handle decision (approval/denial)
      if (message.type === 'decision' && message.approval_id) {
        // Remove the decided approval from the list
        setApprovals(prev => prev.filter(a => a.approval_id !== message.approval_id));
        return;
      }
    });

    return unsubscribe;
  }, [subscribe]);

  if (loading) {
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
            <h2 className="text-2xl font-bold text-gray-900">
              Pending Approvals
            </h2>
            <p className="text-sm text-gray-500 mt-1">
              {approvals.length} request{approvals.length !== 1 ? 's' : ''} awaiting review
            </p>
          </div>
          <div className="flex items-center space-x-4">
            {/* WebSocket status */}
            <div className="flex items-center space-x-2">
              {isConnected ? (
                <>
                  <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
                  <span className="text-sm text-gray-600">Live</span>
                </>
              ) : (
                <>
                  <div className="w-2 h-2 bg-red-500 rounded-full" />
                  <span className="text-sm text-gray-600">Disconnected</span>
                </>
              )}
            </div>
            
            {/* Refresh button */}
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