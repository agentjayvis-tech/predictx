# Getting Started with PredictX Mobile

Quick start guide for setting up and running the PredictX React Native app.

## Prerequisites

- **Node.js** 16+ (verify: `node --version`)
- **npm** or **yarn** (verify: `npm --version`)
- **Expo CLI** (install: `npm install -g expo-cli`)
- **iOS**: macOS + Xcode 13+
- **Android**: Android Studio + SDK

## Installation

### 1. Clone and Setup

```bash
cd /Users/jayesh/Repos/bet-predict/apps/mobile
npm install
```

### 2. Configure Environment

```bash
cp .env.example .env
# Edit .env with your configuration
```

### 3. Start Development Server

```bash
npm start
```

This launches the Expo CLI. You'll see a menu with options:
- **Press `i`** → iOS emulator
- **Press `a`** → Android emulator
- **Scan QR code** → Expo Go app on physical device

## Project Structure

```
apps/mobile/
├── src/
│   ├── App.tsx                  # App entry point
│   ├── assets/                  # Icons, images, splash
│   ├── components/              # Reusable UI components
│   │   ├── Text.tsx            # Typography component
│   │   ├── Button.tsx          # Button component
│   │   └── MarketCard.tsx      # Market display
│   ├── config/                 # Configuration
│   │   ├── api.ts              # API endpoints
│   │   └── notifications.ts    # Notification config
│   ├── navigation/             # React Navigation
│   │   ├── types.ts            # Route type definitions
│   │   └── RootNavigator.tsx   # Main navigator
│   ├── screens/                # App screens
│   │   ├── OnboardingScreen.tsx    # 45-sec onboarding
│   │   ├── FeedScreen.tsx          # Market feed with swipes
│   │   ├── PortfolioScreen.tsx     # User portfolio
│   │   └── SettingsScreen.tsx      # User settings
│   ├── services/               # API clients
│   │   └── apiClient.ts        # Axios HTTP client
│   ├── store/                  # State management (Zustand)
│   │   ├── useAuthStore.ts     # Auth state
│   │   ├── useMarketStore.ts   # Markets state
│   │   └── useWalletStore.ts   # Wallet state
│   ├── theme/                  # Design system
│   │   ├── colors.ts           # Color tokens
│   │   ├── typography.ts       # Font sizes, weights
│   │   └── spacing.ts          # Spacing scale
│   └── utils/                  # Utilities
│       └── notifications.ts    # Push notification setup
├── app.json                    # Expo app config
├── package.json                # Dependencies & scripts
├── tsconfig.json               # TypeScript config
├── babel.config.js             # Babel config
├── jest.config.js              # Jest test config
└── README.md                   # Project readme

```

## Development Workflow

### Running on Emulator/Simulator

```bash
# Start dev server
npm start

# iOS Simulator (macOS only)
i

# Android Emulator (requires Android Studio)
a
```

### Running on Physical Device

```bash
# Start dev server
npm start

# Scan QR code with Expo Go app
# Or use deep link: exp://...
```

### Hot Reload

Changes to `src/` are automatically reloaded:
- Save file → Fast refresh
- No need to rebuild

### Running Tests

```bash
# Run unit tests
npm test

# Run with coverage
npm test -- --coverage

# Run specific test file
npm test -- Text.test.tsx
```

### Type Checking

```bash
# Check TypeScript
npm run type-check

# Watch mode
npx tsc --watch
```

### Linting

```bash
# Run ESLint
npm run lint

# Fix auto-fixable issues
npx eslint src --fix
```

## Building for Production

### Android APK (Development/Testing)

```bash
npm run build:android
# Downloads APK to your machine
# Install with: adb install app-release.apk
```

### Android App Bundle (Production)

```bash
eas build --platform android
# Requires EAS account: eas login
# Builds AAB for Play Store submission
```

### iOS (Production)

```bash
eas build --platform ios
# Requires Apple Developer account
# Builds IPA for TestFlight/App Store
```

## Configuration Files Explained

### `app.json`
- App metadata (name, version, slug)
- Icon and splash screen
- Platform-specific settings (iOS bundle ID, Android package)
- Expo plugins (notifications, build properties)

### `package.json`
- Dependencies and dev dependencies
- Scripts (start, build, test, lint)
- Project metadata

### `tsconfig.json`
- TypeScript compiler options
- Path aliases (`@/*` → `src/*`)
- Strict mode enabled

### `.env`
- API endpoints (dev vs prod)
- Expo project ID
- Environment variables (not in git)

## Common Tasks

### Add a New Screen

1. Create file: `src/screens/NewScreen.tsx`
2. Add to navigation: `src/navigation/RootNavigator.tsx`
3. Add route types: `src/navigation/types.ts`

Example:
```typescript
// src/screens/NewScreen.tsx
import React from 'react';
import { SafeAreaView } from 'react-native';
import { Text } from '@/components/Text';

export const NewScreen: React.FC = () => {
  return (
    <SafeAreaView>
      <Text variant="h2">New Screen</Text>
    </SafeAreaView>
  );
};
```

### Add a New Component

1. Create file: `src/components/NewComponent.tsx`
2. Import in parent: `import { NewComponent } from '@/components/NewComponent'`

### Add a New Store

1. Create file: `src/store/useNewStore.ts`
2. Use in components: `const { data, setData } = useNewStore()`

### Integrate with Backend API

1. Add endpoint: `src/services/apiClient.ts`
2. Use in component:
```typescript
const { data } = await apiClient.market.getMarkets();
```

### Change Color Scheme

Edit `src/theme/colors.ts` and rebuild

## Troubleshooting

### "Metro bundler failed"
```bash
# Clear cache and restart
npm start -- --clear
```

### "Cannot find module"
```bash
# Reinstall dependencies
rm -rf node_modules
npm install
npm start -- --clear
```

### Emulator not running
```bash
# For Android, launch emulator first
~/Library/Android/sdk/emulator/emulator -avd Pixel_4
# Then: npm start → a
```

### Slow development build
```bash
# Use Hermes for faster rebuild
# Edit app.json: "jsEngine": "hermes"
npm start -- --clear
```

## Deployment Checklist

- [ ] Update version in `app.json`
- [ ] Test all screens on iOS
- [ ] Test all screens on Android
- [ ] Verify APK size < 5MB
- [ ] Test on slow 3G connection
- [ ] Run `npm run lint` and `npm run type-check`
- [ ] Update CHANGELOG
- [ ] Build: `eas build --platform android --platform ios`
- [ ] Test on TestFlight/internal testing
- [ ] Submit to App Store/Play Store

## Next Steps

1. **Read Architecture**: `ARCHITECTURE.md`
2. **Understand Onboarding**: `ONBOARDING.md`
3. **Size Optimization**: `SIZE_OPTIMIZATION.md`
4. **View Full README**: `README.md`

## Resources

- [React Native Docs](https://reactnative.dev/)
- [Expo Docs](https://docs.expo.dev/)
- [React Navigation](https://reactnavigation.org/)
- [Zustand GitHub](https://github.com/pmndrs/zustand)
- [TypeScript Handbook](https://www.typescriptlang.org/docs/)

## Support

For issues or questions:
1. Check Expo CLI logs: `npm start`
2. Review error message in terminal
3. Search GitHub issues
4. Open new issue with reproduction steps
