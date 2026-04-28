import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { renderWithProviders } from '@/test/test-utils';
import { server } from '@/test/mocks/server';
import { AddressSearchInput } from './AddressSearchInput';

describe('AddressSearchInput', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('matches placeholder text from UI-SPEC', () => {
    const onSelect = vi.fn();
    renderWithProviders(<AddressSearchInput onSelect={onSelect} />);
    expect(
      screen.getByPlaceholderText('Поиск адреса в Москве...'),
    ).toBeInTheDocument();
  });

  it('does not fetch suggestions for queries shorter than 3 chars', async () => {
    const user = userEvent.setup({
      advanceTimers: vi.advanceTimersByTime,
    });
    const onSelect = vi.fn();
    renderWithProviders(<AddressSearchInput onSelect={onSelect} />);

    const input = screen.getByPlaceholderText('Поиск адреса в Москве...');
    await user.type(input, 'Тв');

    await act(async () => {
      vi.advanceTimersByTime(700);
    });

    expect(screen.queryByText(/Москва, Тверская/)).not.toBeInTheDocument();
  });

  it('fetches suggestions after 500ms debounce', async () => {
    const user = userEvent.setup({
      advanceTimers: vi.advanceTimersByTime,
    });
    const onSelect = vi.fn();
    renderWithProviders(<AddressSearchInput onSelect={onSelect} />);

    const input = screen.getByPlaceholderText('Поиск адреса в Москве...');
    await user.type(input, 'Тверская');

    // Right at 499ms — debounce not flushed yet
    await act(async () => {
      vi.advanceTimersByTime(499);
    });
    expect(screen.queryByText(/Тверская, 1/)).not.toBeInTheDocument();

    // Cross threshold and let MSW respond
    await act(async () => {
      vi.advanceTimersByTime(1);
    });

    await waitFor(
      () => {
        expect(
          screen.getByText(/Москва, Тверская, 1/),
        ).toBeInTheDocument();
      },
      { timeout: 3000 },
    );
  });

  it('clicking suggestion fires onSelect with parsed lat/lon/displayName', async () => {
    const user = userEvent.setup({
      advanceTimers: vi.advanceTimersByTime,
    });
    const onSelect = vi.fn();
    renderWithProviders(<AddressSearchInput onSelect={onSelect} />);

    const input = screen.getByPlaceholderText('Поиск адреса в Москве...');
    await user.type(input, 'Тверская');

    await act(async () => {
      vi.advanceTimersByTime(500);
    });

    const suggestion = await screen.findByText(/Москва, Тверская, 1/);
    await user.click(suggestion);

    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect).toHaveBeenCalledWith({
      lat: 55.7558,
      lon: 37.6173,
      displayName: 'Москва, Тверская, 1',
    });
  });

  it('shows "Адрес не найден" when results empty', async () => {
    server.use(
      http.get('https://nominatim.openstreetmap.org/search', () =>
        HttpResponse.json([]),
      ),
    );

    const user = userEvent.setup({
      advanceTimers: vi.advanceTimersByTime,
    });
    const onSelect = vi.fn();
    renderWithProviders(<AddressSearchInput onSelect={onSelect} />);

    const input = screen.getByPlaceholderText('Поиск адреса в Москве...');
    await user.type(input, 'ХХХХ');

    await act(async () => {
      vi.advanceTimersByTime(500);
    });

    await waitFor(() => {
      expect(screen.getByText('Адрес не найден')).toBeInTheDocument();
    });
  });
});
