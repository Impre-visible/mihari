<script lang="ts">
  import type { Component } from "svelte";
  import {
    ArrowBendDownRightIcon,
    ChatCircleTextIcon,
    CheckCircleIcon,
    ChatsIcon,
    FileTextIcon,
    FlagCheckeredIcon,
    FolderOpenIcon,
    GitCommitIcon,
    GitDiffIcon,
    GitPullRequestIcon,
    ListBulletsIcon,
    MagnifyingGlassIcon,
    NotebookIcon,
    PencilSimpleIcon,
    TerminalWindowIcon,
    WrenchIcon,
    XCircleIcon,
  } from "phosphor-svelte";
  import { Spinner } from "$lib/ui";
  import Diff from "./Diff.svelte";
  import Fold from "./Fold.svelte";
  import { exitCode, failed, str, withoutExitCode, type ToolCall } from "./transcript";

  // live: the job still runs, so a call without a result is under way.
  type Props = { call: ToolCall; live: boolean };
  let { call, live }: Props = $props();

  type View = { icon: Component; title: string; detail?: string; open: boolean };

  // What changes something stays open; what only reads starts folded.
  const VIEWS: Record<string, (a: Record<string, unknown>) => View> = {
    read_file: (a) => ({ icon: FileTextIcon, title: "Lecture", detail: str(a.path) + range(a), open: false }),
    write_file: (a) => ({ icon: PencilSimpleIcon, title: "Écriture", detail: str(a.path), open: true }),
    list_files: (a) => ({ icon: FolderOpenIcon, title: "Fichiers", detail: str(a.path) || ".", open: false }),
    search: (a) => ({ icon: MagnifyingGlassIcon, title: "Recherche", detail: str(a.pattern), open: false }),
    run_command: (a) => ({ icon: TerminalWindowIcon, title: "Commande", detail: str(a.command), open: true }),
    get_merge_request: () => ({ icon: GitPullRequestIcon, title: "Lecture de la MR", open: false }),
    get_issue: () => ({ icon: ListBulletsIcon, title: "Lecture de l'issue", open: false }),
    get_diff: (a) => ({ icon: GitDiffIcon, title: "Diff", detail: str(a.path) || "tous les fichiers", open: false }),
    get_new_changes: () => ({ icon: GitDiffIcon, title: "Nouveaux changements", open: false }),
    list_commits: () => ({ icon: GitCommitIcon, title: "Commits", open: false }),
    get_pipeline: () => ({ icon: WrenchIcon, title: "Pipeline", open: false }),
    read_thread: () => ({ icon: ChatsIcon, title: "Lecture du fil", open: false }),
    read_comments: () => ({ icon: ChatsIcon, title: "Lecture des commentaires", open: false }),
    comment_line: (a) => ({ icon: ChatCircleTextIcon, title: "Commentaire", detail: `${str(a.path)}:${str(a.line)}`, open: true }),
    suggest_change: (a) => ({ icon: ChatCircleTextIcon, title: "Suggestion", detail: str(a.path) + range(a), open: true }),
    reply_to_thread: () => ({ icon: ArrowBendDownRightIcon, title: "Réponse dans un fil", open: true }),
    reply: () => ({ icon: ArrowBendDownRightIcon, title: "Réponse", open: true }),
    push: () => ({ icon: GitCommitIcon, title: "Push", open: true }),
    open_merge_request: (a) => ({ icon: GitPullRequestIcon, title: "MR ouverte", detail: str(a.title), open: true }),
    remember: () => ({ icon: NotebookIcon, title: "Note pour le projet", open: true }),
    finish: () => ({ icon: FlagCheckeredIcon, title: "Fin", open: true }),
  };

  function range(a: Record<string, unknown>) {
    const start = a.start_line, end = a.end_line;
    if (!start && !end) return "";
    return `:${str(start) || 1}${end && end !== start ? `-${str(end)}` : ""}`;
  }

  let view = $derived(
    VIEWS[call.name]?.(call.args) ?? { icon: WrenchIcon, title: call.name, open: false },
  );
  let output = $derived(call.result?.text ?? "");
  let isError = $derived(failed(call));
  let code = $derived(exitCode(output));
  // Shells say how a command went; a non-zero exit is a failure worth seeing.
  let bad = $derived(isError || (code !== null && code !== 0));
  let pending = $derived(call.result === null);

  function numbered(text: string) {
    const lines = text.replace(/\n$/, "").split("\n");
    const width = String(lines.length).length;
    return lines.map((l, i) => `${String(i + 1).padStart(width)}  ${l}`).join("\n");
  }
</script>

{#snippet comment(body: string)}
  <div class="rounded-md border border-separator bg-window px-3 py-2">
    <p class="p-subhead-2 whitespace-pre-wrap select-text">{body}</p>
  </div>
{/snippet}

<details class="group rounded-lg border border-separator" open={view.open || bad}>
  <summary class="flex cursor-default list-none items-center gap-2 px-3 py-2 [&::-webkit-details-marker]:hidden">
    <view.icon size={15} class="shrink-0 text-secondary-label" aria-hidden="true" />
    <span class="p-subhead-2 shrink-0 font-semibold">{view.title}</span>
    {#if view.detail}
      <span class="p-subhead-2 min-w-0 truncate font-mono text-secondary-label">{view.detail}</span>
    {/if}
    <span class="ms-auto flex shrink-0 items-center gap-1.5">
      {#if pending && live}
        <Spinner small label="En cours" />
      {:else if pending}
        <span class="p-subhead-2 text-tertiary-label">sans résultat</span>
      {:else if bad}
        <XCircleIcon size={15} weight="fill" class="text-(--p-sys-red)" aria-label="Échec" />
      {:else}
        <CheckCircleIcon size={15} weight="fill" class="text-(--p-sys-green)" aria-label="Réussi" />
      {/if}
    </span>
  </summary>

  <div class="flex flex-col gap-2 border-t border-separator px-3 py-2">
    {#if call.name === "write_file"}
      <Fold text={numbered(str(call.args.content))} class="text-label" />
      {#if isError}<Fold text={output} class="text-(--p-sys-red)" />{/if}
    {:else if call.name === "run_command"}
      <div class="rounded-md bg-black/85 px-3 py-2 text-white/90">
        <pre class="p-subhead-2 font-mono whitespace-pre-wrap select-text"><span class="text-(--p-sys-green)">$</span> {str(
            call.args.command,
          )}</pre>
        {#if !pending}
          <Fold text={withoutExitCode(output)} class="mt-1 text-white/75" />
          {#if code !== null}
            <p class={["p-subhead-2 mt-1 font-mono", code === 0 ? "text-white/50" : "text-(--p-sys-red)"]}>
              code de sortie {code}
            </p>
          {/if}
        {/if}
      </div>
    {:else if call.name === "get_diff" || call.name === "get_new_changes"}
      {#if isError}<Fold text={output} class="text-(--p-sys-red)" />{:else}<Diff text={output} />{/if}
    {:else if call.name === "comment_line" || call.name === "reply" || call.name === "reply_to_thread"}
      {@render comment(str(call.args.body))}
      {#if output}<p class={["p-subhead-2", isError ? "text-(--p-sys-red)" : "text-secondary-label"]}>{output}</p>{/if}
    {:else if call.name === "suggest_change"}
      {#if str(call.args.comment)}{@render comment(str(call.args.comment))}{/if}
      <Diff text={str(call.args.code).split("\n").map((l) => "+" + l).join("\n")} />
      {#if output}<p class={["p-subhead-2", isError ? "text-(--p-sys-red)" : "text-secondary-label"]}>{output}</p>{/if}
    {:else if call.name === "push"}
      <pre class="p-subhead-2 font-mono whitespace-pre-wrap select-text">{str(call.args.message)}</pre>
      {#if output}<Fold text={output} class={isError ? "text-(--p-sys-red)" : "text-secondary-label"} />{/if}
    {:else if call.name === "open_merge_request"}
      {@render comment(str(call.args.description))}
      {#if output}<Fold text={output} class={isError ? "text-(--p-sys-red)" : "text-secondary-label"} />{/if}
    {:else if call.name === "remember"}
      <p class="p-subhead-2 select-text">{str(call.args.note)}</p>
    {:else if call.name === "finish"}
      <p class={["p-subhead-2 select-text", call.args.success === false && "text-(--p-sys-red)"]}>
        {call.args.success === false ? "Abandon : " : ""}{str(call.args.summary)}
      </p>
    {:else}
      {#if !(call.name in VIEWS) && call.raw && call.raw !== "{}"}
        <Fold text={JSON.stringify(call.args, null, 2)} class="text-secondary-label" />
      {/if}
      {#if !pending}
        <Fold text={output} class={isError ? "text-(--p-sys-red)" : "text-label"} />
      {/if}
    {/if}
  </div>
</details>
