// @vitest-environment node
// BE-10 T5: schema.d.ts must equal what `pnpm run gen:api` produces from backend/api/openapi.yaml.
import { readFileSync } from "node:fs";
import openapiTS, { COMMENT_HEADER, astToString } from "openapi-typescript";

const spec = new URL("../../../../../backend/api/openapi.yaml", import.meta.url);
const generated = new URL("./schema.d.ts", import.meta.url);
const lf = (s: string) => s.replace(/\r\n/g, "\n");

describe("OpenAPI generated types", () => {
  it("schema.d.ts is up to date with backend/api/openapi.yaml", async () => {
    const expected = `${COMMENT_HEADER}${astToString(await openapiTS(spec))}`;
    const actual = readFileSync(generated, "utf8");
    expect(
      lf(actual) === lf(expected),
      "schema.d.ts is stale: run `pnpm run gen:api` in frontend/CompanyPortal-Vite",
    ).toBe(true);
  });
});
