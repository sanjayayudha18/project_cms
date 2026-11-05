import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { TopBar } from '../TopBar';

describe('TopBar', () => {
  it('renders search input with correct placeholder', () => {
    render(<TopBar onMenuClick={() => {}} />);

    const input = screen.getByPlaceholderText('Search ATM, vendor, invoice...');
    expect(input).toBeInTheDocument();
  });

  it('focuses search input on Cmd+K shortcut', () => {
    render(<TopBar onMenuClick={() => {}} />);

    const input = screen.getByPlaceholderText('Search ATM, vendor, invoice...');
    fireEvent.keyDown(document, { key: 'k', metaKey: true });

    expect(document.activeElement).toBe(input);
  });

  it('focuses search input on Ctrl+K shortcut', () => {
    render(<TopBar onMenuClick={() => {}} />);

    const input = screen.getByPlaceholderText('Search ATM, vendor, invoice...');
    fireEvent.keyDown(document, { key: 'k', ctrlKey: true });

    expect(document.activeElement).toBe(input);
  });

  it('calls onMenuClick when hamburger button is clicked', () => {
    const onMenuClick = vi.fn();
    render(<TopBar onMenuClick={onMenuClick} />);

    const hamburger = screen.getByRole('button', { name: 'Open menu' });
    fireEvent.click(hamburger);

    expect(onMenuClick).toHaveBeenCalledOnce();
  });

  it('displays correct initials "RB" in the avatar', () => {
    render(<TopBar onMenuClick={() => {}} />);

    expect(screen.getByText('RB')).toBeInTheDocument();
  });

  it('renders full name "Raden Budiman"', () => {
    render(<TopBar onMenuClick={() => {}} />);

    expect(screen.getByText('Raden Budiman')).toBeInTheDocument();
  });

  it('renders notification bell with unread dot indicator', () => {
    render(<TopBar onMenuClick={() => {}} />);

    const bell = screen.getByRole('button', { name: 'Notifications (unread)' });
    expect(bell).toBeInTheDocument();
    // The dot indicator is a child span (aria-hidden)
    const dot = bell.querySelector('span');
    expect(dot).toBeInTheDocument();
  });
});
