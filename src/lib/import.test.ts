import "../happydom";
import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, expect, test } from "bun:test";

import { parse, render, asRows } from "./spreadsheet";
import fs from "fs";
import helpers from "./template_helpers";
import _ from "lodash";
import Handlebars from "handlebars";
import dayjs from "dayjs";
import customParseFormat from "dayjs/plugin/customParseFormat";
dayjs.extend(customParseFormat);
import isSameOrBefore from "dayjs/plugin/isSameOrBefore";
dayjs.extend(isSameOrBefore);
import utc from "dayjs/plugin/utc";
import timezone from "dayjs/plugin/timezone"; // dependent on utc plugin
dayjs.extend(utc);
dayjs.extend(timezone);
import localeData from "dayjs/plugin/localeData";
dayjs.extend(localeData);
import updateLocale from "dayjs/plugin/updateLocale";
dayjs.extend(updateLocale);

function normalizeLineEndings(value: string) {
  return value.replace(/\r\n/g, "\n");
}

function toBlobPart(input: Buffer) {
  return new Uint8Array(input);
}

// Import fixtures from the Germany/EU corpus use locale-specific decimal
// separators (e.g. "1.234,56"), so they must be rendered with that locale.
const FIXTURE_LOCALES: Record<string, string> = {
  "MT940 Account Statement": "de-DE",
  "Trade Republic Cash CSV": "de-DE",
  "Trade Republic Trades CSV": "de-DE"
};

Handlebars.registerHelper(
  _.mapValues(helpers, (helper, name) => {
    return function (...args: any[]) {
      try {
        return helper.apply(this, args);
      } catch (e) {
        console.log("Error in helper", name, args, e);
      }
    };
  })
);

describe("import", () => {
  fs.readdirSync("fixture/import").forEach((dir) => {
    test(dir, async () => {
      // JSON files in a fixture directory are goldens (e.g. monthly.rows.json),
      // not import inputs.
      const files = fs
        .readdirSync(`fixture/import/${dir}`)
        .filter((file) => !file.endsWith(".json"));
      for (const file of files) {
        const [name, extension] = file.split(".");
        if (extension === "ledger") {
          const inputFile = _.find(files, (f) => f != file && f.startsWith(name));
          if (inputFile.endsWith(".pdf")) {
            break;
          }
          const input = fs.readFileSync(`fixture/import/${dir}/${inputFile}`);
          const output = fs.readFileSync(`fixture/import/${dir}/${file}`).toString();
          const template = fs
            .readFileSync(`internal/model/template/templates/${dir}.handlebars`)
            .toString();

          const compiled = Handlebars.compile(template);
          const result = await parse(new File([toBlobPart(input)], inputFile));
          if (inputFile.endsWith(".xls")) {
            expect(result.error).toBe(
              "Legacy .xls files are no longer supported. Please re-save the file as .xlsx or CSV."
            );
            return;
          }
          const rows = asRows(result);

          const previousConfig = globalThis.USER_CONFIG;
          globalThis.USER_CONFIG = {
            ...previousConfig,
            locale: FIXTURE_LOCALES[dir] ?? "en-IN"
          };
          try {
            const actual = render(rows, compiled, { trim: true });

            expect(normalizeLineEndings(actual)).toBe(normalizeLineEndings(_.trim(output)));
          } finally {
            globalThis.USER_CONFIG = previousConfig;
          }
        }
      }
    });
  });
});

describe("broker csv normalization", () => {
  test("trade republic trades expose normalized semantic fields", async () => {
    const input = fs.readFileSync("fixture/import/Trade Republic Trades CSV/trades.csv");
    const result = await parse(new File([toBlobPart(input)], "trades.csv"));
    const rows = asRows(result);

    expect(rows[0].broker).toBe("Trade Republic");
    expect(rows[0].importType).toBe("broker-trade");
    expect(rows[0].transactionKind).toBe("buy");
    expect(rows[0].tradeDate).toBe("28.04.2026");
    expect(rows[0].settlementDate).toBe("30.04.2026");
    expect(rows[0].symbol).toBe("VWCE");
    expect(rows[0].principal).toBe("191,00");
    expect(rows[0].feeAmount).toBe("1,00");
    expect(rows[0].taxAmount).toBe("0,00");
    expect(rows[0].netCashAmount).toBe("-190");
  });

  test("trade republic cash exposes normalized cash-flow fields", async () => {
    const input = fs.readFileSync("fixture/import/Trade Republic Cash CSV/cash.csv");
    const result = await parse(new File([toBlobPart(input)], "cash.csv"));
    const rows = asRows(result);

    expect(rows[0].broker).toBe("Trade Republic");
    expect(rows[0].importType).toBe("broker-cash");
    expect(rows[0].transactionKind).toBe("dividend");
    expect(rows[0].cashDate).toBe("30.04.2026");
    expect(rows[0].cashAmount).toBe("45,67");
    expect(rows[0].symbol).toBe("VWCE");
  });

  test("scalable capital trades expose normalized semantic fields", async () => {
    const input = fs.readFileSync("fixture/import/Scalable Capital Trades CSV/trades.csv");
    const result = await parse(new File([toBlobPart(input)], "trades.csv"));
    const rows = asRows(result);

    expect(rows[0].broker).toBe("Scalable Capital");
    expect(rows[0].importType).toBe("broker-trade");
    expect(rows[0].transactionKind).toBe("buy");
    expect(rows[0].tradeDate).toBe("2026-04-28");
    expect(rows[0].settlementDate).toBe("2026-04-30");
    expect(rows[0].quantity).toBe("2");
    expect(rows[0].principal).toBe("191.00");
    expect(rows[1].quantity).toBe("1");
    expect(rows[1].netCashAmount).toBe("148.03");
  });

  test("unsupported broker csv stays unnormalized", async () => {
    const csv = ["Booked At,Action,Amount", "2026-04-30,Dividend,45.67"].join("\n");

    const result = await parse(new File([csv], "unsupported.csv"));
    const rows = asRows(result);

    expect(rows[0].broker).toBeUndefined();
    expect(rows[0].transactionKind).toBeUndefined();
    expect(rows[0].A).toBe("Booked At");
    expect(rows[1].A).toBe("2026-04-30");
  });
});

describe("template helpers", () => {
  test("acronym", () => {
    expect(helpers.acronym("Foo Bar baz")).toBe("FBB");
    expect(helpers.acronym("foo   the bar")).toBe("FB");
    expect(helpers.acronym("Motital S & P 500")).toBe("MSP");
    expect(helpers.acronym("Axis Liquid Growth Direct Plan")).toBe("AL");
  });

  test("amount helper normalizes comma decimals", () => {
    // Comma decimals are only unambiguous in a comma-decimal locale, while
    // lakh grouping is specific to en-IN. Both must keep working.
    const previousConfig = globalThis.USER_CONFIG;
    const withLocale = (locale: string, fn: () => void) => {
      globalThis.USER_CONFIG = { ...previousConfig, locale };
      try {
        fn();
      } finally {
        globalThis.USER_CONFIG = previousConfig;
      }
    };

    withLocale("de-DE", () => {
      expect(String(helpers.amount("92,33", { hash: {} }))).toBe("92.33");
      expect(String(helpers.amount("2,707819", { hash: {} }))).toBe("2.707819");
      expect(String(helpers.amount("1.234,56", { hash: {} }))).toBe("1234.56");
      expect(String(helpers.amount("10.005,05 EUR", { hash: {} }))).toBe("10005.05");
    });

    withLocale("en-IN", () => {
      expect(String(helpers.amount("100,000", { hash: {} }))).toBe("100000");
      expect(String(helpers.amount("1,00,000", { hash: {} }))).toBe("100000");
    });
  });

  test("date helper normalizes day-first dates", () => {
    expect(helpers.date("28.04.2026", "DD.MM.YYYY")).toBe("2026/04/28");
  });

  test("negate preserves decimal notation", () => {
    expect(String(helpers.negate("0.00000001"))).toBe("-0.00000001");
    expect(String(helpers.negate(0.00000001))).toBe("-0.00000001");
    expect(String(helpers.negate("-0.00000001"))).toBe("0.00000001");
    expect(String(helpers.negate("₹ 1,234.56"))).toBe("-1234.56");
    expect(String(helpers.negate(0))).toBe("0");
  });
});

describe("numeric template helpers", () => {
  let originalConfig: UserConfig;

  beforeEach(() => {
    originalConfig = globalThis.USER_CONFIG;
    globalThis.USER_CONFIG = { ...originalConfig, locale: "en-IN", amount_alignment_column: 52 };
  });

  afterEach(() => {
    globalThis.USER_CONFIG = originalConfig;
  });

  const evaluate = (template: string, value: string | number = "") =>
    Handlebars.compile(template)({ value });

  for (const [locale, value] of [
    ["en-IN", "₹ 1,234.56"],
    ["en-US", "$1,234.56"],
    ["de-DE", "1.234,56 €"],
    ["fr-FR", "1\u202f234,56 €"]
  ]) {
    test(`numeric helpers use ${locale}`, () => {
      USER_CONFIG.locale = locale;
      assert.strictEqual(evaluate("{{amount value}}", value), "1234.56");
      assert.strictEqual(evaluate("{{negate value}}", value), "-1234.56");
      assert.strictEqual(evaluate("{{round value precision=1}}", value), "1234.6");
      assert.strictEqual(evaluate("{{gt value 1234}}", value), "true");
      assert.strictEqual(evaluate("{{gte value 1234.56}}", value), "true");
      assert.strictEqual(evaluate("{{lt value 1235}}", value), "true");
      assert.strictEqual(evaluate("{{lte value 1234.56}}", value), "true");
    });
  }

  test("negate renders tiny values without scientific notation (#284)", () => {
    assert.strictEqual(evaluate("{{negate value}}", "0.00000001"), "-0.00000001");
    assert.strictEqual(evaluate("{{negate value}}", 0.00000001), "-0.00000001");
    assert.strictEqual(evaluate("{{negate value}}", "-0.00000001"), "0.00000001");
    assert.strictEqual(evaluate("{{negate value}}", 0), "0");
  });

  test("decimal commas are not discarded (#421)", () => {
    USER_CONFIG.locale = "de-DE";
    for (const [value, expected] of [
      ["100,01", "-100.01"],
      ["-200,01", "200.01"],
      ["300,01", "-300.01"],
      ["-400,01", "400.01"]
    ]) {
      assert.strictEqual(evaluate("{{negate value}}", value), expected);
    }
  });

  test("scientific notation is accepted in input (#206)", () => {
    assert.strictEqual(evaluate("{{amount value}}", "3.66e-04"), "0.000366");
    assert.strictEqual(evaluate("{{negate value}}", "1E+3"), "-1000");
    assert.strictEqual(evaluate("{{amount value}}", "1e3 EUR"), "1000");
    USER_CONFIG.locale = "de-DE";
    assert.strictEqual(evaluate("{{amount value}}", "3,66e-04 €"), "0.000366");
  });

  test("numeric inputs do not undergo locale parsing", () => {
    USER_CONFIG.locale = "de-DE";
    assert.strictEqual(evaluate("{{amount value}}", 1234.56), "1234.56");
    assert.strictEqual(evaluate("{{negate value}}", 1234.56), "-1234.56");
  });

  test("nested helpers do not parse canonical output as localized input", () => {
    USER_CONFIG.locale = "de-DE";
    assert.strictEqual(evaluate("{{negate (amount value)}}", "1.234,560"), "-1234.56");
    assert.strictEqual(evaluate("{{amount (negate value)}}", "1.234,56"), "-1234.56");
    assert.strictEqual(evaluate("{{negate (round value precision=2)}}", "1.234,567"), "-1234.57");
    assert.strictEqual(evaluate("{{lt (amount value) 2000}}", "1.234,56"), "true");
  });

  test("decimal precision is preserved for arithmetic and comparisons", () => {
    assert.strictEqual(
      evaluate("{{negate value}}", "9007199254740993.123456789"),
      "-9007199254740993.123456789"
    );
    assert.strictEqual(evaluate('{{gt value "9007199254740992"}}', "9007199254740993"), "true");
    assert.strictEqual(
      evaluate('{{lt value "1.000000000000000002"}}', "1.000000000000000001"),
      "true"
    );
  });

  test("round uses decimal arithmetic and retains rounding towards positive infinity on ties", () => {
    assert.strictEqual(evaluate("{{round value precision=2}}", "1.005"), "1.01");
    assert.strictEqual(evaluate('{{round value precision="2"}}', "1.005"), "1.01");
    assert.strictEqual(evaluate("{{round value}}", "-1.5"), "-1");
    assert.strictEqual(evaluate("{{round value precision=-2}}", "1250.56"), "1251");
    assert.strictEqual(evaluate("{{round value precision=8}}", "0.00000001"), "0.00000001");
  });

  test("amount retains fractional trailing zeros used by existing templates", () => {
    assert.strictEqual(evaluate("{{amount value}}", "₹ 1,234.5600"), "1234.5600");
    USER_CONFIG.locale = "de-DE";
    assert.strictEqual(evaluate("{{amount value}}", "1.234,5600 €"), "1234.5600");
  });

  test("invalid input renders empty amounts and false numeric comparisons", () => {
    for (const value of ["", "N/A", "--12", "1.2.3", NaN, Infinity]) {
      assert.strictEqual(evaluate("{{amount value}}", value), "");
      assert.strictEqual(evaluate("{{negate value}}", value), "");
      assert.strictEqual(evaluate("{{round value}}", value), "");
      for (const helper of ["gt", "gte", "lt", "lte"]) {
        assert.strictEqual(evaluate(`{{${helper} value 0}}`, value), "false");
        assert.strictEqual(evaluate(`{{${helper} 0 value}}`, value), "false");
      }
    }
    assert.strictEqual(evaluate('{{amount value default="0"}}'), "0");
    assert.strictEqual(evaluate("{{amount value default=0}}"), "0");
    assert.strictEqual(evaluate('{{amount value default="fallback"}}'), "fallback");
    assert.strictEqual(evaluate('{{amount value default="fallback"}}', "0"), "0");
  });

  test("renders localized CSV input as complete journal entries", async () => {
    USER_CONFIG.locale = "de-DE";
    const result = await parse(
      new File(['"20.10.2026";"-400,01"\n"20.10.2026";"3,66e-04"'], "amounts.csv")
    );
    const template = Handlebars.compile(
      '{{date ROW.A "DD.MM.YYYY"}} Test\n    Assets:Test  {{negate ROW.B}} EUR\n    Expenses:Test'
    );
    assert.strictEqual(
      render(asRows(result), template, { trim: true }),
      [
        "2026/10/20 Test",
        "    Assets:Test" + " ".repeat(31) + "400.01 EUR",
        "    Expenses:Test",
        "",
        "2026/10/20 Test",
        "    Assets:Test" + " ".repeat(28) + "-0.000366 EUR",
        "    Expenses:Test"
      ].join("\n")
    );
  });
});
