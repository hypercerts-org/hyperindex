import { describe, expect, it } from "vitest";

import {
  isAdminDID,
  normalizePublicURL,
  parseAdminDIDs,
  resolvePublicClientURL,
  validateHyperindexURLConfiguration,
} from "./env";

describe("normalizePublicURL", () => {
  const cases: Array<{ input: string; expected: string }> = [
    { input: "", expected: "" },
    { input: "   ", expected: "" },
    { input: "my-app.vercel.app", expected: "https://my-app.vercel.app" },
    { input: "https://my-app.vercel.app", expected: "https://my-app.vercel.app" },
    { input: "http://localhost:3000", expected: "http://localhost:3000" },
    { input: "https://my-app.vercel.app/", expected: "https://my-app.vercel.app" },
    { input: " my-app.vercel.app/ ", expected: "https://my-app.vercel.app" },
  ];

  it.each(cases)("normalizes $input", ({ input, expected }: { input: string; expected: string }) => {
    expect(normalizePublicURL(input)).toBe(expected);
  });
});

describe("resolvePublicClientURL", () => {
  it("prefers NEXT_PUBLIC_CLIENT_URL when both are set", () => {
    expect(resolvePublicClientURL("custom.example.com", "branch.vercel.app")).toBe("https://custom.example.com");
  });

  it("falls back to branch url when client url is empty", () => {
    expect(resolvePublicClientURL("", "branch.vercel.app")).toBe("https://branch.vercel.app");
  });

  it("returns empty string when both are empty", () => {
    expect(resolvePublicClientURL("", "")).toBe("");
  });
});

describe("parseAdminDIDs", () => {
  it("parses comma-separated admin dids", () => {
    expect(parseAdminDIDs("did:plc:one, did:plc:two , ,did:plc:three")).toEqual([
      "did:plc:one",
      "did:plc:two",
      "did:plc:three",
    ]);
  });

  it("returns an empty array for empty input", () => {
    expect(parseAdminDIDs("")).toEqual([]);
  });
});

describe("isAdminDID", () => {
  it("matches trimmed user dids against admin dids", () => {
    expect(isAdminDID(" did:plc:admin ", ["did:plc:admin"])).toBe(true);
  });

  it("returns false for missing or unknown dids", () => {
    expect(isAdminDID(undefined, ["did:plc:admin"])).toBe(false);
    expect(isAdminDID("did:plc:user", ["did:plc:admin"])).toBe(false);
  });
});

describe("validateHyperindexURLConfiguration", () => {
  it("throws when the backend url matches the client origin", () => {
    expect(() =>
      validateHyperindexURLConfiguration("www.dev.hi.gainforest.app", "", "https://www.dev.hi.gainforest.app"),
    ).toThrow(/points to the client origin/);
  });

  it("allows different backend and client origins", () => {
    expect(() =>
      validateHyperindexURLConfiguration(
        "www.dev.hi.gainforest.app",
        "",
        "https://hyperindex-staging.up.railway.app",
      ),
    ).not.toThrow();
  });
});
