import notificationsData from "@/data/notifications.json";
import type { Notification } from "@/lib/types";
import * as fc from "fast-check";

/**
 * Property 13: Notification Type Routing
 * Validates: Requirements 8.4
 *
 * For any notification with a type field, the mapped navigation target should be
 * deterministic: "New Assignment" → "/orders", "Order Status Changed" → "/orders",
 * "Invoice Status Updated" → "/invoices", "Schedule Updated" → "/schedule".
 * No notification type should map to an undefined route.
 */

type NotificationType =
  | "New Assignment"
  | "Order Status Changed"
  | "Invoice Status Updated"
  | "Schedule Updated";

const typeToRoute: Record<NotificationType, string> = {
  "New Assignment": "/orders",
  "Order Status Changed": "/orders",
  "Invoice Status Updated": "/invoices",
  "Schedule Updated": "/schedule",
};

const VALID_ROUTES = ["/orders", "/invoices", "/schedule"];

function getRouteForNotificationType(type: NotificationType): string {
  return typeToRoute[type];
}

const notificationTypeArb = fc.constantFrom<NotificationType>(
  "New Assignment",
  "Order Status Changed",
  "Invoice Status Updated",
  "Schedule Updated",
);

describe("Property 13: Notification Type Routing", () => {
  it("each notification type maps to a defined route (never undefined)", () => {
    fc.assert(
      fc.property(notificationTypeArb, (type) => {
        const route = getRouteForNotificationType(type);
        expect(route).toBeDefined();
        expect(typeof route).toBe("string");
        expect(route.length).toBeGreaterThan(0);
      }),
      { numRuns: 100 },
    );
  });

  it("the mapping is deterministic (same type always produces same route)", () => {
    fc.assert(
      fc.property(notificationTypeArb, (type) => {
        const route1 = getRouteForNotificationType(type);
        const route2 = getRouteForNotificationType(type);
        expect(route1).toBe(route2);
      }),
      { numRuns: 100 },
    );
  });

  it("verifies specific mappings: New Assignment → /orders, Order Status Changed → /orders, Invoice Status Updated → /invoices, Schedule Updated → /schedule", () => {
    fc.assert(
      fc.property(notificationTypeArb, (type) => {
        const route = getRouteForNotificationType(type);
        switch (type) {
          case "New Assignment":
            expect(route).toBe("/orders");
            break;
          case "Order Status Changed":
            expect(route).toBe("/orders");
            break;
          case "Invoice Status Updated":
            expect(route).toBe("/invoices");
            break;
          case "Schedule Updated":
            expect(route).toBe("/schedule");
            break;
        }
      }),
      { numRuns: 100 },
    );
  });

  it("the mapped route is always one of the defined portal routes", () => {
    fc.assert(
      fc.property(notificationTypeArb, (type) => {
        const route = getRouteForNotificationType(type);
        expect(VALID_ROUTES).toContain(route);
      }),
      { numRuns: 100 },
    );
  });

  describe("mock data validation", () => {
    it("every notification in mock data has a relatedRoute matching the expected mapping from its type", () => {
      const notifications = notificationsData as Notification[];

      for (const notification of notifications) {
        const expectedRoute = getRouteForNotificationType(notification.type as NotificationType);
        expect(notification.relatedRoute).toBe(expectedRoute);
      }
    });

    it("all notification types in mock data are valid types with defined routes", () => {
      const notifications = notificationsData as Notification[];
      const validTypes: readonly string[] = Object.keys(typeToRoute);

      for (const notification of notifications) {
        expect(validTypes).toContain(notification.type);
      }
    });
  });
});
