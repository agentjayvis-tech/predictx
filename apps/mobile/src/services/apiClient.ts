/**
 * API Client
 * Handles HTTP requests to backend microservices
 */
import axios, { AxiosInstance, AxiosError } from 'axios';
import { getApiConfig } from '@/config/api';
import { useAuthStore } from '@/store/useAuthStore';

let apiClient: AxiosInstance | null = null;

export function initializeApiClient() {
  const config = getApiConfig();

  apiClient = axios.create({
    baseURL: config.walletService,
    timeout: 10000,
    headers: {
      'Content-Type': 'application/json',
    },
  });

  // Add request interceptor for auth token
  apiClient.interceptors.request.use(
    (config) => {
      const { accessToken } = useAuthStore.getState();
      if (accessToken) {
        config.headers.Authorization = `Bearer ${accessToken}`;
      }
      return config;
    },
    (error) => {
      return Promise.reject(error);
    }
  );

  // Add response interceptor for error handling
  apiClient.interceptors.response.use(
    (response) => response,
    (error: AxiosError) => {
      if (error.response?.status === 401) {
        // Token expired, logout user
        useAuthStore.getState().logout();
      }
      return Promise.reject(error);
    }
  );

  return apiClient;
}

export function getApiClient(): AxiosInstance {
  if (!apiClient) {
    initializeApiClient();
  }
  return apiClient!;
}

/**
 * Wallet API
 */
export const walletApi = {
  getBalance: (userId: string) =>
    getApiClient().get(`/v1/wallets/${userId}/balance`),

  deposit: (userId: string, amount: number) =>
    getApiClient().post(`/v1/wallets/${userId}/deposit`, { amount }),

  spend: (userId: string, amount: number, reason: string) =>
    getApiClient().post(`/v1/wallets/${userId}/spend`, { amount, reason }),

  refund: (userId: string, amount: number) =>
    getApiClient().post(`/v1/wallets/${userId}/refund`, { amount }),
};

/**
 * Market API
 */
export const marketApi = {
  getMarkets: (limit = 20, offset = 0) =>
    getApiClient().get('/v1/markets', { params: { limit, offset } }),

  getMarket: (marketId: string) =>
    getApiClient().get(`/v1/markets/${marketId}`),

  getResolvableMarkets: () =>
    getApiClient().get('/internal/markets/resolvable'),
};

/**
 * Settlement API
 */
export const settlementApi = {
  getUserPnL: (userId: string, marketId: string) =>
    getApiClient().get(`/v1/settlements/user/${userId}/market/${marketId}`),

  getSettlementStatus: (settlementId: string) =>
    getApiClient().get(`/v1/settlements/${settlementId}`),
};

export default {
  wallet: walletApi,
  market: marketApi,
  settlement: settlementApi,
  init: initializeApiClient,
};
