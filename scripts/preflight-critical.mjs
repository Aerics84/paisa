import path from "path";
import { fileURLToPath } from "url";
import { run } from "./lib/changed-files.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const rootDir = path.resolve(__dirname, "..");

function runOrExit(command, args, env = process.env) {
  const result = run(rootDir, command, args, { env });
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

runOrExit("npm", ["run", "check:format:changed"]);
runOrExit("node", ["./scripts/gofmt-changed.mjs"]);
runOrExit("bun", ["test", "--preload", "./src/happydom.ts", "src/lib/import.test.ts"]);
runOrExit("go", ["build", ...(process.env.GO_VERSION_LDFLAGS || "").split(/\s+/).filter(Boolean)]);

const regressionEnv = { ...process.env };
delete regressionEnv.PAISA_CONFIG;
regressionEnv.TZ = "UTC";
runOrExit("bun", ["test", "tests/regression.test.ts"], regressionEnv);
