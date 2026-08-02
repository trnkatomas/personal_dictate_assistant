import { derived, writable } from 'svelte/store';
import { settings } from '$lib/stores.js';
import { GetSystemInfo } from '../../../wailsjs/go/main/App';
import en from './en.js';
import cs from './cs.js';

const DICTIONARIES = { en, cs };

// Set once at startup from the OS locale (see initLocale below) — used
// whenever settings.uiLanguage is 'auto' rather than an explicit override.
const systemLocale = writable('en');

// The active locale: an explicit settings.uiLanguage wins, otherwise fall
// back to the detected OS locale, and finally to English if neither maps to
// a dictionary we actually have (e.g. the OS is set to German).
export const locale = derived(
  [settings, systemLocale],
  ([$settings, $systemLocale]) => {
    const wanted = ($settings.uiLanguage && $settings.uiLanguage !== 'auto')
      ? $settings.uiLanguage
      : $systemLocale;
    return DICTIONARIES[wanted] ? wanted : 'en';
  }
);

// `t` is the active dictionary — components read strings as $t.section.key,
// or call $t.section.someFn(arg) for the handful of parameterized strings.
export const t = derived(locale, ($locale) => DICTIONARIES[$locale]);

// Detect the OS locale once at startup so 'auto' has something to follow.
// Best-effort: GetSystemInfo failing just leaves systemLocale at 'en'.
export async function initLocale() {
  try {
    const info = await GetSystemInfo();
    systemLocale.set((info.locale || 'en').slice(0, 2));
  } catch (_) {
    systemLocale.set('en');
  }
}
