/**
 * Text Component
 * Reusable text component with typography variants
 */
import React from 'react';
import {
  Text as RNText,
  TextProps as RNTextProps,
  StyleSheet,
} from 'react-native';
import { Colors } from '@/theme/colors';
import { Typography } from '@/theme/typography';

export type TextVariant = 'h1' | 'h2' | 'h3' | 'body' | 'caption' | 'label';

interface TextProps extends RNTextProps {
  variant?: TextVariant;
  color?: string;
  weight?: keyof typeof Typography.weight;
}

const styles = StyleSheet.create({
  text: {
    fontFamily: 'System',
  },
  h1: {
    fontSize: Typography.size['4xl'],
    fontWeight: Typography.weight.extrabold,
    lineHeight: Typography.size['4xl'] * Typography.lineHeight.tight,
  },
  h2: {
    fontSize: Typography.size['3xl'],
    fontWeight: Typography.weight.bold,
    lineHeight: Typography.size['3xl'] * Typography.lineHeight.tight,
  },
  h3: {
    fontSize: Typography.size['2xl'],
    fontWeight: Typography.weight.semibold,
    lineHeight: Typography.size['2xl'] * Typography.lineHeight.normal,
  },
  body: {
    fontSize: Typography.size.base,
    fontWeight: Typography.weight.normal,
    lineHeight: Typography.size.base * Typography.lineHeight.normal,
  },
  caption: {
    fontSize: Typography.size.sm,
    fontWeight: Typography.weight.normal,
    lineHeight: Typography.size.sm * Typography.lineHeight.normal,
  },
  label: {
    fontSize: Typography.size.sm,
    fontWeight: Typography.weight.semibold,
    lineHeight: Typography.size.sm * Typography.lineHeight.normal,
  },
});

export const Text: React.FC<TextProps> = ({
  variant = 'body',
  color = Colors.text,
  weight,
  style,
  ...props
}) => {
  const variantStyle = styles[variant];
  const fontWeight = weight
    ? { fontWeight: Typography.weight[weight] }
    : {};

  return (
    <RNText
      {...props}
      style={[
        styles.text,
        variantStyle,
        { color },
        fontWeight,
        style,
      ]}
    />
  );
};
