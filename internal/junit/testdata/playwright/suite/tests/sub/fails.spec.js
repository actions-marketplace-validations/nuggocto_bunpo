const { test, expect } = require("@playwright/test");

test("passes", () => {
  expect(true).toBe(true);
});

test("fails", () => {
  expect(1).toBe(2);
});
