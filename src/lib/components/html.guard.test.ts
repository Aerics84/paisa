import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

describe("untrusted HTML", () => {
  test("provider/help/subtitle fields are not rendered with @html", () => {
    for (const file of ["LevelItem.svelte", "PriceCodeSearchModal.svelte"]) {
      const source = readFileSync(join(__dirname, file), "utf-8");
      expect(source).not.toContain("{@html subtitle}");
      expect(source).not.toContain("{@html selectedProvider.description}");
      expect(source).not.toContain("{@html field.help}");
    }
  });
});
