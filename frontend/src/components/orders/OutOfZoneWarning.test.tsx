import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { OutOfZoneWarning } from './OutOfZoneWarning';

describe('OutOfZoneWarning', () => {
  it('renders nothing when show is false', () => {
    const { container } = render(<OutOfZoneWarning show={false} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders text + alert role when show is true', () => {
    render(<OutOfZoneWarning show={true} />);
    expect(
      screen.getByText('Доставка доступна только по Москве и МО'),
    ).toBeInTheDocument();
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });
});
