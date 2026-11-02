import { describe, expect, test } from "bun:test";
import { syntaxTree } from "@codemirror/language";
import { EditorState } from "@codemirror/state";
import dayjs from "dayjs";
import isSameOrAfter from "dayjs/plugin/isSameOrAfter";
import isSameOrBefore from "dayjs/plugin/isSameOrBefore";
import { buildAST, parseDate, type TransactionPredicate } from "./search_query_editor";
import { queryExtension } from "./search/parser/query";
import { Environment, buildAST as buildSheetAST, Query } from "./sheet/interpreter";
import { sheetLanguage } from "./sheet/language";
import type { Transaction } from "./utils";

function assertRange(text: string, start: string, end: string) {
  const result = parseDate(text, reference);
  expect(result.start.format("YYYY-MM-DDTHH:mm:ss")).toEqual(start);
  expect(result.end.format("YYYY-MM-DDTHH:mm:ss")).toEqual(end);
}

dayjs.extend(isSameOrAfter);
dayjs.extend(isSameOrBefore);

const reference = new Date("2023-09-03");
describe("parseDate", () => {
  test("natural language", () => {
    assertRange("today", "2023-09-03T00:00:00", "2023-09-03T23:59:59");
    assertRange("this month", "2023-09-01T00:00:00", "2023-09-30T23:59:59");
    assertRange("last month", "2023-08-01T00:00:00", "2023-08-31T23:59:59");
    assertRange("this year", "2023-01-01T00:00:00", "2023-12-31T23:59:59");
    assertRange("2023", "2023-01-01T00:00:00", "2023-12-31T23:59:59");
    assertRange("2023-01", "2023-01-01T00:00:00", "2023-01-31T23:59:59");
    assertRange("last week", "2023-08-27T00:00:00", "2023-09-02T23:59:59");
    // TODO
    // assertRange("jan 2023", "2023-01-01T00:00:00", "2023-01-31T23:59:59");
  });
});

describe("BooleanConditionAST", () => {
  test("evaluates a binary date condition without throwing", () => {
    // The sheet formula `date_query={...}` passes the content between the
    // braces to the search query parser.
    const state = EditorState.create({
      doc: "date >= [2025-04-01] AND date <= [2028-03-31]",
      extensions: [queryExtension()]
    });

    const ast = buildAST(state, syntaxTree(state).topNode);
    const predicate: TransactionPredicate = ast.evaluate();

    expect(typeof predicate).toBe("function");
    expect(predicate({ date: dayjs("2026-06-15") } as Transaction)).toBe(true);
  });

  test("evaluates the date_query sheet formula without throwing", () => {
    const text = "date_query={date >= [2025-04-01] AND date <= [2028-03-31]}";
    const parser = sheetLanguage.parser.configure({ dialect: "skip_comment" });
    const state = EditorState.create({ doc: text });
    const ast = buildSheetAST(parser.parse(text).topNode, state);
    const env = new Environment();
    env.postings = [];

    ast.evaluate(env);

    const query = env.scope.date_query as Query;
    expect(query).toBeInstanceOf(Query);
    expect(typeof query.predicate).toBe("function");
  });
});
