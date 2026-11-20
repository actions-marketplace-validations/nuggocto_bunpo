const { defineConfig } = require("@playwright/test");

module.exports = defineConfig({
  testDir: "tests",
  reporter: [["junit", { outputFile: "report.xml" }]],
});
