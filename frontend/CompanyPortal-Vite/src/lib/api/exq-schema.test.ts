// @vitest-environment node
// 12.2: exq-schema.d.ts must equal what `pnpm run gen:api:exq` produces from backend-exq/api/openapi.yaml.
import { readFileSync } from "node:fs";
import openapiTS, { COMMENT_HEADER, astToString } from "openapi-typescript";

const spec = new URL("../../../../../backend-exq/api/openapi.yaml", import.meta.url);
const generated = new URL("./exq-schema.d.ts", import.meta.url);
const lf = (s: string) => s.replace(/\r\n/g, "\n");

describe("OpenAPI generated types", () => {
  it("exq-schema.d.ts is up to date with backend-exq/api/openapi.yaml", async () => {
    const expected = `${COMMENT_HEADER}${astToString(await openapiTS(spec))}`;
    const actual = readFileSync(generated, "utf8");
    expect(
      lf(actual) === lf(expected),
      "exq-schema.d.ts is stale: run `pnpm run gen:api:exq` in frontend/CompanyPortal-Vite",
    ).toBe(true);
  });
});
