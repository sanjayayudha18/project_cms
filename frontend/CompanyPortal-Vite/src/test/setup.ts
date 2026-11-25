import "@testing-library/jest-dom/vitest";
import * as axeMatchers from "vitest-axe/matchers";
import "vitest-axe/extend-expect";
import { expect } from "vitest";

expect.extend(axeMatchers);
