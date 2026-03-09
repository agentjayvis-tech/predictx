# PredictX Mobile App

A lightweight, fast-loading React Native application for prediction markets with swipe-to-bet functionality.

## Features

- **Lightweight**: <5MB APK with minimal dependencies
- **Fast**: 45-second onboarding flow
- **Works on 3G**: Optimized for slow networks
- **Swipe-to-Bet**: Intuitive gesture-based betting
- **Push Notifications**: Real-time market alerts
- **iOS & Android**: Native cross-platform support

## Tech Stack

- **React Native** + **Expo** for cross-platform development
- **React Navigation** for routing
- **Zustand** for state management
- **TypeScript** for type safety
- **Snapchat UX Design** for UI/UX

## Getting Started

### Prerequisites

- Node.js 16+ and npm/yarn
- Expo CLI: `npm install -g expo-cli`

### Installation

```bash
# Install dependencies
npm install

# Start development server
npm start

# Build for Android
npm run build:android

# Build for iOS
npm run build:ios
```

## Project Structure

```
src/
├── assets/           # Icons, images, splash screens
├── components/       # Reusable UI components
├── config/          # API and notification config
├── navigation/      # React Navigation setup
├── screens/         # App screens
├── store/           # Zustand stores (auth, market, wallet)
├── theme/           # Design tokens (colors, typography, spacing)
├── utils/           # Utilities (notifications, etc.)
└── App.tsx          # App entry point
```

## Architecture

### Navigation
- **Onboarding**: 45-sec guided tour (5 steps, auto-advance)
- **MainTabs**: Bottom-tab navigation
  - **Feed**: Market discovery with swipe gestures
  - **Portfolio**: User balance and statistics
  - **Settings**: Preferences and account management

### State Management
- **useAuthStore**: User authentication, onboarding progress
- **useMarketStore**: Markets data, favorites, positions
- **useWalletStore**: Balance, currency, loading state

### Design System
- **Colors**: Dark theme with purple accents (#7C6AFF)
- **Typography**: Mobile-optimized font sizes (11-32pt)
- **Spacing**: 4px unit scale (0-96px)

## Optimizations for <5MB

1. **Minimal dependencies**: Only essential React Navigation, state management
2. **Code splitting**: Lazy load screens where possible
3. **Asset optimization**: Use emoji icons instead of image files
4. **Tree shaking**: All imports are explicitly named
5. **Hermes**: Enable on Android to reduce bundle size

## Push Notifications

- Set up in `src/utils/notifications.ts`
- Handlers: foreground display, user tap responses
- Deep linking: `predictx://market/{id}`, etc.
- Token is registered on app launch and sent to backend

## Performance Targets

- **APK size**: <5MB (measured with eas build)
- **Load time**: <3 seconds on 3G
- **Onboarding**: 45 seconds total
- **Feed refresh**: <2 seconds
- **Match latency**: <10ms (via streaming WebSocket)

## Configuration

### `app.json`
- App name, version, icons, splash screen
- iOS and Android specific settings
- Expo plugins for notifications and build properties

### `.env`
- API endpoints (development/production)
- Expo project ID for EAS

## Build & Deployment

### Local Development
```bash
npm start
# Press 'i' for iOS, 'a' for Android
```

### EAS Build
```bash
npm run build:android  # Builds APK for Play Store
npm run build:ios      # Builds IPA for App Store
```

### Configuration
- `eas.json`: Build and submit profiles
- `app.json`: App metadata and plugins

## Testing

```bash
npm test                # Run Jest tests
npm run lint            # Run ESLint
npm run type-check      # TypeScript check
```

## Contributing

1. Follow the existing code style (TypeScript, ESLint config)
2. Keep components focused and reusable
3. Add proper typing and comments
4. Test on both Android and iOS

## License

Proprietary - PredictX Inc.

## Support

See main project: https://www.notion.so/PredictX-Project-Hub-31ecc2b170e6817e97f6fc88b9d99d81
