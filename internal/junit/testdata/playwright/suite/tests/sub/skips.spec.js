const { test, expect } = require("@playwright/test");

test("passes", () => {
  expect(true).toBe(true);
});

test.skip("skipped", () => {});
