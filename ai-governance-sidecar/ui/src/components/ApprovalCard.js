import React, { useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { 
  AlertTriangle, 
  Clock, 
  CheckCircle, 
  XCircle, 
  ChevronDown, 
  ChevronUp 
} from 'lucide-react';

const ApprovalCard = ({ approval, onApprove, onDeny }) => {
  const [expanded, setExpanded] = useState(false);
  const [approving, setApproving] = useState(false);
  const [denying, setDenying] = useState(false);
  const [showApproveDialog, setShowApproveDialog] = useState(false);
  const [showDenyDialog, setShowDenyDialog] = useState(false);
  const [approver, setApprover] = useState('');
  const [comment, setComment] = useState('');

  const handleApprove = async () => {
    if (!approver.trim()) {
      alert('Please enter your name or email');
      return;
    }

    setApproving(true);
    try {
      await onApprove(approval.approval_id, approver, comment);
      setShowApproveDialog(false);
    } catch (err) {
      console.error('Approval failed:', err);
    } finally {
      setApproving(false);
    }
  };

  const handleDeny = async () => {
    if (!approver.trim()) {
      alert('Please enter your name or email');
      return;
    }

    if (!comment.trim()) {
      alert('Please provide a reason for denial');
      return;
    }

    setDenying(true);
    try {
      await onDeny(approval.approval_id, approver, comment);
      setShowDenyDialog(false);
    } catch (err) {
      console.error('Denial failed:', err);
    } finally {
      setDenying(false);
    }
  };

  const getTimeRemaining = () => {
    if (!approval.expires_at) return null;
    const expiresAt = new Date(approval.expires_at);
    const now = new Date();
    const remaining = expiresAt - now;
    
    if (remaining < 0) {
      return { text: 'Expired', urgent: true };
    }
    
    const minutes = Math.floor(remaining / 60000);
    if (minutes < 10) {
      return { text: `${minutes}m remaining`, urgent: true };
    }
    
    return { 
      text: formatDistanceToNow(expiresAt, { addSuffix: true }), 
      urgent: false 
    };
  };

  const timeRemaining = getTimeRemaining();

  return (
    <div className="bg-white rounded-lg shadow-md border border-gray-200 hover:shadow-lg transition-shadow">
      {/* Card Header */}
      <div className="p-6">
        <div className="flex items-start justify-between">
          <div className="flex-1">
            <div className="flex items-center space-x-3 mb-2">
              <AlertTriangle className="w-5 h-5 text-yellow-500" />
              <h3 className="text-lg font-semibold text-gray-900">
                {approval.request?.tool || 'Unknown Tool'} - {approval.request?.action || 'Unknown Action'}
              </h3>
            </div>
            
            <div className="flex items-center space-x-4 text-sm text-gray-500 mb-4">
              <span className="flex items-center">
                <Clock className="w-4 h-4 mr-1" />
                {formatDistanceToNow(new Date(approval.created_at), { addSuffix: true })}
              </span>
              {timeRemaining && (
                <span className={`flex items-center ${timeRemaining.urgent ? 'text-red-600 font-medium' : ''}`}>
                  <AlertTriangle className="w-4 h-4 mr-1" />
                  {timeRemaining.text}
                </span>
              )}
            </div>

            <p className="text-gray-700 mb-4">
              <span className="font-medium">Reason:</span> {approval.reason || 'No reason provided'}
            </p>

            {approval.confidence && (
              <div className="mb-4">
                <span className="text-sm text-gray-600">Confidence: </span>
                <span className="text-sm font-medium">
                  {(approval.confidence * 100).toFixed(0)}%
                </span>
                <div className="w-full bg-gray-200 rounded-full h-2 mt-1">
                  <div 
                    className="bg-blue-500 h-2 rounded-full"
                    style={{ width: `${approval.confidence * 100}%` }}
                  />
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Request Details (Collapsible) */}
        <div className="mt-4">
          <button
            onClick={() => setExpanded(!expanded)}
            className="flex items-center space-x-2 text-sm text-blue-600 hover:text-blue-800"
          >
            {expanded ? (
              <>
                <ChevronUp className="w-4 h-4" />
                <span>Hide details</span>
              </>
            ) : (
              <>
                <ChevronDown className="w-4 h-4" />
                <span>Show details</span>
              </>
            )}
          </button>

          {expanded && (
            <div className="mt-4 p-4 bg-gray-50 rounded-lg">
              <h4 className="text-sm font-semibold text-gray-700 mb-2">Request Details</h4>
              <pre className="text-xs text-gray-600 overflow-auto">
                {JSON.stringify(approval.request, null, 2)}
              </pre>
            </div>
          )}
        </div>

        {/* Action Buttons */}
        <div className="flex items-center space-x-3 mt-6">
          <button
            onClick={() => setShowApproveDialog(true)}
            className="flex items-center space-x-2 px-6 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors"
          >
            <CheckCircle className="w-4 h-4" />
            <span>Approve</span>
          </button>
          
          <button
            onClick={() => setShowDenyDialog(true)}
            className="flex items-center space-x-2 px-6 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
          >
            <XCircle className="w-4 h-4" />
            <span>Deny</span>
          </button>
        </div>
      </div>

      {/* Approve Dialog */}
      {showApproveDialog && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-xl p-6 max-w-md w-full mx-4">
            <h3 className="text-xl font-semibold text-gray-900 mb-4">
              Approve Request
            </h3>
            
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Your Name or Email *
                </label>
                <input
                  type="text"
                  value={approver}
                  onChange={(e) => setApprover(e.target.value)}
                  placeholder="john@example.com"
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Comment (optional)
                </label>
                <textarea
                  value={comment}
                  onChange={(e) => setComment(e.target.value)}
                  placeholder="Add any comments..."
                  rows={3}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent"
                />
              </div>
            </div>

            <div className="flex items-center space-x-3 mt-6">
              <button
                onClick={handleApprove}
                disabled={approving}
                className="flex-1 px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors disabled:opacity-50"
              >
                {approving ? 'Approving...' : 'Confirm Approval'}
              </button>
              <button
                onClick={() => setShowApproveDialog(false)}
                disabled={approving}
                className="flex-1 px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Deny Dialog */}
      {showDenyDialog && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-xl p-6 max-w-md w-full mx-4">
            <h3 className="text-xl font-semibold text-gray-900 mb-4">
              Deny Request
            </h3>
            
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Your Name or Email *
                </label>
                <input
                  type="text"
                  value={approver}
                  onChange={(e) => setApprover(e.target.value)}
                  placeholder="john@example.com"
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-red-500 focus:border-transparent"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Reason for Denial *
                </label>
                <textarea
                  value={comment}
                  onChange={(e) => setComment(e.target.value)}
                  placeholder="Explain why this request is being denied..."
                  rows={3}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-red-500 focus:border-transparent"
                />
              </div>
            </div>

            <div className="flex items-center space-x-3 mt-6">
              <button
                onClick={handleDeny}
                disabled={denying}
                className="flex-1 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors disabled:opacity-50"
              >
                {denying ? 'Denying...' : 'Confirm Denial'}
              </button>
              <button
                onClick={() => setShowDenyDialog(false)}
                disabled={denying}
                className="flex-1 px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default ApprovalCard;