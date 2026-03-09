/**
 * App Entry Point
 * Sets up notifications, theme, and navigation
 */
import React, { useEffect } from 'react';
import { StatusBar, useColorScheme } from 'react-native';
import * as SplashScreen from 'expo-splash-screen';
import { Colors } from '@/theme/colors';
import { RootNavigator } from '@/navigation/RootNavigator';
import { registerForPushNotifications, setupNotificationListeners } from '@/utils/notifications';

// Keep the splash screen visible while we fetch resources
SplashScreen.preventAutoHideAsync();

export default function App() {
  const colorScheme = useColorScheme();

  useEffect(() => {
    const setupApp = async () => {
      try {
        // Register for push notifications
        const token = await registerForPushNotifications();
        if (token) {
          console.log('Push token registered:', token);
          // TODO: Send token to backend
        }

        // Setup notification listeners
        const unsubscribe = setupNotificationListeners();

        // Hide splash screen
        await SplashScreen.hideAsync();

        return () => {
          unsubscribe();
        };
      } catch (error) {
        console.error('Failed to setup app:', error);
        await SplashScreen.hideAsync();
      }
    };

    setupApp();
  }, []);

  return (
    <>
      <StatusBar
        barStyle="light-content"
        backgroundColor={Colors.bg}
        translucent={false}
      />
      <RootNavigator />
    </>
  );
}
