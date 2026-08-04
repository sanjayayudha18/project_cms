import { render, screen } from "@testing-library/react";
import * as fc from "fast-check";
import { Monitor } from "lucide-react";
import { describe, expect, it, vi } from "vitest";
import { ModuleCard, type ModuleCardProps } from "./ModuleCard";

// Mock TanStack Router's Link component to avoid needing a full router context
vi.mock("@tanstack/react-router", () => ({
  Link: ({
    to,
    children,
    className,
  }: { to: string; children: React.ReactNode; className?: string }) => (
    <a href={to} className={className} data-testid="module-card-link">
      {children}
    </a>
  ),
}));

/**
 * Property 9: Module Landing Card Rendering
 * Validates: Requirements 5.2
 *
 * For any array of ModuleCard objects, rendering the module landing page SHALL produce
 * exactly one card element per entry in the array, and each card SHALL contain the
 * card's title text.
 */
describe("Property 9: Module Landing Card Rendering", () => {
  const arbModuleCardProps: fc.Arbitrary<ModuleCardProps> = fc.record({
    id: fc.uuid(),
    title: fc.string({ minLength: 1, unit: "grapheme" }).filter((s) => s.trim().length > 0),
    description: fc.string(),
    href: fc.string({ minLength: 1 }),
    icon: fc.constant(Monitor),
    disabled: fc.boolean(),
  });

  const arbModuleCardArray = fc.array(arbModuleCardProps, { minLength: 1, maxLength: 10 });

  it("renders exactly N card elements for an array of N ModuleCard items, each containing its title", () => {
    fc.assert(
      fc.property(arbModuleCardArray, (cards) => {
        const { unmount, container } = render(
          <div data-testid="card-grid">
            {cards.map((card) => (
              <ModuleCard key={card.id} {...card} />
            ))}
          </div>,
        );

        // Each card renders either a <div> (disabled) or an <a> (enabled via mocked Link)
        // Both contain an <h3> with the title
        const headings = container.querySelectorAll("h3");
        expect(headings).toHaveLength(cards.length);

        // Each card's title text must appear in its corresponding <h3>
        for (let i = 0; i < cards.length; i++) {
          expect(headings[i].textContent).toBe(cards[i].title);
        }

        unmount();
      }),
      { numRuns: 50 },
    );
  });
});

/**
 * Property 10: Disabled Card "Segera Hadir" Label
 * Validates: Requirements 5.6
 *
 * For any ModuleCard with disabled: true, rendering that card SHALL produce output
 * containing the text "Segera Hadir" and the card SHALL NOT have a functional
 * click/navigation handler (no <a> element).
 */
describe("Property 10: Disabled Card 'Segera Hadir' Label", () => {
  const arbDisabledCardProps: fc.Arbitrary<ModuleCardProps> = fc.record({
    id: fc.string({ minLength: 1 }),
    title: fc.string({ minLength: 1, unit: "grapheme" }).filter((s) => s.trim().length > 0),
    description: fc.string(),
    href: fc.string({ minLength: 1 }),
    icon: fc.constant(Monitor),
    disabled: fc.constant(true),
  });

  it("disabled card displays 'Segera Hadir' text and has no link element", () => {
    fc.assert(
      fc.property(arbDisabledCardProps, (cardProps) => {
        const { unmount, container } = render(<ModuleCard {...cardProps} />);

        // "Segera Hadir" badge must be visible
        expect(screen.getByText("Segera Hadir")).toBeInTheDocument();

        // No <a> element should be present (disabled card uses <div>, not <Link>)
        const links = container.querySelectorAll("a");
        expect(links).toHaveLength(0);

        // The card should have cursor-not-allowed style (rendered as a div)
        const cardDiv = container.firstElementChild;
        expect(cardDiv?.tagName).toBe("DIV");
        expect(cardDiv?.className).toContain("cursor-not-allowed");

        unmount();
      }),
      { numRuns: 50 },
    );
  });
});
