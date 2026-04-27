/**
 * Generates easystock-api/data/dataset.json from shared TypeScript dataset.
 * Run from repo: npm run export-dataset (cwd: easyStock).
 */
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  mockPicks,
  mockSectorNews,
  mockSectors,
  mockSectorStocks,
  mockStocks,
} from "../shared/dataset.ts";

const __dirname = dirname(fileURLToPath(import.meta.url));
const outPath = join(__dirname, "../../easystock-api/staticdata/dataset.json");

mkdirSync(dirname(outPath), { recursive: true });
writeFileSync(
  outPath,
  JSON.stringify(
    {
      picks: mockPicks,
      stocks: mockStocks,
      sectors: mockSectors,
      sectorStocks: mockSectorStocks,
      sectorNews: mockSectorNews,
    },
    null,
    2,
  ),
  "utf8",
);
console.log("Wrote", outPath);
