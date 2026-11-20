const { test, expect } = require("@playwright/test");

test("waits", async () => {
  await new Promise((resolve) => setTimeout(resolve, 150));
});
