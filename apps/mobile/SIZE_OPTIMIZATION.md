# Bundle Size Optimization Guide

## Target: <5MB APK

This guide explains how PredictX Mobile achieves sub-5MB bundle size and how to maintain it.

## Current Dependencies Analysis

### Core Dependencies (Mandatory)
| Package | Size (gzip) | Purpose |
|---------|-----------|---------|
| react-native | ~800KB | Framework |
| react | ~40KB | UI library |
| @react-navigation/native | ~50KB | Navigation |
| @react-navigation/bottom-tabs | ~30KB | Tab navigation |
| zustand | ~2KB | State management |
| axios | ~6KB | HTTP client |

**Total**: ~930KB

### Optional Dependencies (Remove if unused)
- `react-native-reanimated`: If using complex animations
- `react-native-gesture-handler`: If using advanced gestures
- Heavy image libraries: Use emoji instead

## Size Breakdown by Bundle

### Android APK
```
Total APK: ~4.5MB
├── APK overhead: ~500KB (manifest, resources)
├── Native code: ~1.5MB (React Native, Hermes)
├── JS bundle: ~800KB (app code + deps)
├── Assets: ~1MB (splash, icon, fonts)
└── Metadata: ~200KB (signatures, etc.)
```

### iOS IPA
```
Total IPA: ~4.8MB
├── App wrapper: ~300KB
├── Native frameworks: ~2MB (iOS RN, Hermes)
├── JS bundle: ~800KB
├── Assets: ~900KB
└── Metadata: ~100KB
```

## Optimization Techniques

### 1. Bundle Size Analysis

```bash
# Analyze bundle size
npm run build:web -- --stats

# Use bundletool for Android
bundletool analyze --bundle=app.aab --mode=detailed

# Use Xcode for iOS
# Xcode → Product → Build & Analyze
```

### 2. Dependency Reduction

✅ **Keep**
- react-native, react
- @react-navigation (essential)
- zustand (2KB, essential for state)
- axios (6KB, essential for API)

❌ **Remove**
- Redux, MobX (too heavy for mobile)
- Lodash (use native JS)
- moment.js (use native Date)
- Heavy UI kits

⚠️ **Consider**
- react-native-reanimated (only if needed)
- Image libraries (use emoji)
- Date libraries (use Date native)

### 3. Code Splitting

```typescript
// Before: All screens bundled together
import { FeedScreen } from '@/screens/FeedScreen';

// After: Lazy load screens (if needed)
const FeedScreen = lazy(() => import('@/screens/FeedScreen'));
```

### 4. Asset Optimization

#### Icons
```typescript
// ❌ Bad: Image files (~2-5KB each)
<Image source={require('./icons/home.png')} />

// ✅ Good: Emoji (0 bytes)
<Text style={size: 24}>🏠</Text>
```

#### Fonts
```typescript
// ❌ Bad: Custom fonts (~50-100KB)
fontFamily: 'CustomFont'

// ✅ Good: System fonts (0 bytes)
fontFamily: 'System'
```

#### Splash Screen
- Keep resolution: 1080x1920 (not 4K)
- Use PNG, compress with TinyPNG
- Target: <100KB

#### App Icon
- Single source: 1024x1024 PNG
- Use Expo to auto-generate sizes
- Compress before upload

### 5. Hermes Engine (Android)

Enable in `app.json`:
```json
{
  "android": {
    "jsEngine": "hermes",
    "enableOnBackGesture": true
  }
}
```

**Benefits**:
- ~20% smaller APK
- ~40% faster startup
- ~25% less RAM usage

### 6. Minification & Optimization

Expo handles minification automatically:
- JavaScript: Uglified
- CSS: Minified
- Assets: Optimized

### 7. ProGuard Rules (Android)

```proguard
# app.json
{
  "android": {
    "minifyEnabled": true,
    "shrinkResources": true
  }
}
```

## Monitoring Bundle Size

### Pre-commit Hook

Create `.git/hooks/pre-commit`:
```bash
#!/bin/bash
SIZE=$(stat -f%z app-release.apk)
if [ $SIZE -gt 5242880 ]; then  # 5MB
  echo "⚠️  APK size: $(($SIZE / 1024 / 1024))MB - exceeds 5MB limit!"
  exit 1
fi
```

### CI/CD Integration

Add to GitHub Actions:
```yaml
- name: Check APK size
  run: |
    SIZE=$(stat -c%s app-release.apk)
    if [ $SIZE -gt 5242880 ]; then
      echo "❌ APK exceeds 5MB limit"
      exit 1
    fi
    echo "✅ APK size: $((SIZE / 1024 / 1024))MB"
```

## Performance vs Size Trade-offs

| Feature | Size Impact | Recommendation |
|---------|-------------|-----------------|
| Animations | +50-100KB | Use sparingly |
| Charts | +200-300KB | Defer/lazy load |
| Maps | +500KB+ | Avoid |
| Image gallery | +100-200KB | Lazy load images |
| Real-time updates | +30-50KB | Essential, keep |
| Analytics | +50KB | Defer or skip |

## Maintenance Checklist

- [ ] Monitor bundle size in CI/CD
- [ ] Review new dependencies before adding
- [ ] Use `npm audit` for security
- [ ] Test on low-end devices
- [ ] Measure startup time on 3G
- [ ] Check APK size before release
- [ ] Profile with Android Studio (Android)
- [ ] Use Instruments (iOS)

## Common Issues & Solutions

### Issue: APK size creeping up
**Solution**:
1. Run `bundletool analyze --bundle=app.aab`
2. Identify new large dependencies
3. Consider alternatives (e.g., axios → fetch)

### Issue: Slow startup on 3G
**Solution**:
1. Enable Hermes on Android
2. Lazy load heavy screens
3. Cache API responses

### Issue: Large assets
**Solution**:
1. Use emoji instead of icons
2. Compress images with TinyPNG
3. Use WebP format
4. Remove unused assets

## Tools & Commands

```bash
# Analyze build
npm run build:web -- --stats

# Check dependency sizes
npm list --depth=0

# Clean rebuild
expo prebuild --clean && npm run build:android

# Estimate final APK
npm run build:android -- --dry-run

# Profile bundle
webpack-bundle-analyzer dist/bundle.html
```

## References

- [Expo Bundle Size](https://docs.expo.dev/guides/optimizing-bundle-size/)
- [React Native Performance](https://reactnative.dev/docs/optimizing-flatlist-configuration)
- [Android App Bundle](https://developer.android.com/guide/app-bundle)
- [bundletool Documentation](https://developer.android.com/studio/command-line/bundletool)
