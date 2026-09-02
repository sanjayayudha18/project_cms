import { EmptyState } from "@/components/ui/EmptyState";
import { render, screen } from "@testing-library/react";
import { FileSearch } from "lucide-react";
import { describe, expect, it } from "vitest";

describe("EmptyState", () => {
  it("renders title text", () => {
    render(<EmptyState title="No orders found" />);

    expect(screen.getByText("No orders found")).toBeInTheDocument();
  });

  it("renders description when provided", () => {
    render(
      <EmptyState
        title="No data"
        description="Try adjusting your filters to find what you are looking for."
      />,
    );

    expect(screen.getByText("No data")).toBeInTheDocument();
    expect(
      screen.getByText("Try adjusting your filters to find what you are looking for."),
    ).toBeInTheDocument();
  });

  it("does not render description when not provided", () => {
    const { container } = render(<EmptyState title="Empty" />);

    expect(screen.getByText("Empty")).toBeInTheDocument();
    // No paragraph element for description
    const paragraphs = container.querySelectorAll("p");
    expect(paragraphs).toHaveLength(0);
  });

  it("renders default icon (Inbox) when none specified", () => {
    const { container } = render(<EmptyState title="No items" />);

    // Default icon is Inbox — an SVG should be rendered
    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveAttribute("aria-hidden", "true");
  });

  it("renders custom icon when provided", () => {
    const { container } = render(<EmptyState title="No results" icon={FileSearch} />);

    // Custom icon (FileSearch) should render as SVG
    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveAttribute("aria-hidden", "true");
  });
});
