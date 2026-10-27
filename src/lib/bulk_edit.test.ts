import { describe, expect, test } from "bun:test";
import fs from "fs";

import { applyChanges } from "./bulk_edit";
import type { LedgerFile } from "./utils";
import _ from "lodash";

describe("bulk_editor", () => {
  const before = fs.readFileSync("fixture/main.ledger");
  const transactions = JSON.parse(fs.readFileSync("fixture/main.transactions.json").toString());
  fs.readdirSync("fixture/bulk_edit").forEach((dir) => {
    test(dir, () => {
      const files = fs.readdirSync(`fixture/bulk_edit/${dir}`);
      for (const file of files) {
        const [name, extension] = file.split(".");
        if (extension === "ledger") {
          const args = JSON.parse(
            fs.readFileSync(`fixture/bulk_edit/${dir}/${name}.json`).toString()
          );
          const after = fs.readFileSync(`fixture/bulk_edit/${dir}/${name}.ledger`).toString();
          const ledgerFile: LedgerFile = {
            type: "file",
            name: "main.ledger",
            content: before.toString(),
            versions: []
          };
          const {
            newFiles: [newLedgerFile]
          } = applyChanges([ledgerFile], transactions, dir, args);
          expect(_.trim(newLedgerFile.content)).toBe(_.trim(after.toString()));
        }
      }
    });
  });

  test("rename_account preserves CRLF line endings", () => {
    const toCRLF = (text: string) => text.replace(/\r?\n/g, "\r\n");
    const args = JSON.parse(
      fs.readFileSync("fixture/bulk_edit/rename_account/simple.json").toString()
    );
    const ledgerFile: LedgerFile = {
      type: "file",
      name: "main.ledger",
      content: toCRLF(before.toString()),
      versions: []
    };
    const {
      newFiles: [newLedgerFile],
      updatedTransactionsCount
    } = applyChanges([ledgerFile], transactions, "rename_account", args);
    const after = fs.readFileSync("fixture/bulk_edit/rename_account/simple.ledger").toString();

    expect(updatedTransactionsCount).toBe(2);
    expect(_.trim(newLedgerFile.content)).toBe(_.trim(toCRLF(after)));
  });
});
