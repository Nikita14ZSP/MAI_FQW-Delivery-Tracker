import type { ReactElement, ReactNode } from 'react';
import { render, type RenderOptions } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { AuthContext, type AuthContextValue } from '@/contexts/AuthContext';
import type { User } from '@/types/auth';

export interface ProviderOptions extends Omit<RenderOptions, 'wrapper'> {
  route?: string;
  user?: User | null;
  authOverrides?: Partial<AuthContextValue>;
  queryClient?: QueryClient;
}

export function renderWithProviders(
  ui: ReactElement,
  opts: ProviderOptions = {},
) {
  const {
    route = '/',
    user = null,
    authOverrides = {},
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    }),
    ...rtlOpts
  } = opts;

  const authValue: AuthContextValue = {
    user,
    loading: false,
    login: async () => user as User,
    register: async () => user as User,
    logout: () => {},
    ...authOverrides,
  };

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[route]}>
          <AuthContext.Provider value={authValue}>
            {children}
          </AuthContext.Provider>
        </MemoryRouter>
      </QueryClientProvider>
    );
  }
  return { ...render(ui, { wrapper: Wrapper, ...rtlOpts }), queryClient };
}
