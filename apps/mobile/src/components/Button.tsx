/**
 * Button Component
 * Primary, secondary, and tertiary button variants
 */
import React from 'react';
import {
  TouchableOpacity,
  TouchableOpacityProps,
  StyleSheet,
} from 'react-native';
import { Colors } from '@/theme/colors';
import { Spacing } from '@/theme/spacing';
import { Text } from './Text';

export type ButtonVariant = 'primary' | 'secondary' | 'tertiary';

interface ButtonProps extends TouchableOpacityProps {
  label: string;
  variant?: ButtonVariant;
  size?: 'sm' | 'md' | 'lg';
  disabled?: boolean;
  loading?: boolean;
  onPress: () => void;
}

const styles = StyleSheet.create({
  button: {
    borderRadius: 12,
    alignItems: 'center',
    justifyContent: 'center',
    flexDirection: 'row',
  },
  sm: {
    paddingHorizontal: Spacing[3],
    paddingVertical: Spacing[2],
  },
  md: {
    paddingHorizontal: Spacing[4],
    paddingVertical: Spacing[3],
  },
  lg: {
    paddingHorizontal: Spacing[6],
    paddingVertical: Spacing[4],
  },
  primary: {
    backgroundColor: Colors.accent,
  },
  secondary: {
    backgroundColor: Colors.surface2,
    borderWidth: 1,
    borderColor: Colors.border,
  },
  tertiary: {
    backgroundColor: 'transparent',
  },
  disabled: {
    opacity: 0.5,
  },
});

export const Button: React.FC<ButtonProps> = ({
  label,
  variant = 'primary',
  size = 'md',
  disabled = false,
  style,
  ...props
}) => {
  const variantStyle = styles[variant];
  const sizeStyle = styles[size];

  return (
    <TouchableOpacity
      {...props}
      disabled={disabled}
      style={[
        styles.button,
        variantStyle,
        sizeStyle,
        disabled && styles.disabled,
        style,
      ]}
    >
      <Text
        variant="label"
        color={variant === 'primary' ? Colors.bg : Colors.text}
        weight="semibold"
      >
        {label}
      </Text>
    </TouchableOpacity>
  );
};
