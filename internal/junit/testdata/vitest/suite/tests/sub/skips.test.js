import { test, expect } from "vitest";

test("passes", () => {
  expect(true).toBe(true);
});

test.skip("skipped", () => {});
