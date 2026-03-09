/**
 * Push Notifications Utilities
 * Register device, handle permissions, process notifications
 */
import * as Notifications from 'expo-notifications';
import * as Device from 'expo-device';

Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowAlert: true,
    shouldPlaySound: true,
    shouldSetBadge: true,
  }),
});

export async function registerForPushNotifications(): Promise<string | null> {
  if (!Device.isDevice) {
    console.log('Push notifications only work on device');
    return null;
  }

  try {
    // Request permissions
    const { status: existingStatus } = await Notifications.getPermissionsAsync();
    let finalStatus = existingStatus;

    if (existingStatus !== 'granted') {
      const { status } = await Notifications.requestPermissionsAsync();
      finalStatus = status;
    }

    if (finalStatus !== 'granted') {
      console.log('Failed to get push token: permission denied');
      return null;
    }

    // Get Expo push token
    try {
      const token = (await Notifications.getExpoPushTokenAsync()).data;
      console.log('Push token:', token);
      return token;
    } catch (err) {
      console.error('Failed to get push token:', err);
      return null;
    }
  } catch (err) {
    console.error('registerForPushNotifications error:', err);
    return null;
  }
}

export function setupNotificationListeners() {
  // Listen to notification when app is in foreground
  const subscription = Notifications.addNotificationReceivedListener(
    (notification) => {
      console.log('Notification received:', notification);
    }
  );

  // Listen to notification when user taps it
  const responseSubscription = Notifications.addNotificationResponseReceivedListener(
    (response) => {
      console.log('Notification response:', response);
      // Handle deep links, navigation, etc.
    }
  );

  return () => {
    subscription.remove();
    responseSubscription.remove();
  };
}
