/**
 * WebSocket Hook for Real-Time Alerts
 * Subscribes to loss streak alerts and other RG notifications via Phoenix Channels
 */
import { useEffect, useRef, useCallback } from 'react';
import { useAuthStore } from '@/store/useAuthStore';
import { useRGStore, LossStreakAlert } from '@/store/useRGStore';
import { getApiConfig } from '@/config/api';

interface PhoenixMessage {
  topic: string;
  event: string;
  payload: any;
  ref: number | null;
}

export function useWebSocketAlerts() {
  const wsRef = useRef<WebSocket | null>(null);
  const userId = useAuthStore((state) => state.userId);
  const accessToken = useAuthStore((state) => state.accessToken);
  const setLossStreakAlert = useRGStore((state) => state.setLossStreakAlert);
  const messageRefCounter = useRef(0);

  const connect = useCallback(() => {
    if (!userId || !accessToken || wsRef.current?.readyState === WebSocket.OPEN) {
      return;
    }

    try {
      const config = getApiConfig();
      // Convert HTTP gateway URL to WebSocket URL
      const wsUrl = config.walletService
        .replace(/^https?:\/\//, 'wss://')
        .replace(':8005', ':8005/socket');

      wsRef.current = new WebSocket(`${wsUrl}?token=${accessToken}`);

      wsRef.current.onopen = () => {
        console.log('[WebSocket] Connected');
        // Subscribe to user's RG alert channel
        sendMessage({
          topic: `user:${userId}:rg_alerts`,
          event: 'phx_join',
          payload: {},
          ref: messageRefCounter.current++,
        });
      };

      wsRef.current.onmessage = (event) => {
        try {
          const message: PhoenixMessage = JSON.parse(event.data);
          handleMessage(message);
        } catch (error) {
          console.warn('[WebSocket] Failed to parse message:', error);
        }
      };

      wsRef.current.onerror = (error) => {
        console.error('[WebSocket] Error:', error);
      };

      wsRef.current.onclose = () => {
        console.log('[WebSocket] Disconnected');
        // Attempt reconnect after delay
        setTimeout(() => {
          connect();
        }, 3000);
      };
    } catch (error) {
      console.error('[WebSocket] Failed to connect:', error);
    }
  }, [userId, accessToken]);

  const disconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
  }, []);

  const sendMessage = useCallback((message: PhoenixMessage) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(message));
    }
  }, []);

  const handleMessage = (message: PhoenixMessage) => {
    if (message.topic === `user:${userId}:rg_alerts`) {
      if (message.event === 'loss_streak_alert') {
        // Convert Kafka event payload to frontend LossStreakAlert
        const alert: LossStreakAlert = {
          userId: message.payload.user_id,
          consecutiveLosses: message.payload.consecutive_losses,
          marketIds: message.payload.market_ids || [],
          totalLossMinor: message.payload.total_loss_minor || 0,
          timestamp: message.payload.timestamp,
        };
        setLossStreakAlert(alert);
      }
    }
  };

  // Connect on mount, disconnect on unmount
  useEffect(() => {
    connect();
    return () => {
      disconnect();
    };
  }, [connect, disconnect]);

  return {
    connected: wsRef.current?.readyState === WebSocket.OPEN,
    disconnect,
  };
}
