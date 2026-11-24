import { spawn } from "node:child_process";
import { copyFileSync, cpSync, existsSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { DatabaseSync } from "node:sqlite";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const source = join(root, "tests", "fixture", "germany-eu");
const seedDatabase = join(root, "internal", "model", "migrations", "testdata", "v0.8.6.db");
const seeded = process.env.PAISA_E2E_SEED === "true";
const target = join(root, ".cache", seeded ? "e2e-seeded" : "e2e");
const port = process.env.PAISA_E2E_PORT || "5758";

const index = join(root, "web", "static", "index.html");
if (!existsSync(index)) {
  console.error("web/static/index.html is missing. Run `npm run build` before `npm run e2e`.");
  process.exit(1);
}

// Never touch the committed fixture. Each run gets a fresh copy, including the
// SQLite main database, WAL and SHM files so the database stays consistent.
rmSync(target, { recursive: true, force: true });
mkdirSync(target, { recursive: true });
cpSync(source, target, { recursive: true, preserveTimestamps: true });

if (seeded) {
  // The UI regression routes need recent postings, a recurring sequence and a
  // nested expense account. The germany-eu fixture journal is 2022-only and has
  // depth 1, so use the 54-posting migration fixture and tag it for the
  // rendering checks. The committed fixtures stay untouched: only this cache
  // copy is modified.
  copyFileSync(seedDatabase, join(target, "paisa.db"));
  for (const suffix of ["-wal", "-shm", ".synced"]) {
    rmSync(join(target, `paisa.db${suffix}`), { force: true });
  }

  const database = new DatabaseSync(join(target, "paisa.db"));
  database.exec(`
    UPDATE postings
       SET tag_recurring = 'Rent', tag_period = '4 * ?'
     WHERE account = 'Expenses:Rent';
    UPDATE postings
       SET account = 'Expenses:Food:Groceries'
     WHERE id = (SELECT min(id) FROM postings WHERE account = 'Expenses:Food');
  `);
  database.close();

  // Exercise the FileTree folder branch (summary + nested list + chevron)
  // without adding a directory to the committed germany-eu fixture. The file
  // is not included by main.ledger, so it does not change the synced data.
  mkdirSync(join(target, "archive"), { recursive: true });
  writeFileSync(
    join(target, "archive", "old.ledger"),
    "2022/01/01 Opening\n    Assets:Checking:ING    0,00 EUR\n    Equity:Opening-Balances\n"
  );
}

const configPath = join(target, "paisa.yaml");

const child = spawn("go", ["run", ".", "serve", "--port", port], {
  cwd: root,
  stdio: "inherit",
  windowsHide: true,
  env: { ...process.env, PAISA_CONFIG: configPath }
});

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => child.kill(signal));
}

child.on("error", (error) => {
  console.error("failed to start paisa server:", error);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.exit(0);
  }
  process.exit(code ?? 0);
});
