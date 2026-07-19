# io-tester

Dev-workload performance benchmark. Measures filesystem I/O, compilation-like
workloads, and process overhead — the patterns that dominate actual dev work
(compilers, package managers, git, build systems).

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

## Compiler used for build benchmarks

The `build_c` and `build_incremental` benchmarks pick a compiler in this order:

1. `CC` environment variable
2. `cc`, `gcc`, `clang`, `tcc` in your `PATH`

The Nix wrapper provides `tcc` by default so the benchmarks run out of the box.
Use your own compiler for more realistic numbers:

```bash
CC=gcc nix run . -- build_c --files=200
CC=clang nix run . -- build_incremental --files=500
```

## Presets

Presets are named bundles of defaults. Individual flags still override them.

| Preset | Files | Size | Workers | Depth | Dir |
|----------|------|------|---------|-------|-----|
| `quick` | 100 | 64B | 1 | 4 | temp |
| `concurrent` | 10,000 | 256B | 8 | 6 | `test` |
| `dev` | 5,000 | 256B | 4 | 6 | temp |
| `heavy` | 50,000 | 1KB | 16 | 8 | `test` |

```bash
# Fast smoke test
nix run . -- --preset=quick

# Heavy concurrent write test
nix run . -- concurrent_write --preset=concurrent

# Heavy build on a fast SSD
nix run . -- build_c --preset=heavy --dir=/fast-ssd/build
```

## Save & compare runs

Save a result to JSON:

```bash
nix run github:kodicw/io-tester -- --preset=quick --save=wsl.json
```

Then compare another run against it:

```bash
nix run github:kodicw/io-tester -- --preset=quick --save=crostini.json --compare=wsl.json
```

Or compare two saved files directly:

```bash
nix run github:kodicw/io-tester -- compare wsl.json crostini.json
```

The comparison shows each benchmark's baseline ops/sec, current ops/sec, and the percentage delta.

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
| `build_c`         | Compile a simulated C project              |
| `build_incremental`| Touch one file and rebuild                |
| `process_spawn`   | Spawn many short processes                 |

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
