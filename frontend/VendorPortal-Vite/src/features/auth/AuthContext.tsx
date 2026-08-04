import {
  createContext,
  useCallback,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import type { AuthState, JwtPayload, VendorUser } from '@/lib/types';
import { queryClient } from '@/lib/queryClient';
import vendorsData from '@/data/vendors.json';

export interface AuthContextValue {
  readonly state: AuthState;
  login(username: string, password: string): Promise<void>;
  logout(): void;
}

export const AuthContext = createContext<AuthContextValue | null>(null);

function encodeJwt(payload: JwtPayload): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const body = btoa(JSON.stringify(payload));
  const signature = btoa('simulated-signature');
  return `${header}.${body}.${signature}`;
}

function decodeJwtPayload(token: string): JwtPayload | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    const body = parts[1];
    if (!body) return null;
    const payload = JSON.parse(atob(body)) as JwtPayload;
    if (!payload.vendor_id) return null;
    return payload;
  } catch {
    return null;
  }
}

function buildUserFromPayload(payload: JwtPayload): VendorUser {
  return {
    id: payload.sub,
    username: '', // Not stored in JWT
    displayName: payload.display_name,
    vendorId: payload.vendor_id,
    vendorName: payload.vendor_name,
    role: payload.role,
  };
}

interface AuthProviderProps {
  readonly children: ReactNode;
}

export function AuthProvider({ children }: AuthProviderProps) {
  const [token, setToken] = useState<string | null>(null);
  const [user, setUser] = useState<VendorUser | null>(null);

  const login = useCallback(async (username: string, password: string) => {
    // Simulate async network call
    await Promise.resolve();

    const matchedUser = vendorsData.find(
      (u) => u.username === username && u.password === password,
    );

    if (!matchedUser) {
      throw new Error('Username atau password salah');
    }

    const now = Math.floor(Date.now() / 1000);
    const payload: JwtPayload = {
      sub: matchedUser.id,
      auth_source: 'local',
      role: 'Vendor',
      vendor_id: matchedUser.vendorId,
      vendor_name: matchedUser.vendorName,
      display_name: matchedUser.displayName,
      exp: now + 3600, // 1 hour expiry
      iat: now,
    };

    const jwt = encodeJwt(payload);

    // Decode and validate the JWT contains vendor_id
    const decoded = decodeJwtPayload(jwt);
    if (!decoded?.vendor_id) {
      throw new Error('Sesi tidak valid. Silakan login kembali.');
    }

    const vendorUser = buildUserFromPayload(decoded);
    setToken(jwt);
    setUser(vendorUser);
  }, []);

  const logout = useCallback(() => {
    setToken(null);
    setUser(null);
    queryClient.clear();
  }, []);

  const state: AuthState = useMemo(
    () => ({
      token,
      user,
      isAuthenticated: token !== null && user !== null,
    }),
    [token, user],
  );

  const value: AuthContextValue = useMemo(
    () => ({ state, login, logout }),
    [state, login, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
