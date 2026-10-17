import {
  type Locale,
  matchLocale,
  setLocale,
  UI_LANGUAGE_PREF,
  UI_LANGUAGE_SETTING,
} from "@/lib/i18n/i18n"
import { readPref } from "@/lib/prefs"
import { Store } from "@/lib/rpc"

// The page keeps its interface language in localStorage (i18n.ts); this is the
// copy in the backend settings, the one the native window reads. It lives apart
// from i18n.ts so that module stays free of RPC for the node suites.

// The global (project-less) scope, as prompt-language-store.ts writes it.
const GLOBAL_SCOPE = ""

/** Switches the page's language at once, then mirrors it to the backend. */
export function saveLocale(locale: Locale): Promise<void> {
  setLocale(locale)
  return Store.SetSetting(UI_LANGUAGE_SETTING, GLOBAL_SCOPE, locale).then(() => {})
}

/** Copies a language someone picked on this page before the mirror existed
 * into an empty backend setting. A language only guessed from the browser is
 * never copied, and a value already there is never overwritten. */
export async function seedUiLanguageSetting(): Promise<void> {
  const stored = readPref(UI_LANGUAGE_PREF)
  if (stored === null) return
  if (await Store.GetSetting(UI_LANGUAGE_SETTING, GLOBAL_SCOPE)) return
  await Store.SetSetting(UI_LANGUAGE_SETTING, GLOBAL_SCOPE, matchLocale(stored))
}
