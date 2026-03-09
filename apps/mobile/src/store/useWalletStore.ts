/**
 * Wallet Store (Zustand)
 * Manages user balance, transaction history
 */
import { create } from 'zustand';

export interface WalletState {
  balance: number;
  currency: string;
  loading: boolean;

  // Actions
  setBalance: (balance: number) => void;
  addBalance: (amount: number) => void;
  deductBalance: (amount: number) => void;
  setLoading: (loading: boolean) => void;
}

export const useWalletStore = create<WalletState>((set) => ({
  balance: 0,
  currency: 'INR',
  loading: false,

  setBalance: (balance) => set({ balance }),
  addBalance: (amount) => set((state) => ({ balance: state.balance + amount })),
  deductBalance: (amount) => set((state) => ({ balance: state.balance - amount })),
  setLoading: (loading) => set({ loading }),
}));
