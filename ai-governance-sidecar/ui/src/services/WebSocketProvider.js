import React, { createContext, useContext, useEffect, useRef, useState, useCallback } from 'react';
import { useAuth } from '../contexts/AuthContext';

const WebSocketContext = createContext(null);

export const useWebSocket = () => {
  const ctx = useContext(WebSocketContext);
  if (!ctx) throw new Error('useWebSocket must be used within WebSocketProvider');
  return ctx;
};

export const WebSocketProvider = ({ children }) => {
  const { token } = useAuth();

  const wsRef = useRef(null);
  const reconnectTimerRef = useRef(null);
  const tokenRef = useRef(token);
  const listenersRef = useRef(new Set());

  const [isConnected, setIsConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState(null);

  useEffect(() => { tokenRef.current = token; }, [token]);

  const API_BASE =
    process.env.REACT_APP_API_URL ||
    process.env.REACT_APP_API_BASE_URL ||
    '/api';

  const WS_BASE =
    process.env.REACT_APP_WS_URL ||
    (API_BASE.startsWith('http')
      ? API_BASE.replace(/^http/, 'ws').replace(/\/$/, '') + '/ws'
      : '/api/ws');

  const clearReconnect = () => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
  };

  const scheduleReconnect = useCallback(() => {
    clearReconnect();
    // 6–10s with jitter to avoid thundering herd
    const base = 6000;
    const jitter = Math.floor(Math.random() * 4000);
    reconnectTimerRef.current = setTimeout(() => {
      const tok = tokenRef.current;
      if (!tok) return; // don’t connect without a token
      // if an existing socket is alive, do nothing
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) return;

      const url = `${WS_BASE}${WS_BASE.includes('?') ? '&' : '?'}token=${encodeURIComponent(tok)}`;
      try {
        const ws = new WebSocket(url);
        wsRef.current = ws;

        ws.onopen = () => {
          setIsConnected(true);
        };

        ws.onmessage = (evt) => {
          try {
            const msg = JSON.parse(evt.data);
            setLastMessage(msg);
            // fan out to subscribers
            listenersRef.current.forEach((fn) => {
              try { fn(msg); } catch {}
            });
          } catch (e) {
            // non-JSON frames are ignored
          }
        };

        ws.onerror = () => {
          // errors will be followed by onclose; don’t spam logs
        };

        ws.onclose = () => {
          setIsConnected(false);
          // schedule another reconnect (unless token cleared)
          scheduleReconnect();
        };
      } catch {
        // on construction failure, try again later
        scheduleReconnect();
      }
    }, base + jitter);
  }, [WS_BASE]);

  // Connect only when token or WS_BASE changes
  useEffect(() => {
    clearReconnect();

    // close any existing socket first
    if (wsRef.current) {
      try { wsRef.current.close(); } catch {}
      wsRef.current = null;
    }

    if (!token) {
      setIsConnected(false);
      return;
    }

    // initial connect
    scheduleReconnect();

    return () => {
      clearReconnect();
      if (wsRef.current) {
        try { wsRef.current.close(); } catch {}
        wsRef.current = null;
      }
    };
  }, [token, WS_BASE, scheduleReconnect]);

  // Subscribe without causing re-renders or reconnects
  const subscribe = useCallback((fn) => {
    listenersRef.current.add(fn);
    return () => listenersRef.current.delete(fn);
  }, []);

  return (
    <WebSocketContext.Provider value={{ isConnected, lastMessage, subscribe }}>
      {children}
    </WebSocketContext.Provider>
  );
};
