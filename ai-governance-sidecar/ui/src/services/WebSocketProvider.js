import React, { createContext, useContext, useEffect, useState, useCallback } from 'react';

const WebSocketContext = createContext(null);

export const useWebSocket = () => {
  const context = useContext(WebSocketContext);
  if (!context) {
    throw new Error('useWebSocket must be used within WebSocketProvider');
  }
  return context;
};

export const WebSocketProvider = ({ children }) => {
  const [ws, setWs] = useState(null);
  const [isConnected, setIsConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState(null);
  const [listeners, setListeners] = useState([]);

  const WS_URL = process.env.REACT_APP_WS_URL || 'ws://localhost:8080/ws';

  // Connect to WebSocket
  const connect = useCallback(() => {
    try {
      const websocket = new WebSocket(WS_URL);

      websocket.onopen = () => {
        console.log('WebSocket connected');
        setIsConnected(true);
      };

      websocket.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data);
          console.log('WebSocket message:', message);
          setLastMessage(message);
          
          // Notify all listeners
          listeners.forEach(listener => listener(message));
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error);
        }
      };

      websocket.onerror = (error) => {
        console.error('WebSocket error:', error);
      };

      websocket.onclose = () => {
        console.log('WebSocket disconnected');
        setIsConnected(false);
        
        // Attempt to reconnect after 5 seconds
        setTimeout(() => {
          console.log('Attempting to reconnect...');
          connect();
        }, 5000);
      };

      setWs(websocket);
    } catch (error) {
      console.error('Failed to create WebSocket:', error);
    }
  }, [WS_URL, listeners]);

  // Subscribe to WebSocket messages
  const subscribe = useCallback((callback) => {
    setListeners(prev => [...prev, callback]);
    
    // Return unsubscribe function
    return () => {
      setListeners(prev => prev.filter(listener => listener !== callback));
    };
  }, []);

  // Initialize WebSocket connection
  useEffect(() => {
    connect();

    // Cleanup on unmount
    return () => {
      if (ws) {
        ws.close();
      }
    };
  }, [connect]);

  const value = {
    isConnected,
    lastMessage,
    subscribe,
  };

  return (
    <WebSocketContext.Provider value={value}>
      {children}
    </WebSocketContext.Provider>
  );
};