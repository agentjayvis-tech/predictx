/**
 * Responsible Gambling Store (Zustand)
 * Manages deposit limits, cool-off periods, self-exclusion, and loss tracking
 */
import { create } from 'zustand';
import { rgApi } from '@/services/apiClient';
import { useAuthStore } from '@/store/useAuthStore';

export interface DepositSettings {
  userId: string;
  dailyDepositLimitMinor: number;
  monthlyDepositLimitMinor?: number;
  enabled: boolean;
  remainingDailyBudget?: number;
  createdAt: string;
  updatedAt: string;
}

export interface ExclusionSettings {
  userId: string;
  countryCode?: string;
  inCoolOff: boolean;
  coolOffRemainingHours: number;
  coolOffDurationHours: number;
  coolOffCancellable: boolean;
  isSelfExcluded: boolean;
  selfExclusionRemainingDays: number;
  selfExclusionDurationDays?: number;
  createdAt: string;
  updatedAt: string;
}

export interface LossStreakAlert {
  userId: string;
  consecutiveLosses: number;
  marketIds: string[];
  totalLossMinor: number;
  timestamp: string;
}

export interface RGState {
  // Deposit settings
  depositSettings: DepositSettings | null;
  depositSettingsLoading: boolean;
  depositSettingsError: string | null;

  // Exclusion settings
  exclusionSettings: ExclusionSettings | null;
  exclusionSettingsLoading: boolean;
  exclusionSettingsError: string | null;

  // Loss streak
  lossStreakThreshold: number;
  lossStreakAlert: LossStreakAlert | null;
  showLossAlert: boolean;

  // UI state
  showDepositLimitWarning: boolean;
  canCancelCoolOff: boolean;

  // Actions (setters)
  setDepositSettings: (settings: DepositSettings | null) => void;
  setDepositSettingsLoading: (loading: boolean) => void;
  setDepositSettingsError: (error: string | null) => void;

  setExclusionSettings: (settings: ExclusionSettings | null) => void;
  setExclusionSettingsLoading: (loading: boolean) => void;
  setExclusionSettingsError: (error: string | null) => void;

  setLossStreakThreshold: (threshold: number) => void;
  setLossStreakAlert: (alert: LossStreakAlert | null) => void;
  setShowLossAlert: (show: boolean) => void;

  // Async actions
  fetchDepositSettings: () => Promise<void>;
  updateDepositLimit: (dailyLimitMinor: number, monthlyLimitMinor?: number) => Promise<void>;
  fetchExclusionSettings: () => Promise<void>;
  startCoolOff: (durationHours: 24 | 168 | 720) => Promise<void>;
  cancelCoolOff: () => Promise<void>;
  selfExclude: (durationDays?: number) => Promise<void>;
  fetchLossStreakThreshold: () => Promise<void>;
  updateLossStreakThreshold: (threshold: number) => Promise<void>;

  // Helper actions
  dismissLossAlert: () => void;
  canDeposit: () => boolean;
  canPlaceOrder: () => boolean;
  getRemainingDailyBudget: () => number;
}

export const useRGStore = create<RGState>((set, get) => ({
  // Initial state
  depositSettings: null,
  depositSettingsLoading: false,
  depositSettingsError: null,

  exclusionSettings: null,
  exclusionSettingsLoading: false,
  exclusionSettingsError: null,

  lossStreakThreshold: 3,
  lossStreakAlert: null,
  showLossAlert: false,

  showDepositLimitWarning: false,
  canCancelCoolOff: false,

  // Actions
  setDepositSettings: (settings) => {
    set({ depositSettings: settings, depositSettingsError: null });
  },

  setDepositSettingsLoading: (loading) => {
    set({ depositSettingsLoading: loading });
  },

  setDepositSettingsError: (error) => {
    set({ depositSettingsError: error });
  },

  setExclusionSettings: (settings) => {
    set({
      exclusionSettings: settings,
      exclusionSettingsError: null,
      canCancelCoolOff: settings?.coolOffCancellable ?? false,
    });
  },

  setExclusionSettingsLoading: (loading) => {
    set({ exclusionSettingsLoading: loading });
  },

  setExclusionSettingsError: (error) => {
    set({ exclusionSettingsError: error });
  },

  setLossStreakThreshold: (threshold) => {
    set({ lossStreakThreshold: Math.max(1, Math.min(10, threshold)) });
  },

  setLossStreakAlert: (alert) => {
    set({ lossStreakAlert: alert, showLossAlert: !!alert });
  },

  setShowLossAlert: (show) => {
    set({ showLossAlert: show });
  },

  dismissLossAlert: () => {
    set({ showLossAlert: false });
  },

  canDeposit: () => {
    const state = get();
    if (!state.exclusionSettings) return true;

    // Cannot deposit if in cool-off
    if (state.exclusionSettings.inCoolOff) return false;

    // Cannot deposit if self-excluded
    if (state.exclusionSettings.isSelfExcluded) return false;

    // Cannot deposit if limit disabled
    if (state.depositSettings && !state.depositSettings.enabled) return false;

    return true;
  },

  canPlaceOrder: () => {
    const state = get();
    if (!state.exclusionSettings) return true;

    // Cannot place order if in cool-off
    if (state.exclusionSettings.inCoolOff) return false;

    // Cannot place order if self-excluded
    if (state.exclusionSettings.isSelfExcluded) return false;

    return true;
  },

  getRemainingDailyBudget: () => {
    const state = get();
    if (!state.depositSettings) return 0;
    return state.depositSettings.remainingDailyBudget ?? 0;
  },

  // Async API actions
  fetchDepositSettings: async () => {
    const userId = useAuthStore.getState().userId;
    if (!userId) {
      set({ depositSettingsError: 'User not authenticated' });
      return;
    }

    set({ depositSettingsLoading: true, depositSettingsError: null });
    try {
      const response = await rgApi.getDepositSettings(userId);
      set({
        depositSettings: response.data,
        depositSettingsLoading: false,
      });
    } catch (error: any) {
      set({
        depositSettingsError: error.message || 'Failed to fetch deposit settings',
        depositSettingsLoading: false,
      });
    }
  },

  updateDepositLimit: async (dailyLimitMinor: number, monthlyLimitMinor?: number) => {
    const userId = useAuthStore.getState().userId;
    if (!userId) {
      set({ depositSettingsError: 'User not authenticated' });
      return;
    }

    set({ depositSettingsLoading: true, depositSettingsError: null });
    try {
      const response = await rgApi.updateDepositSettings(userId, dailyLimitMinor, monthlyLimitMinor);
      set({
        depositSettings: response.data,
        depositSettingsLoading: false,
      });
    } catch (error: any) {
      set({
        depositSettingsError: error.message || 'Failed to update deposit limit',
        depositSettingsLoading: false,
      });
    }
  },

  fetchExclusionSettings: async () => {
    const userId = useAuthStore.getState().userId;
    if (!userId) {
      set({ exclusionSettingsError: 'User not authenticated' });
      return;
    }

    set({ exclusionSettingsLoading: true, exclusionSettingsError: null });
    try {
      const response = await rgApi.getExclusionSettings(userId);
      set({
        exclusionSettings: response.data,
        canCancelCoolOff: response.data.coolOffCancellable,
        exclusionSettingsLoading: false,
      });
    } catch (error: any) {
      set({
        exclusionSettingsError: error.message || 'Failed to fetch exclusion settings',
        exclusionSettingsLoading: false,
      });
    }
  },

  startCoolOff: async (durationHours: 24 | 168 | 720) => {
    const userId = useAuthStore.getState().userId;
    if (!userId) {
      set({ exclusionSettingsError: 'User not authenticated' });
      return;
    }

    set({ exclusionSettingsLoading: true, exclusionSettingsError: null });
    try {
      const response = await rgApi.startCoolOff(userId, durationHours);
      set({
        exclusionSettings: response.data,
        canCancelCoolOff: response.data.coolOffCancellable,
        exclusionSettingsLoading: false,
      });
    } catch (error: any) {
      set({
        exclusionSettingsError: error.message || 'Failed to start cool-off',
        exclusionSettingsLoading: false,
      });
    }
  },

  cancelCoolOff: async () => {
    const userId = useAuthStore.getState().userId;
    if (!userId) {
      set({ exclusionSettingsError: 'User not authenticated' });
      return;
    }

    set({ exclusionSettingsLoading: true, exclusionSettingsError: null });
    try {
      const response = await rgApi.cancelCoolOff(userId);
      set({
        exclusionSettings: response.data,
        canCancelCoolOff: response.data.coolOffCancellable,
        exclusionSettingsLoading: false,
      });
    } catch (error: any) {
      set({
        exclusionSettingsError: error.message || 'Failed to cancel cool-off',
        exclusionSettingsLoading: false,
      });
    }
  },

  selfExclude: async (durationDays?: number) => {
    const userId = useAuthStore.getState().userId;
    if (!userId) {
      set({ exclusionSettingsError: 'User not authenticated' });
      return;
    }

    set({ exclusionSettingsLoading: true, exclusionSettingsError: null });
    try {
      const response = await rgApi.selfExclude(userId, durationDays);
      set({
        exclusionSettings: response.data,
        canCancelCoolOff: response.data.coolOffCancellable,
        exclusionSettingsLoading: false,
      });
    } catch (error: any) {
      set({
        exclusionSettingsError: error.message || 'Failed to self-exclude',
        exclusionSettingsLoading: false,
      });
    }
  },

  fetchLossStreakThreshold: async () => {
    const userId = useAuthStore.getState().userId;
    if (!userId) return;

    try {
      const response = await rgApi.getLossStreakThreshold(userId);
      set({ lossStreakThreshold: response.data.threshold });
    } catch (error) {
      // Non-critical; log but don't block
      console.warn('Failed to fetch loss streak threshold:', error);
    }
  },

  updateLossStreakThreshold: async (threshold: number) => {
    const userId = useAuthStore.getState().userId;
    if (!userId) return;

    // Clamp to valid range
    const clampedThreshold = Math.max(1, Math.min(10, threshold));
    set({ lossStreakThreshold: clampedThreshold });

    try {
      await rgApi.updateLossStreakThreshold(userId, clampedThreshold);
    } catch (error: any) {
      console.warn('Failed to update loss streak threshold:', error.message);
    }
  },
}));
