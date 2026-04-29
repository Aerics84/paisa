import "../happydom";
import { describe, expect, test } from "bun:test";

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
      const files = fs.readdirSync(`fixture/import/${dir}`);
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
          const result = await parse(new File([input], inputFile));
          const rows = asRows(result);

          const actual = render(rows, compiled, { trim: true });

          expect(actual).toBe(_.trim(output));
        }
      }
    });
  });
});

describe("broker csv normalization", () => {
  test("trade republic trades expose normalized semantic fields", async () => {
    const input = fs.readFileSync("fixture/import/Trade Republic Trades CSV/trades.csv");
    const result = await parse(new File([input], "trades.csv"));
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
    const result = await parse(new File([input], "cash.csv"));
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
    const result = await parse(new File([input], "trades.csv"));
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
    expect(helpers.amount("92,33", { hash: {} })).toBe("92.33");
    expect(helpers.amount("2,707819", { hash: {} })).toBe("2.707819");
    expect(helpers.amount("1.234,56", { hash: {} })).toBe("1234.56");
    expect(helpers.amount("10.005,05 EUR", { hash: {} })).toBe("10005.05");
    expect(helpers.amount("100,000", { hash: {} })).toBe("100000");
    expect(helpers.amount("1,00,000", { hash: {} })).toBe("100000");
  });

  test("date helper normalizes day-first dates", () => {
    expect(helpers.date("28.04.2026", "DD.MM.YYYY")).toBe("2026/04/28");
  });
});
