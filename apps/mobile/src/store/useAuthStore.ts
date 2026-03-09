/**
 * Authentication Store (Zustand)
 * Manages user auth state, tokens, and onboarding progress
 */
import { create } from 'zustand';

export interface User {
  id: string;
  phoneNumber: string;
  displayName: string;
  avatar?: string;
  walletId: string;
  createdAt: string;
}

export interface AuthState {
  user: User | null;
  accessToken: string | null;
  refreshToken: string | null;
  onboardingStep: number; // 0: not started, 1-4: step, 5: complete
  isAuthenticated: boolean;

  // Actions
  setUser: (user: User | null) => void;
  setTokens: (access: string, refresh: string) => void;
  setOnboardingStep: (step: number) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  accessToken: null,
  refreshToken: null,
  onboardingStep: 0,
  isAuthenticated: false,

  setUser: (user) => set({ user, isAuthenticated: !!user }),
  setTokens: (access, refresh) => set({ accessToken: access, refreshToken: refresh }),
  setOnboardingStep: (step) => set({ onboardingStep: step }),
  logout: () => set({
    user: null,
    accessToken: null,
    refreshToken: null,
    isAuthenticated: false,
    onboardingStep: 0,
  }),
}));
