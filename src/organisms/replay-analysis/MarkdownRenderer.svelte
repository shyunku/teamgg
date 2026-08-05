<script>
  export let source = "";

  const escapeHtml = (value) =>
    String(value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#039;");

  const inline = (value) =>
    escapeHtml(value)
      .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
      .replace(/\x60([^\x60]+)\x60/g, "<code>$1</code>")
      .replace(/\*(.+?)\*/g, "<em>$1</em>");

  const render = (markdown) => {
    const output = [];
    let listType = null;
    const closeList = () => {
      if (!listType) return;
      output.push(`</${listType}>`);
      listType = null;
    };

    for (const rawLine of String(markdown ?? "").replace(/\r/g, "").split("\n")) {
      const line = rawLine.trimEnd();
      if (!line.trim()) {
        closeList();
        continue;
      }
      const heading = line.match(/^(#{1,4})\s+(.+)$/);
      if (heading) {
        closeList();
        const level = heading[1].length;
        output.push(`<h${level}>${inline(heading[2])}</h${level}>`);
        continue;
      }
      const unordered = line.match(/^\s*[-*]\s+(.+)$/);
      if (unordered) {
        if (listType !== "ul") { closeList(); listType = "ul"; output.push("<ul>"); }
        output.push(`<li>${inline(unordered[1])}</li>`);
        continue;
      }
      const ordered = line.match(/^\s*\d+[.)]\s+(.+)$/);
      if (ordered) {
        if (listType !== "ol") { closeList(); listType = "ol"; output.push("<ol>"); }
        output.push(`<li>${inline(ordered[1])}</li>`);
        continue;
      }
      const quote = line.match(/^>\s?(.*)$/);
      if (quote) {
        closeList();
        output.push(`<blockquote>${inline(quote[1])}</blockquote>`);
        continue;
      }
      closeList();
      output.push(`<p>${inline(line)}</p>`);
    }
    closeList();
    return output.join("\n");
  };

  $: html = render(source);
</script>

<article class="markdown-body">{@html html}</article>

<style lang="scss">
  @import "../../styles/variables.scss";

  .markdown-body {
    color: $color-text-secondary;
    font-size: 14px;
    line-height: 1.75;
    white-space: normal;

    :global(h1), :global(h2), :global(h3), :global(h4) {
      color: $color-text-primary;
      white-space: normal;
    }
    :global(h1) { margin: 0 0 22px; font-size: 27px; }
    :global(h2) { margin: 36px 0 14px; padding-bottom: 9px; border-bottom: 1px solid $color-border; font-size: 20px; }
    :global(h3) { margin: 26px 0 10px; color: $color-highlight-light; font-size: 16px; }
    :global(h4) { margin: 20px 0 8px; font-size: 14px; }
    :global(p) { margin: 8px 0; white-space: normal; }
    :global(ul), :global(ol) { margin: 9px 0 16px; padding-left: 24px; white-space: normal; }
    :global(li) { margin: 6px 0; white-space: normal; }
    :global(strong) { color: $color-text-primary; font-weight: 700; }
    :global(code) { padding: 2px 5px; border-radius: 4px; background: $color-accent-soft; color: $color-accent-light; font-size: 0.9em; }
    :global(blockquote) { margin: 14px 0; padding: 10px 14px; border-left: 3px solid $color-highlight; background: rgba(79, 140, 255, 0.06); color: $color-text-secondary; white-space: normal; }
  }
</style>
