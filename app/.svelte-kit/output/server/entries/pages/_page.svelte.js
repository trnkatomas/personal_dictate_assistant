import { r as onDestroy, t as createEventDispatcher } from "../../chunks/index-server.js";
import { E as writable, R as attr, c as unsubscribe_stores, i as ensure_array_like, n as attr_class, s as store_get, z as escape_html } from "../../chunks/dev.js";
import "../../chunks/index-server2.js";
import { diffWords } from "diff";
//#region src/lib/stores.js
/** Writable store that syncs its value to localStorage. */
function persisted(key, initial) {
	let stored = initial;
	if (typeof window !== "undefined") try {
		const item = localStorage.getItem(key);
		if (item !== null) stored = JSON.parse(item);
	} catch {}
	const store = writable(stored);
	if (typeof window !== "undefined") store.subscribe((v) => {
		try {
			localStorage.setItem(key, JSON.stringify(v));
		} catch {}
	});
	return store;
}
var settings = persisted("da:settings", {
	whisperUrl: "/api/whisper",
	ollamaUrl: "/api/ollama",
	ollamaModel: "gemma3:1b",
	whisperLanguage: "",
	whisperTask: "transcribe"
});
var prompt = persisted("da:prompt", "Add punctuation, fix grammar errors, and format into proper sentences. Return only the corrected text — no explanations, no preamble.");
var isRecording = writable(false);
var isTranscribing = writable(false);
var isTransforming = writable(false);
writable(null);
var rawText = writable("");
var transformedText = writable("");
var showSettings = writable(false);
var showDiff = writable(false);
//#endregion
//#region src/lib/components/Recorder.svelte
function Recorder($$renderer, $$props) {
	$$renderer.component(($$renderer) => {
		var $$store_subs;
		createEventDispatcher();
		onDestroy(() => {});
		$$renderer.push(`<div class="recorder svelte-1wkgwsm"><canvas class="waveform svelte-1wkgwsm"></canvas> <button${attr_class("record-btn svelte-1wkgwsm", void 0, { "recording": store_get($$store_subs ??= {}, "$isRecording", isRecording) })}${attr("title", store_get($$store_subs ??= {}, "$isRecording", isRecording) ? "Stop recording (Space)" : "Start recording (Space)")}>`);
		if (store_get($$store_subs ??= {}, "$isRecording", isRecording)) {
			$$renderer.push("<!--[0-->");
			$$renderer.push(`<svg viewBox="0 0 24 24" fill="currentColor" width="18" height="18" class="svelte-1wkgwsm"><rect x="5" y="5" width="14" height="14" rx="2" class="svelte-1wkgwsm"></rect></svg> Stop`);
		} else {
			$$renderer.push("<!--[-1-->");
			$$renderer.push(`<svg viewBox="0 0 24 24" fill="currentColor" width="18" height="18" class="svelte-1wkgwsm"><circle cx="12" cy="12" r="7" class="svelte-1wkgwsm"></circle></svg> Record`);
		}
		$$renderer.push(`<!--]--></button></div>`);
		if ($$store_subs) unsubscribe_stores($$store_subs);
	});
}
//#endregion
//#region src/lib/components/TranscriptPane.svelte
function TranscriptPane($$renderer) {
	var $$store_subs;
	$$renderer.push(`<div class="pane svelte-197n1f8"><div class="pane-header svelte-197n1f8"><h2 class="svelte-197n1f8">Transcription</h2> <div class="pane-actions svelte-197n1f8">`);
	if (store_get($$store_subs ??= {}, "$isTranscribing", isTranscribing)) {
		$$renderer.push("<!--[0-->");
		$$renderer.push(`<span class="badge loading svelte-197n1f8">Transcribing…</span>`);
	} else if (store_get($$store_subs ??= {}, "$rawText", rawText)) {
		$$renderer.push("<!--[1-->");
		$$renderer.push(`<button class="action-btn svelte-197n1f8">${escape_html("Copy")}</button>`);
	} else $$renderer.push("<!--[-1-->");
	$$renderer.push(`<!--]--></div></div> <textarea class="pane-body svelte-197n1f8" placeholder="Raw transcription will appear here after recording…" spellcheck="false" aria-label="Raw transcription">`);
	const $$body = escape_html(store_get($$store_subs ??= {}, "$rawText", rawText));
	if ($$body) $$renderer.push(`${$$body}`);
	$$renderer.push(`</textarea></div>`);
	if ($$store_subs) unsubscribe_stores($$store_subs);
}
//#endregion
//#region src/lib/components/TransformPane.svelte
function TransformPane($$renderer, $$props) {
	$$renderer.component(($$renderer) => {
		var $$store_subs;
		$$renderer.push(`<div class="pane svelte-qzmn2"><div class="pane-header svelte-qzmn2"><h2 class="svelte-qzmn2">Transformed</h2> <div class="pane-actions svelte-qzmn2">`);
		if (store_get($$store_subs ??= {}, "$isTransforming", isTransforming)) {
			$$renderer.push("<!--[0-->");
			$$renderer.push(`<span class="badge loading svelte-qzmn2">Transforming…</span>`);
		} else if (store_get($$store_subs ??= {}, "$transformedText", transformedText)) {
			$$renderer.push("<!--[1-->");
			$$renderer.push(`<button class="action-btn svelte-qzmn2">${escape_html("Copy")}</button>`);
		} else $$renderer.push("<!--[-1-->");
		$$renderer.push(`<!--]--></div></div> <div class="prompt-strip svelte-qzmn2"><textarea class="prompt-input svelte-qzmn2" rows="2" placeholder="LLM instruction prompt…" aria-label="Transform prompt">`);
		const $$body = escape_html(store_get($$store_subs ??= {}, "$prompt", prompt));
		if ($$body) $$renderer.push(`${$$body}`);
		$$renderer.push(`</textarea> <button class="transform-btn svelte-qzmn2"${attr("disabled", store_get($$store_subs ??= {}, "$isTransforming", isTransforming) || !store_get($$store_subs ??= {}, "$rawText", rawText).trim(), true)}>`);
		if (store_get($$store_subs ??= {}, "$isTransforming", isTransforming)) {
			$$renderer.push("<!--[0-->");
			$$renderer.push(`<svg class="spin svelte-qzmn2" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" class="svelte-qzmn2"></path></svg>`);
		} else {
			$$renderer.push("<!--[-1-->");
			$$renderer.push(`↗`);
		}
		$$renderer.push(`<!--]--> Transform</button></div> `);
		$$renderer.push("<!--[-1-->");
		$$renderer.push(`<!--]--> <textarea class="pane-body svelte-qzmn2" placeholder="Transformed text will stream in here…" spellcheck="true" aria-label="Transformed text">`);
		const $$body_1 = escape_html(store_get($$store_subs ??= {}, "$transformedText", transformedText));
		if ($$body_1) $$renderer.push(`${$$body_1}`);
		$$renderer.push(`</textarea></div>`);
		if ($$store_subs) unsubscribe_stores($$store_subs);
	});
}
//#endregion
//#region src/lib/components/DiffView.svelte
function DiffView($$renderer, $$props) {
	$$renderer.component(($$renderer) => {
		var $$store_subs;
		let parts, removedCount, addedCount;
		$: parts = store_get($$store_subs ??= {}, "$rawText", rawText) && store_get($$store_subs ??= {}, "$transformedText", transformedText) ? diffWords(store_get($$store_subs ??= {}, "$rawText", rawText), store_get($$store_subs ??= {}, "$transformedText", transformedText)) : [];
		$: removedCount = parts.filter((p) => p.removed).length;
		$: addedCount = parts.filter((p) => p.added).length;
		$$renderer.push(`<div class="diff-pane svelte-1fghi9m"><div class="diff-header svelte-1fghi9m"><div class="diff-title svelte-1fghi9m"><span class="side old svelte-1fghi9m">Transcription</span> <span class="arrow svelte-1fghi9m">→</span> <span class="side new svelte-1fghi9m">Transformed</span></div> `);
		if (parts.length > 0) {
			$$renderer.push("<!--[0-->");
			$$renderer.push(`<div class="stats svelte-1fghi9m"><span class="stat del svelte-1fghi9m">−${escape_html(removedCount)} words</span> <span class="stat ins svelte-1fghi9m">+${escape_html(addedCount)} words</span></div>`);
		} else $$renderer.push("<!--[-1-->");
		$$renderer.push(`<!--]--></div> `);
		if (parts.length > 0) {
			$$renderer.push("<!--[0-->");
			$$renderer.push(`<div class="diff-body svelte-1fghi9m"><p class="diff-text svelte-1fghi9m"><!--[-->`);
			const each_array = ensure_array_like(parts);
			for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
				let part = each_array[$$index];
				if (part.removed) {
					$$renderer.push("<!--[0-->");
					$$renderer.push(`<del class="svelte-1fghi9m">${escape_html(part.value)}</del>`);
				} else if (part.added) {
					$$renderer.push("<!--[1-->");
					$$renderer.push(`<ins class="svelte-1fghi9m">${escape_html(part.value)}</ins>`);
				} else {
					$$renderer.push("<!--[-1-->");
					$$renderer.push(`${escape_html(part.value)}`);
				}
				$$renderer.push(`<!--]-->`);
			}
			$$renderer.push(`<!--]--></p></div>`);
		} else {
			$$renderer.push("<!--[-1-->");
			$$renderer.push(`<div class="empty svelte-1fghi9m"><p class="svelte-1fghi9m">Nothing to diff yet.</p> <p class="hint svelte-1fghi9m">Record → transcribe → transform, then come back here.</p></div>`);
		}
		$$renderer.push(`<!--]--></div>`);
		if ($$store_subs) unsubscribe_stores($$store_subs);
	});
}
//#endregion
//#region src/lib/components/Settings.svelte
function Settings($$renderer, $$props) {
	$$renderer.component(($$renderer) => {
		var $$store_subs;
		if (store_get($$store_subs ??= {}, "$showSettings", showSettings)) {
			$$renderer.push("<!--[0-->");
			$$renderer.push(`<div class="overlay svelte-anx9w7"><div class="panel svelte-anx9w7" role="dialog" aria-label="Settings"><div class="panel-header svelte-anx9w7"><h3 class="svelte-anx9w7">Settings</h3> <button class="close-btn svelte-anx9w7">✕</button></div> <div class="fields svelte-anx9w7"><label class="svelte-anx9w7"><span class="svelte-anx9w7">Whisper URL</span> <input type="url"${attr("value", store_get($$store_subs ??= {}, "$settings", settings).whisperUrl)} class="svelte-anx9w7"/></label> <label class="svelte-anx9w7"><span class="svelte-anx9w7">Ollama URL</span> <input type="url"${attr("value", store_get($$store_subs ??= {}, "$settings", settings).ollamaUrl)} class="svelte-anx9w7"/></label> <label class="svelte-anx9w7"><span class="svelte-anx9w7">Ollama model</span> <input type="text"${attr("value", store_get($$store_subs ??= {}, "$settings", settings).ollamaModel)} placeholder="llama3.2" class="svelte-anx9w7"/></label> <label class="svelte-anx9w7"><span class="svelte-anx9w7">Whisper language <em class="svelte-anx9w7">(leave blank to auto-detect)</em></span> <input type="text"${attr("value", store_get($$store_subs ??= {}, "$settings", settings).whisperLanguage)} placeholder="e.g. en, de, cs" class="svelte-anx9w7"/></label> <label class="svelte-anx9w7"><span class="svelte-anx9w7">Whisper task</span> `);
			$$renderer.select({
				value: store_get($$store_subs ??= {}, "$settings", settings).whisperTask,
				class: ""
			}, ($$renderer) => {
				$$renderer.option({
					value: "transcribe",
					class: ""
				}, ($$renderer) => {
					$$renderer.push(`Transcribe (keep original language)`);
				}, "svelte-anx9w7");
				$$renderer.option({
					value: "translate",
					class: ""
				}, ($$renderer) => {
					$$renderer.push(`Translate to English`);
				}, "svelte-anx9w7");
			}, "svelte-anx9w7");
			$$renderer.push(`</label></div> <div class="panel-footer svelte-anx9w7"><button class="reset-btn svelte-anx9w7">Reset to defaults</button> <button class="save-btn svelte-anx9w7">Done</button></div></div></div>`);
		} else $$renderer.push("<!--[-1-->");
		$$renderer.push(`<!--]-->`);
		if ($$store_subs) unsubscribe_stores($$store_subs);
	});
}
//#endregion
//#region src/routes/+page.svelte
function _page($$renderer, $$props) {
	$$renderer.component(($$renderer) => {
		var $$store_subs;
		Settings($$renderer, {});
		$$renderer.push(`<!----> <div class="layout svelte-1uha8ag"><header class="topbar svelte-1uha8ag"><span class="app-name svelte-1uha8ag">🎙 Dictate</span> <div class="topbar-actions svelte-1uha8ag"><button${attr_class("tool-btn svelte-1uha8ag", void 0, { "active": store_get($$store_subs ??= {}, "$showDiff", showDiff) })} title="Toggle diff view">⟷ Diff</button> <button class="tool-btn svelte-1uha8ag" title="Open settings">⚙ Settings</button></div></header> <section class="record-section svelte-1uha8ag">`);
		Recorder($$renderer, {});
		$$renderer.push(`<!----> `);
		$$renderer.push("<!--[-1-->");
		$$renderer.push(`<!--]--></section> <section class="panes svelte-1uha8ag">`);
		if (store_get($$store_subs ??= {}, "$showDiff", showDiff)) {
			$$renderer.push("<!--[0-->");
			DiffView($$renderer, {});
		} else {
			$$renderer.push("<!--[-1-->");
			TranscriptPane($$renderer, {});
			$$renderer.push(`<!----> <div class="divider svelte-1uha8ag"></div> `);
			TransformPane($$renderer, {});
			$$renderer.push(`<!---->`);
		}
		$$renderer.push(`<!--]--></section></div>`);
		if ($$store_subs) unsubscribe_stores($$store_subs);
	});
}
//#endregion
export { _page as default };
