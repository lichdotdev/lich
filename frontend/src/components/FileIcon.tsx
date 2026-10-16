import type { ComponentType } from "react"
import { Database, File } from "lucide-react"
// Deep per-icon imports, NOT the devicons-react barrel: the barrel is one CJS
// module re-exporting every icon, so importing from it bundles all ~4000 of
// them (~10MB). The package's exports map allows ./icons/*, so each icon is its
// own ~1.4KB module.
import AstroOriginal from "devicons-react/icons/AstroOriginal"
import BashOriginal from "devicons-react/icons/BashOriginal"
import COriginal from "devicons-react/icons/COriginal"
import ClojureOriginal from "devicons-react/icons/ClojureOriginal"
import CoffeescriptOriginal from "devicons-react/icons/CoffeescriptOriginal"
import CplusplusOriginal from "devicons-react/icons/CplusplusOriginal"
import CrystalOriginal from "devicons-react/icons/CrystalOriginal"
import CsharpOriginal from "devicons-react/icons/CsharpOriginal"
import Css3Original from "devicons-react/icons/Css3Original"
import DartOriginal from "devicons-react/icons/DartOriginal"
import DockerOriginal from "devicons-react/icons/DockerOriginal"
import ElixirOriginal from "devicons-react/icons/ElixirOriginal"
import ElmOriginal from "devicons-react/icons/ElmOriginal"
import ErlangOriginal from "devicons-react/icons/ErlangOriginal"
import FsharpOriginal from "devicons-react/icons/FsharpOriginal"
import GitOriginal from "devicons-react/icons/GitOriginal"
import GleamOriginal from "devicons-react/icons/GleamOriginal"
import GoOriginal from "devicons-react/icons/GoOriginal"
import GradleOriginal from "devicons-react/icons/GradleOriginal"
import GraphqlPlain from "devicons-react/icons/GraphqlPlain"
import GroovyOriginal from "devicons-react/icons/GroovyOriginal"
import HandlebarsOriginal from "devicons-react/icons/HandlebarsOriginal"
import HaskellOriginal from "devicons-react/icons/HaskellOriginal"
import Html5Original from "devicons-react/icons/Html5Original"
import JavaOriginal from "devicons-react/icons/JavaOriginal"
import JavascriptOriginal from "devicons-react/icons/JavascriptOriginal"
import JsonOriginal from "devicons-react/icons/JsonOriginal"
import JuliaOriginal from "devicons-react/icons/JuliaOriginal"
import KotlinOriginal from "devicons-react/icons/KotlinOriginal"
import LatexOriginal from "devicons-react/icons/LatexOriginal"
import LuaOriginal from "devicons-react/icons/LuaOriginal"
import MarkdownOriginal from "devicons-react/icons/MarkdownOriginal"
import NimOriginal from "devicons-react/icons/NimOriginal"
import NixosOriginal from "devicons-react/icons/NixosOriginal"
import OcamlOriginal from "devicons-react/icons/OcamlOriginal"
import PerlOriginal from "devicons-react/icons/PerlOriginal"
import PhpOriginal from "devicons-react/icons/PhpOriginal"
import PowershellOriginal from "devicons-react/icons/PowershellOriginal"
import PrismaOriginal from "devicons-react/icons/PrismaOriginal"
import PugOriginal from "devicons-react/icons/PugOriginal"
import PythonOriginal from "devicons-react/icons/PythonOriginal"
import ROriginal from "devicons-react/icons/ROriginal"
import RubyOriginal from "devicons-react/icons/RubyOriginal"
import RustOriginal from "devicons-react/icons/RustOriginal"
import SassOriginal from "devicons-react/icons/SassOriginal"
import ScalaOriginal from "devicons-react/icons/ScalaOriginal"
import SolidityOriginal from "devicons-react/icons/SolidityOriginal"
import SvelteOriginal from "devicons-react/icons/SvelteOriginal"
import SwiftOriginal from "devicons-react/icons/SwiftOriginal"
import TerraformOriginal from "devicons-react/icons/TerraformOriginal"
import TypescriptOriginal from "devicons-react/icons/TypescriptOriginal"
import VimOriginal from "devicons-react/icons/VimOriginal"
import VuejsOriginal from "devicons-react/icons/VuejsOriginal"
import XmlOriginal from "devicons-react/icons/XmlOriginal"
import YamlOriginal from "devicons-react/icons/YamlOriginal"
import ZigOriginal from "devicons-react/icons/ZigOriginal"
import { extname } from "@/lib/git/lang-badge"

// devicons-react components spread extra props onto their <svg> and take a
// `size` that sets both dimensions. Only the props this module passes are
// typed — a narrower shape than the icons' own SVGProps, which they satisfy.
type IconComponent = ComponentType<{
  size?: number | string
  fill?: string
  className?: string
}>

// ICONS holds every icon a file can get. `mono` flags the marks that ship with
// no fill of their own (they render black and vanish on a dark ground), so
// those ride the row's text color via fill=currentColor; every other icon keeps
// its brand colors. Database is Lucide's, stroked in currentColor already.
const ICONS = {
  AstroOriginal: { Icon: AstroOriginal },
  BashOriginal: { Icon: BashOriginal },
  COriginal: { Icon: COriginal },
  ClojureOriginal: { Icon: ClojureOriginal },
  CoffeescriptOriginal: { Icon: CoffeescriptOriginal },
  CplusplusOriginal: { Icon: CplusplusOriginal },
  CrystalOriginal: { Icon: CrystalOriginal, mono: true },
  CsharpOriginal: { Icon: CsharpOriginal },
  Css3Original: { Icon: Css3Original },
  DartOriginal: { Icon: DartOriginal },
  Database: { Icon: Database },
  DockerOriginal: { Icon: DockerOriginal },
  ElixirOriginal: { Icon: ElixirOriginal },
  ElmOriginal: { Icon: ElmOriginal },
  ErlangOriginal: { Icon: ErlangOriginal },
  FsharpOriginal: { Icon: FsharpOriginal },
  GitOriginal: { Icon: GitOriginal },
  GleamOriginal: { Icon: GleamOriginal },
  GoOriginal: { Icon: GoOriginal },
  GradleOriginal: { Icon: GradleOriginal },
  GraphqlPlain: { Icon: GraphqlPlain },
  GroovyOriginal: { Icon: GroovyOriginal },
  HandlebarsOriginal: { Icon: HandlebarsOriginal, mono: true },
  HaskellOriginal: { Icon: HaskellOriginal },
  Html5Original: { Icon: Html5Original },
  JavaOriginal: { Icon: JavaOriginal },
  JavascriptOriginal: { Icon: JavascriptOriginal },
  JsonOriginal: { Icon: JsonOriginal },
  JuliaOriginal: { Icon: JuliaOriginal },
  KotlinOriginal: { Icon: KotlinOriginal },
  LatexOriginal: { Icon: LatexOriginal, mono: true },
  LuaOriginal: { Icon: LuaOriginal },
  MarkdownOriginal: { Icon: MarkdownOriginal, mono: true },
  NimOriginal: { Icon: NimOriginal },
  NixosOriginal: { Icon: NixosOriginal },
  OcamlOriginal: { Icon: OcamlOriginal },
  PerlOriginal: { Icon: PerlOriginal },
  PhpOriginal: { Icon: PhpOriginal },
  PowershellOriginal: { Icon: PowershellOriginal },
  PrismaOriginal: { Icon: PrismaOriginal },
  PugOriginal: { Icon: PugOriginal },
  PythonOriginal: { Icon: PythonOriginal },
  ROriginal: { Icon: ROriginal },
  RubyOriginal: { Icon: RubyOriginal },
  RustOriginal: { Icon: RustOriginal, mono: true },
  SassOriginal: { Icon: SassOriginal },
  ScalaOriginal: { Icon: ScalaOriginal },
  SolidityOriginal: { Icon: SolidityOriginal, mono: true },
  SvelteOriginal: { Icon: SvelteOriginal },
  SwiftOriginal: { Icon: SwiftOriginal },
  TerraformOriginal: { Icon: TerraformOriginal },
  TypescriptOriginal: { Icon: TypescriptOriginal },
  VimOriginal: { Icon: VimOriginal },
  VuejsOriginal: { Icon: VuejsOriginal },
  XmlOriginal: { Icon: XmlOriginal },
  YamlOriginal: { Icon: YamlOriginal },
  ZigOriginal: { Icon: ZigOriginal },
} satisfies Record<string, { Icon: IconComponent; mono?: boolean }>

type IconName = keyof typeof ICONS

// FILE_ICON_NAMES and FILE_ICON_EXTENSIONS pick a file's icon, by its
// lowercased base name first, then by its lowercased extension. They are
// native/ui/icons/devicon/devicons.json, which the native window and gen.sh
// read; FileIcon.test.ts fails when the two drift.
export const FILE_ICON_NAMES: Record<string, IconName> = {
  dockerfile: "DockerOriginal",
  ".gitignore": "GitOriginal",
  ".gitattributes": "GitOriginal",
  ".gitmodules": "GitOriginal",
}

export const FILE_ICON_EXTENSIONS: Record<string, IconName> = {
  astro: "AstroOriginal",
  bash: "BashOriginal",
  c: "COriginal",
  cc: "CplusplusOriginal",
  cjs: "JavascriptOriginal",
  clj: "ClojureOriginal",
  cljc: "ClojureOriginal",
  cljs: "ClojureOriginal",
  coffee: "CoffeescriptOriginal",
  cpp: "CplusplusOriginal",
  cr: "CrystalOriginal",
  cs: "CsharpOriginal",
  css: "Css3Original",
  cts: "TypescriptOriginal",
  cxx: "CplusplusOriginal",
  dart: "DartOriginal",
  edn: "ClojureOriginal",
  elm: "ElmOriginal",
  erl: "ErlangOriginal",
  ex: "ElixirOriginal",
  exs: "ElixirOriginal",
  fs: "FsharpOriginal",
  fsi: "FsharpOriginal",
  fsx: "FsharpOriginal",
  gleam: "GleamOriginal",
  go: "GoOriginal",
  gql: "GraphqlPlain",
  gradle: "GradleOriginal",
  graphql: "GraphqlPlain",
  groovy: "GroovyOriginal",
  h: "COriginal",
  handlebars: "HandlebarsOriginal",
  hbs: "HandlebarsOriginal",
  hh: "CplusplusOriginal",
  hpp: "CplusplusOriginal",
  hrl: "ErlangOriginal",
  hs: "HaskellOriginal",
  htm: "Html5Original",
  html: "Html5Original",
  java: "JavaOriginal",
  jl: "JuliaOriginal",
  js: "JavascriptOriginal",
  json: "JsonOriginal",
  jsonc: "JsonOriginal",
  jsx: "JavascriptOriginal",
  kt: "KotlinOriginal",
  kts: "KotlinOriginal",
  lua: "LuaOriginal",
  md: "MarkdownOriginal",
  mdx: "MarkdownOriginal",
  mjs: "JavascriptOriginal",
  ml: "OcamlOriginal",
  mli: "OcamlOriginal",
  mts: "TypescriptOriginal",
  nim: "NimOriginal",
  nix: "NixosOriginal",
  php: "PhpOriginal",
  pl: "PerlOriginal",
  pm: "PerlOriginal",
  prisma: "PrismaOriginal",
  ps1: "PowershellOriginal",
  psm1: "PowershellOriginal",
  pug: "PugOriginal",
  py: "PythonOriginal",
  r: "ROriginal",
  rb: "RubyOriginal",
  rs: "RustOriginal",
  sass: "SassOriginal",
  sc: "ScalaOriginal",
  scala: "ScalaOriginal",
  scss: "SassOriginal",
  sh: "BashOriginal",
  sol: "SolidityOriginal",
  sql: "Database",
  svelte: "SvelteOriginal",
  swift: "SwiftOriginal",
  tex: "LatexOriginal",
  tf: "TerraformOriginal",
  tfvars: "TerraformOriginal",
  ts: "TypescriptOriginal",
  tsx: "TypescriptOriginal",
  vim: "VimOriginal",
  vue: "VuejsOriginal",
  xml: "XmlOriginal",
  yaml: "YamlOriginal",
  yml: "YamlOriginal",
  zig: "ZigOriginal",
  zsh: "BashOriginal",
}

// ICON_PX matches the tree's size-3.5 (14px) folder icons so rows line up.
const ICON_PX = 14

function iconFor(path: string): IconName | undefined {
  const base = path.slice(path.lastIndexOf("/") + 1).toLowerCase()
  return FILE_ICON_NAMES[base] ?? FILE_ICON_EXTENSIONS[extname(path)]
}

interface FileIconProps {
  path: string
}

export function FileIcon({ path }: FileIconProps) {
  const name = iconFor(path)
  if (!name) {
    return <File className="size-3.5 shrink-0 text-muted-foreground opacity-70" />
  }
  const { Icon, mono }: { Icon: IconComponent; mono?: boolean } = ICONS[name]
  return <Icon size={ICON_PX} fill={mono ? "currentColor" : undefined} className="shrink-0" />
}
