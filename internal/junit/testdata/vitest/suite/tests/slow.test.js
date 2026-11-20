import { test, expect } from "vitest";

test("waits", async () => {
  await new Promise((resolve) => setTimeout(resolve, 150));
});
