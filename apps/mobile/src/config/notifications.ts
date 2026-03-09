/**
 * Push Notifications Configuration
 */

export const NotificationConfig = {
  // Notification categories
  categories: {
    MARKET_ALERT: 'market_alert',
    SETTLEMENT: 'settlement',
    PROMOTION: 'promotion',
    SOCIAL: 'social',
  },

  // Deep link prefixes
  deepLinks: {
    market: 'predictx://market/',
    portfolio: 'predictx://portfolio',
    settlement: 'predictx://settlement/',
  },

  // Sound configurations
  sound: {
    default: 'default',
    none: null,
  },
};

export default NotificationConfig;
