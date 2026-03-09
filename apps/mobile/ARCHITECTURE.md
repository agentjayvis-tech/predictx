# PredictX Mobile App Architecture

## Overview

The PredictX mobile app is a lightweight, performance-optimized React Native application designed for prediction markets. It prioritizes simplicity, speed, and minimal bundle size while providing a feature-rich user experience.

## Design Principles

1. **Lightweight**: Minimal dependencies, optimized for <5MB APK
2. **Fast**: Optimized for 3G networks and quick interactions
3. **Simple**: Single responsibility components, clean separation of concerns
4. **Responsive**: Works seamlessly on iOS and Android
5. **Type-safe**: Full TypeScript coverage

## Architecture Layers

### 1. Presentation Layer

#### Navigation (`src/navigation/`)
- **RootNavigator**: Main navigation orchestrator
  - Handles auth flow (Onboarding vs MainTabs)
  - Manages bottom tab navigation
  - Type-safe route definitions

#### Screens (`src/screens/`)
- **OnboardingScreen**: 45-second guided onboarding
- **FeedScreen**: Main market discovery feed with swipe gestures
- **PortfolioScreen**: User balance and statistics
- **SettingsScreen**: User preferences and account management

#### Components (`src/components/`)
- **Text**: Typography component with variants (h1-h3, body, caption, label)
- **Button**: Reusable button with variants (primary, secondary, tertiary)
- **MarketCard**: Market display with swipe gesture detection

### 2. State Management Layer

**Zustand Stores** (`src/store/`)

```
useAuthStore
├── user (User | null)
├── accessToken (string | null)
├── refreshToken (string | null)
├── onboardingStep (0-5)
└── isAuthenticated (boolean)

useMarketStore
├── markets (Market[])
├── favorites (Set<string>)
├── userPositions (Record<string, Position>)

useWalletStore
├── balance (number)
├── currency (string)
├── loading (boolean)
```

**Why Zustand?**
- No provider boilerplate
- Minimal overhead (<2KB)
- Direct state access from anywhere
- Built-in persistence support

### 3. Data Layer

#### API Client (`src/services/apiClient.ts`)
- Axios-based HTTP client with interceptors
- Request: Auto-inject auth token
- Response: Handle 401 errors, retry logic
- Organized by resource: wallet, market, settlement APIs

#### Configuration (`src/config/`)
- `api.ts`: Environment-aware endpoint URLs
- `notifications.ts`: Deep link and category config

### 4. Design System (`src/theme/`)

#### Colors
- **Dark theme**: bg (#080810), surface (#111119-#222230)
- **Accent**: Purple (#7C6AFF) with light variant
- **Semantic**: Green (#3DDC84), Red (#FF5A5A), Yellow (#FFD644)
- **Text**: Text (#F0F0F4), Text2 (#8E8E9A), Text3 (#55555F)

#### Typography
- **Sizes**: 11pt (xs) to 32pt (4xl)
- **Weights**: 300 (light) to 800 (extrabold)
- **Line heights**: 1.2 (tight) to 1.75 (relaxed)

#### Spacing
- **Base unit**: 4px
- **Scale**: 0-96px (0, 4, 8, 12, 16, 20, 24, 28, 32, 36, 40, 48, 64, 80, 96)

### 5. Utilities

#### Notifications (`src/utils/notifications.ts`)
- Register for push notifications
- Request OS permissions
- Setup foreground/tap listeners
- Handle deep links

## Data Flow

### Onboarding Flow
```
App.tsx
  → RootNavigator checks onboardingStep
  → OnboardingScreen (5 auto-advancing steps)
    → setOnboardingStep(5) on complete
    → triggers RootNavigator switch to MainTabs
```

### Market Feed Flow
```
FeedScreen
  → useEffect: loadMarkets()
    → API: marketApi.getMarkets()
    → useMarketStore.setMarkets()
  → FlatList renders MarketCard[] from store
  → User swipes:
    → handleSwipeLeft (skip)
    → handleSwipeRight (bet) → navigate to BetScreen
```

### Authentication Flow
```
1. User logs in (future Login screen)
   → setUser(), setTokens()
   → setOnboardingStep(0)
2. Onboarding completes
   → setOnboardingStep(5)
3. Navigate to Feed
   → Load markets
   → Display portfolio with balance

4. Logout
   → useAuthStore.logout()
   → Clear tokens, user, etc.
   → Return to Onboarding
```

## Performance Optimizations

### Bundle Size (<5MB Target)

1. **Minimal Dependencies**
   - React Native: Core framework
   - React Navigation: Lightweight routing
   - Zustand: State (2KB)
   - Axios: HTTP (6KB gzipped)
   - expo-*: Core modules only

2. **Code Splitting**
   - Lazy load screens
   - Dynamic imports for heavy modules
   - Tree-shake unused code

3. **Asset Optimization**
   - Use emoji icons (0 bytes) instead of image files
   - Compress splash screen and app icon
   - SVG for logos (if needed)

4. **Hermes Engine** (Android)
   - Bytecode compilation
   - Reduced RAM usage
   - Faster startup

### Network Optimization

1. **3G Compatibility**
   - Min image sizes
   - HTTP/2 support
   - Gzip compression
   - Request batching

2. **Caching**
   - 60s market data TTL
   - Balance cache in store
   - HTTP cache headers

### Runtime Performance

1. **Navigation**
   - Native Stack Navigator (fast transitions)
   - Bottom tabs (persistent state)
   - No deep nesting

2. **Rendering**
   - FlatList for markets (virtualization)
   - Memoized components
   - Avoid inline styles

3. **State Management**
   - Zustand (direct state, no re-render bloat)
   - Shallow equality checks
   - Selective subscription

## Deployment Pipeline

### Build Process
```
Development
  └─ npm start (Expo Go)
     └─ Test on device/emulator

Preview
  └─ eas build --platform android (APK)
     └─ Install via .apk file

Production
  └─ eas build --platform android (AAB)
     └─ Submit to Play Store
  └─ eas build --platform ios (IPA)
     └─ Submit to App Store
```

### Configuration Files
- `app.json`: App metadata, plugins
- `eas.json`: Build profiles
- `package.json`: Scripts, dependencies
- `.env`: Runtime config (dev vs prod)

## Security

1. **Token Management**
   - Store in secure storage (AsyncStorage + Secure Store)
   - Auto-refresh on 401
   - Clear on logout

2. **API Security**
   - HTTPS only
   - Request signing (future)
   - Rate limiting (server-side)

3. **Data Privacy**
   - No sensitive data in logs
   - Encrypt user positions at rest
   - GDPR-compliant data handling

## Testing Strategy

### Unit Tests
- Components: Render and props tests
- Stores: State mutations
- Utils: Pure functions

### Integration Tests
- Navigation flows
- API client with mocks
- Store integration

### E2E Tests (Future)
- Onboarding flow
- Market feed interactions
- Settlement notifications

## Future Enhancements

1. **Features**
   - Live market updates (WebSocket)
   - Advanced charting
   - Social features (leaderboards)
   - In-app messaging

2. **Performance**
   - Code splitting per tab
   - Streaming API for real-time data
   - Service workers (PWA version)

3. **Analytics**
   - User engagement tracking
   - Crash reporting
   - Performance monitoring

4. **Testing**
   - Automated E2E tests
   - A/B testing framework
   - Beta channel support

## Troubleshooting

### Build Issues
- Clear cache: `expo prebuild --clean`
- Rebuild native modules: `eas build --platform android --clean`

### Performance
- Check bundle size: `expo build:web --stats`
- Profile with Hermes: `bundletool analyze --bundle=app.aab`

### Network
- Test on slow 3G: Chrome DevTools throttling
- Check payload sizes: Network tab

## References

- [React Navigation](https://reactnavigation.org/)
- [Zustand](https://github.com/pmndrs/zustand)
- [Expo Docs](https://docs.expo.dev/)
- [React Native Performance](https://reactnative.dev/docs/optimizing-flatlist-configuration)
