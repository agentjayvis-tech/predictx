/**
 * Navigation Type Definitions
 */
import { NavigatorScreenParams } from '@react-navigation/native';

export type RootStackParamList = {
  Onboarding: undefined;
  MainTabs: NavigatorScreenParams<MainTabsParamList>;
  MarketDetail: { marketId: string };
};

export type MainTabsParamList = {
  Feed: undefined;
  Portfolio: undefined;
  Settings: undefined;
};
