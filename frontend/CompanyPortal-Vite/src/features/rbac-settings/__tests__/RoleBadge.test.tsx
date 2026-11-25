import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RoleBadge } from "../components/RoleBadge";
import { roleBadgeVariant } from "../types";

describe("RoleBadge (spec role-management-revamp 8.2, 8.3)", () => {
  it("shows an unknown role by name with the neutral variant", () => {
    render(<RoleBadge {...{ role: "AUDITOR-NEW" }} />);

    const badge = screen.getByText("AUDITOR-NEW");
    expect(badge).toHaveClass("bg-[var(--n-100)]");
    expect(roleBadgeVariant("AUDITOR-NEW")).toBe("neutral");
  });

  it("treats any VENDOR- role as a vendor role, even one created at runtime", () => {
    render(<RoleBadge {...{ role: "VENDOR-NEW-USER" }} />);

    expect(screen.getByText("VENDOR-NEW-USER")).toHaveClass("bg-[var(--warning-bg)]");
  });

  it("keeps the admin roles on the info variant", () => {
    expect(roleBadgeVariant("ADMIN")).toBe("info");
    expect(roleBadgeVariant("APPACCESS")).toBe("info");
  });
});
