/**
 * RG Alert Provider
 * Wraps the app to provide WebSocket connection for real-time RG alerts
 */
import React from 'react';
import { useWebSocketAlerts } from '@/hooks/useWebSocketAlerts';

interface RGAlertProviderProps {
  children: React.ReactNode;
}

/**
 * Provider component that initializes WebSocket connection for RG alerts
 * Should be placed high in the component tree, typically in the main app root
 */
export const RGAlertProvider: React.FC<RGAlertProviderProps> = ({ children }) => {
  // Initialize WebSocket connection
  useWebSocketAlerts();

  return <>{children}</>;
};
