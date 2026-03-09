/**
 * Root Navigator
 * Handles auth flow and main navigation
 */
import React from 'react';
import { NavigationContainer } from '@react-navigation/native';
import { createNativeStackNavigator } from '@react-navigation/stack';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { Colors } from '@/theme/colors';
import { useAuthStore } from '@/store/useAuthStore';
import { OnboardingScreen } from '@/screens/OnboardingScreen';
import { FeedScreen } from '@/screens/FeedScreen';
import { PortfolioScreen } from '@/screens/PortfolioScreen';
import { SettingsScreen } from '@/screens/SettingsScreen';
import { RootStackParamList, MainTabsParamList } from './types';

const Stack = createNativeStackNavigator<RootStackParamList>();
const Tab = createBottomTabNavigator<MainTabsParamList>();

const TabNavigator = () => {
  return (
    <Tab.Navigator
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: Colors.accent,
        tabBarInactiveTintColor: Colors.text2,
        tabBarStyle: {
          backgroundColor: Colors.surface,
          borderTopColor: Colors.border,
          borderTopWidth: 1,
        },
        tabBarLabelStyle: {
          fontSize: 11,
          fontWeight: '500',
          marginTop: -8,
        },
      }}
    >
      <Tab.Screen
        name="Feed"
        component={FeedScreen}
        options={{
          title: 'Predict',
          tabBarLabel: 'Predict',
          tabBarIcon: ({ color }) => (
            <Text color={color} size="lg">
              🎯
            </Text>
          ),
        }}
      />
      <Tab.Screen
        name="Portfolio"
        component={PortfolioScreen}
        options={{
          title: 'Portfolio',
          tabBarLabel: 'Portfolio',
          tabBarIcon: ({ color }) => (
            <Text color={color} size="lg">
              📊
            </Text>
          ),
        }}
      />
      <Tab.Screen
        name="Settings"
        component={SettingsScreen}
        options={{
          title: 'Settings',
          tabBarLabel: 'Settings',
          tabBarIcon: ({ color }) => (
            <Text color={color} size="lg">
              ⚙️
            </Text>
          ),
        }}
      />
    </Tab.Navigator>
  );
};

// Import Text component for icons
import { Text } from '@/components/Text';

export const RootNavigator: React.FC = () => {
  const { onboardingStep } = useAuthStore();

  return (
    <NavigationContainer>
      <Stack.Navigator
        screenOptions={{
          headerShown: false,
          animationEnabled: true,
        }}
      >
        {onboardingStep < 5 ? (
          <Stack.Screen
            name="Onboarding"
            component={OnboardingScreen}
            options={{
              animationEnabled: false,
            }}
          />
        ) : (
          <Stack.Screen
            name="MainTabs"
            component={TabNavigator}
          />
        )}
      </Stack.Navigator>
    </NavigationContainer>
  );
};
