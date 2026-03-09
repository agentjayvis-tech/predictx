/**
 * Market Store (Zustand)
 * Manages market data, user positions, and feed state
 */
import { create } from 'zustand';

export interface MarketOutcome {
  id: string;
  title: string;
  yesOdds: number;
  noOdds: number;
  volume: number;
}

export interface Market {
  id: string;
  question: string;
  category: string;
  closesAt: string;
  resolvesAt: string;
  outcome?: string;
  outcomes: MarketOutcome[];
  volume: number;
  createdAt: string;
  description?: string;
  imageUrl?: string;
}

export interface MarketState {
  markets: Market[];
  favorites: Set<string>;
  userPositions: Record<string, any>;

  // Actions
  setMarkets: (markets: Market[]) => void;
  toggleFavorite: (marketId: string) => void;
  updatePosition: (marketId: string, position: any) => void;
}

export const useMarketStore = create<MarketState>((set) => ({
  markets: [],
  favorites: new Set(),
  userPositions: {},

  setMarkets: (markets) => set({ markets }),
  toggleFavorite: (marketId) =>
    set((state) => {
      const newFavorites = new Set(state.favorites);
      if (newFavorites.has(marketId)) {
        newFavorites.delete(marketId);
      } else {
        newFavorites.add(marketId);
      }
      return { favorites: newFavorites };
    }),
  updatePosition: (marketId, position) =>
    set((state) => ({
      userPositions: {
        ...state.userPositions,
        [marketId]: position,
      },
    })),
}));
