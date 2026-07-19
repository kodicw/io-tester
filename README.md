# io-tester

Filesystem I/O benchmarks for **dev workloads** — specifically many small
file operations typical of compilers, package managers, git, and build tools.

## Quick start

```bash
# Run all Go-based benchmarks (10 patterns)
# Shows a system-info header (fastfetch) and styled output (charmbracelet/lipgloss)
nix run .

# Run a specific benchmark
nix run . -- small_write
nix run . -- concurrent_write
nix run . -- delete_batch

# Run with custom parameters
nix run . -- --files=20000 --size=64 --workers=8
nix run . -- deep_tree --files=5000 --depth=8
```

## External tools (also available)

Uses existing battle-tested tools from nixpkgs:

```bash
# fs_mark — synchronous small-file creation benchmark
nix run .#fs_mark

# fio — flexible I/O tester (random read/write mix)
nix run .#fio

# bonnie++ — filesystem benchmark
nix run .#bonnie

# Run all external tools
nix run .#all-tools
```

## Environment variables

| Variable        | Default  | Description              |
|-----------------|----------|--------------------------|
| `IO_FILES`      | 10000    | Number of file operations|
| `IO_SIZE`       | 256      | File size in bytes       |
| `IO_DEPTH`      | 4        | Directory tree depth     |
| `IO_WORKERS`    | 4        | Concurrent workers       |
| `IO_RUNTIME`    | 10       | fio runtime (seconds)    |

## Go benchmarks

| Benchmark         | What it tests                              |
|-------------------|--------------------------------------------|
| `small_write`     | Create N small files sequentially          |
| `small_read`      | Read back all files                        |
| `mixed_rw`        | Write + immediately read each file         |
| `deep_tree`       | Files spread across a deep directory tree  |
| `concurrent_write`| Parallel file creation with N workers      |
| `append_log`      | Append lines to a single log file          |
| `rename_batch`    | Batch rename many files                    |
| `stat_batch`      | Stat many files                            |
| `symlink_batch`   | Create + resolve many symlinks             |
| `delete_batch`    | Bulk delete files                          |

## System info & styling

Every run starts with a compact hardware/OS summary from
[`fastfetch`](https://github.com/fastfetch-cli/fastfetch):

- OS / Host / Kernel
- CPU / GPU
- Memory
- Disk usage

Output is styled with [`charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss).

## Why

Dev tools (npm install, git checkout, cargo build, tsc --watch)
create thousands of tiny files. Traditional I/O benchmarks measure
sequential throughput (big file R/W). io-tester focuses on the
**metadata-intensive, small-file patterns** that dominate dev workflows.
