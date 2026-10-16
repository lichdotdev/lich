import { readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"
import { describe, expect, it } from "vitest"
import { FILE_ICON_EXTENSIONS, FILE_ICON_NAMES } from "./FileIcon"

// The native window and its PNG generator read the same table from
// devicons.json; a file must get the same icon in both windows.
describe("FileIcon tables", () => {
  const manifest = JSON.parse(
    readFileSync(
      fileURLToPath(new URL("../../../native/ui/icons/devicon/devicons.json", import.meta.url)),
      "utf8",
    ),
  )

  it("match devicons.json by extension", () => {
    expect(FILE_ICON_EXTENSIONS).toEqual(manifest.extensions)
  })

  it("match devicons.json by file name", () => {
    expect(FILE_ICON_NAMES).toEqual(manifest.names)
  })
})
