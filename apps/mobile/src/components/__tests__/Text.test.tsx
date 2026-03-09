/**
 * Text Component Tests
 */
import React from 'react';
import { render } from '@testing-library/react-native';
import { Text } from '../Text';
import { Colors } from '@/theme/colors';

describe('Text Component', () => {
  it('renders correctly with default variant', () => {
    const { getByText } = render(<Text>Hello World</Text>);
    expect(getByText('Hello World')).toBeDefined();
  });

  it('renders with different variants', () => {
    const { getByText: getByText1 } = render(<Text variant="h1">Heading 1</Text>);
    expect(getByText1('Heading 1')).toBeDefined();

    const { getByText: getByText2 } = render(<Text variant="caption">Caption</Text>);
    expect(getByText2('Caption')).toBeDefined();
  });

  it('renders with custom color', () => {
    const { getByText } = render(
      <Text color={Colors.accent}>Accent Text</Text>
    );
    expect(getByText('Accent Text')).toBeDefined();
  });

  it('renders with custom weight', () => {
    const { getByText } = render(
      <Text weight="bold">Bold Text</Text>
    );
    expect(getByText('Bold Text')).toBeDefined();
  });
});
