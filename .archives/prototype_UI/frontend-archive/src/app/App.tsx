import { BrowserRouter, useRoutes } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';

import { queryClient } from '@/lib/queryClient';
import { RoleProvider } from '@/context/RoleContext';
import { AppShell } from './AppShell';
import { routes } from './routes';

function AppRoutes() {
  // Wrap all routes in the AppShell layout route
  const element = useRoutes([
    {
      element: <AppShell />,
      children: routes,
    },
  ]);
  return element;
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <RoleProvider>
        <BrowserRouter>
          <AppRoutes />
        </BrowserRouter>
      </RoleProvider>
    </QueryClientProvider>
  );
}
