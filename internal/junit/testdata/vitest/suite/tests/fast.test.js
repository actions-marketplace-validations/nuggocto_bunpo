import { test, expect } from "vitest";

test("one", () => {
  expect(1 + 1).toBe(2);
});

test("two", () => {
  expect("a".toUpperCase()).toBe("A");
});
