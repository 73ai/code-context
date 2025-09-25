# code-context

Experiments around giving coding agents semantic code understanding and cross-referencing capabilities.

## codegrep 
We're building codegrep - a high-performance code search and intelligence tool that gives LLMs and developers the same semantic understanding of codebases that they
  rely on for safe, effective code editing.

  What It Does

  codegrep is a drop-in replacement for ripgrep that adds powerful semantic capabilities:

  - Full ripgrep compatibility - All existing ripgrep workflows work unchanged
  - Symbol lookup - Find function/variable definitions across files (--symbols)
  - Reference tracking - Locate all usages of symbols (--refs)
  - Call graph analysis - Understand function relationships and dependencies
  - Cross-language intelligence - Semantic search across multiple programming languages

  Why It Matters

  Current tools force a painful trade-off:
  - Fast tools (ripgrep, grep) lack semantic understanding
  - Smart tools (LSP servers) are slow and resource-heavy
  - LLMs are reduced to guessing without proper code context

  codegrep bridges this gap - providing semantic accuracy with grep-level performance.