---
name: idea
description: Capture, find and edit the user's ideas with the `idea` CLI. Use when the user asks to capture something ("capture this", "save this as an idea"), to fold material into an existing idea ("add this to my idea about…"), or asks about their ideas ("what ideas do I have on…").
---

# idea

The user's ideas live on one local server, reached only through the `idea` CLI. For any verb's syntax, flags and exit codes, run `idea help <verb>`. There is no history: every write overwrites, so anything you drop is gone for good.

Touch ideas only when the user refers to them. Reading is then unrestricted; otherwise leave the ideas alone and never search them unprompted.

## Capture

Capture only when the user asks for it; the request is the trigger. Never capture, or offer to, on your own initiative.

1. **Search first**, before every `idea add`: a text Filter on the idea's key terms, e.g. `idea ls borrow checker tag:rust`.
   - A clear match: ask whether to fold the discussion into that idea (then it is an edit, see below) or capture a new one. This is the only question in the capture path.
   - No match: capture without asking.
2. **Distill the body** into a self-contained markdown document:
   - The first paragraph is the gist, then the idea and the reasoning that survived.
   - Open questions go under a closing heading (`## Open questions`).
   - It stands alone: no transcript, no link to the conversation, no "as discussed".
   - The title lives apart from the body, so the body carries no H1 repeating it.
3. **Name it.** You pick the title. Run `idea tags` and reuse the existing tags; mint a new tag only when none fits, and say so in the report.
4. **Leave the rest at its defaults.** Status stays `raw` unless the user names one. Set an attribute only when the user names it or a key already listed by `idea attrs` plainly applies; a new key comes only from the user. Record no provenance (source, session, "captured by").
5. **Add it directly.** Review is the approval step, so there is no draft to approve.
6. **Report** one line per idea: id, title, tags, e.g. `42 Borrow checker for config files [rust, dsl]`.

**One idea per distinct idea.** A discussion with two separable ideas becomes two captures, each through the steps above, and the report names both ids. Cross-reference in plain prose: "see idea 42".

## Editing

1. **Find the idea:** `idea ls <filter> --json`. With one clear match, proceed and name the idea you touch. With several, list them and ask which one. Never guess.
2. **Change it:**
   - A local change: `idea replace`.
   - A restructure: `idea show --json <id>`, rewrite the body, then `idea write <id> --version <n>` with the `version` from that show.
3. **Integrate** new material where it belongs. The document stays a current statement of the idea, with no dated log or "Update:" sections.
4. **No silent loss.** Keep existing content unless the user asks you to remove or contradict it. When you remove something, quote what was removed in your report.
5. **Title and status** change only when the user asks.
6. **Exit 4** means the idea changed under you: re-read it, reapply your change to the fresh body, and retry.

## Boundaries

- **Every write names a Version.** `--force` and `idea rm` are off-limits. Asked to delete, hand the user the command to run themselves, `! idea rm <id>`, and if they only want the idea out of the open set, offer `idea status <id> dropped`.
- **Several ideas at once:** list the affected ideas, wait for the go-ahead, then work one idea at a time.
- **Exit 7**, the daemon is unreachable: report the CLI's message and print the finished document into the conversation, so nothing is lost. The daemon is the user's to run: never start it, and never write the document to a fallback file.
