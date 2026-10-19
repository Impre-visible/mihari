<script lang="ts">
  // Text the agent posts on GitLab, as a comment would show.
  type Props = { body: string };
  let { body }: Props = $props();
</script>

<div class="rounded-md border border-separator bg-window px-3 py-2">
  <p class="p-subhead-2 whitespace-pre-wrap select-text">{body}</p>
</div>
