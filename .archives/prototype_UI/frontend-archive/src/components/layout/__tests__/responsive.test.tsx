import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

import { Sidebar } from '../Sidebar';
import { AppShell } from '@/app/AppShell';

/**
 * Responsive layout breakpoint logic tests.
 * Validates: Requirements 8.1, 8.2, 2.6
 *
 * - Sidebar overlay renders on mobile with scrim
 * - Scrim click closes sidebar (onMobileClose called)
 * - Hamburger visible <760px triggers menu open
 * - AppShell integration: hamburger → sidebar open → scrim → sidebar closed
 */

// Mock matchMedia for controlling viewport behavior
function mockMatchMedia(matches: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
}

// Mock react-router-dom's Outlet for AppShell tests
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    Outlet: () => <div data-testid="outlet">Page Content</div>,
    useLocation: () => ({ pathname: '/dashboard' }),
  };
});

describe('Responsive layout – Sidebar scrim behavior', () => {
  const defaultProps = {
    collapsed: false,
    onToggle: vi.fn(),
    mobileOpen: true,
    onMobileClose: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockMatchMedia(false);
  });

  it('renders backdrop scrim when mobileOpen is true', () => {
    const { container } = render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Sidebar {...defaultProps} />
      </MemoryRouter>
    );

    // The backdrop has bg-black/30 class and aria-hidden="true"
    const backdrop = container.querySelector('.fixed.inset-0 [aria-hidden="true"]');
    expect(backdrop).toBeInTheDocument();
    expect(backdrop).toHaveClass('bg-black/30');
  });

  it('calls onMobileClose when scrim backdrop is clicked', () => {
    const onMobileClose = vi.fn();
    const { container } = render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Sidebar {...defaultProps} onMobileClose={onMobileClose} />
      </MemoryRouter>
    );

    const backdrop = container.querySelector('.absolute.inset-0.bg-black\\/30');
    expect(backdrop).toBeInTheDocument();

    fireEvent.click(backdrop!);
    expect(onMobileClose).toHaveBeenCalledOnce();
  });

  it('does not render scrim when mobileOpen is false', () => {
    const { container } = render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Sidebar {...defaultProps} mobileOpen={false} />
      </MemoryRouter>
    );

    const backdrop = container.querySelector('.absolute.inset-0.bg-black\\/30');
    expect(backdrop).not.toBeInTheDocument();
  });
});

describe('Responsive layout – AppShell integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Simulate mobile viewport (< 1024px triggers collapse, important for sidebar state)
    mockMatchMedia(true);
  });

  it('hamburger click opens mobile sidebar overlay', () => {
    const { container } = render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AppShell />
      </MemoryRouter>
    );

    // Initially, no mobile overlay
    expect(container.querySelector('.fixed.inset-0')).not.toBeInTheDocument();

    // Click hamburger button
    const hamburger = screen.getByRole('button', { name: 'Open menu' });
    fireEvent.click(hamburger);

    // Mobile overlay should now be visible
    expect(container.querySelector('.fixed.inset-0')).toBeInTheDocument();
  });

  it('scrim click closes mobile sidebar overlay', () => {
    const { container } = render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AppShell />
      </MemoryRouter>
    );

    // Open the mobile menu first
    const hamburger = screen.getByRole('button', { name: 'Open menu' });
    fireEvent.click(hamburger);

    // Verify overlay is visible
    expect(container.querySelector('.fixed.inset-0')).toBeInTheDocument();

    // Click the scrim/backdrop to close
    const backdrop = container.querySelector('.absolute.inset-0.bg-black\\/30');
    expect(backdrop).toBeInTheDocument();
    fireEvent.click(backdrop!);

    // Overlay should be removed
    expect(container.querySelector('.fixed.inset-0')).not.toBeInTheDocument();
  });

  it('hamburger toggles sidebar: open → close → open', () => {
    const { container } = render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <AppShell />
      </MemoryRouter>
    );

    const hamburger = screen.getByRole('button', { name: 'Open menu' });

    // Open
    fireEvent.click(hamburger);
    expect(container.querySelector('.fixed.inset-0')).toBeInTheDocument();

    // Close via hamburger toggle
    fireEvent.click(hamburger);
    expect(container.querySelector('.fixed.inset-0')).not.toBeInTheDocument();

    // Open again
    fireEvent.click(hamburger);
    expect(container.querySelector('.fixed.inset-0')).toBeInTheDocument();
  });
});
