import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const localesDir = path.resolve(__dirname, "../locales");
const locales = ["en", "id"];

function flattenKeys(obj, prefix = "") {
  const out = new Map();
  for (const [key, value] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      for (const [k, v] of flattenKeys(value, fullKey)) {
        out.set(k, v);
      }
    } else {
      out.set(fullKey, String(value));
    }
  }
  return out;
}

function extractPlaceholders(str) {
  const matches = str.match(/\{\{\s*[\w.]+\s*\}\}/g) ?? [];
  return [...new Set(matches.map((m) => m.replace(/\s+/g, "")))].sort();
}

let errors = 0;
const baseFiles = fs
  .readdirSync(path.join(localesDir, locales[0]))
  .filter((f) => f.endsWith(".json"))
  .sort();

for (const locale of locales.slice(1)) {
  const compareFiles = fs
    .readdirSync(path.join(localesDir, locale))
    .filter((f) => f.endsWith(".json"))
    .sort();

  if (JSON.stringify(baseFiles) !== JSON.stringify(compareFiles)) {
    console.error(`Namespace file mismatch between en (${baseFiles}) and ${locale} (${compareFiles})`);
    errors++;
    continue;
  }

  for (const file of baseFiles) {
    const baseJson = JSON.parse(fs.readFileSync(path.join(localesDir, locales[0], file), "utf8"));
    const cmpJson = JSON.parse(fs.readFileSync(path.join(localesDir, locale, file), "utf8"));

    const baseMap = flattenKeys(baseJson);
    const cmpMap = flattenKeys(cmpJson);

    for (const [key, baseVal] of baseMap) {
      if (!cmpMap.has(key)) {
        console.error(`[${locale}/${file}] Missing key: ${key}`);
        errors++;
        continue;
      }
      const baseVars = extractPlaceholders(baseVal);
      const cmpVars = extractPlaceholders(cmpMap.get(key));
      if (JSON.stringify(baseVars) !== JSON.stringify(cmpVars)) {
        console.error(`[${locale}/${file}] Placeholder mismatch on "${key}": expected ${baseVars}, got ${cmpVars}`);
        errors++;
      }
    }
    for (const key of cmpMap.keys()) {
      if (!baseMap.has(key)) {
        console.error(`[${locale}/${file}] Extra key not in en: ${key}`);
        errors++;
      }
    }
  }
}

if (errors > 0) {
  console.error(`i18n check failed with ${errors} error(s).`);
  process.exit(1);
}
console.log(`i18n parity check passed across ${baseFiles.length} namespaces (${locales.join(", ")}).`);
