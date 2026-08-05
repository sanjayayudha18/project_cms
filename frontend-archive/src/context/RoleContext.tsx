import { createContext, useContext, useMemo, useState } from 'react';
import type { ReactNode } from 'react';

import { INTERNAL_ROLES } from '@/lib/constants';
import type { Role } from '@/lib/constants';

interface RoleContextValue {
  role: Role;
  setRole: (role: Role) => void;
  isInternal: boolean;
}

const defaultValue: RoleContextValue = {
  role: 'Admin',
  setRole: () => {},
  isInternal: true,
};

const RoleContext = createContext<RoleContextValue>(defaultValue);

// Track whether the provider is mounted
let providerMounted = false;

export function RoleProvider({ children }: { children: ReactNode }) {
  const [role, setRole] = useState<Role>('Admin');

  const value = useMemo<RoleContextValue>(() => {
    const isInternal = (INTERNAL_ROLES as readonly string[]).includes(role);
    return { role, setRole, isInternal };
  }, [role]);

  providerMounted = true;

  return <RoleContext.Provider value={value}>{children}</RoleContext.Provider>;
}

export function useRole(): RoleContextValue {
  const context = useContext(RoleContext);

  // In development, warn if consumed outside the provider
  if (import.meta.env.DEV && !providerMounted) {
    console.warn(
      '[useRole] RoleContext consumed outside of RoleProvider. Falling back to default (Admin).',
    );
  }

  return context;
}

export { RoleContext };
