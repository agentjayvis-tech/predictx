/**
 * API Configuration
 * Endpoints for microservices
 */

const IS_DEV = __DEV__;

export const API_CONFIG = {
  // Development
  dev: {
    walletService: 'http://localhost:8002',
    marketService: 'http://localhost:8001',
    matchingEngine: 'http://localhost:8004',
    settlementService: 'http://localhost:8003',
    resolutionService: 'http://localhost:8000',
    websocket: 'ws://localhost:8005',
  },

  // Production
  prod: {
    walletService: 'https://wallet.predictx.io',
    marketService: 'https://market.predictx.io',
    matchingEngine: 'https://matching.predictx.io',
    settlementService: 'https://settlement.predictx.io',
    resolutionService: 'https://resolution.predictx.io',
    websocket: 'wss://stream.predictx.io',
  },
};

export const getApiConfig = () => {
  return IS_DEV ? API_CONFIG.dev : API_CONFIG.prod;
};
