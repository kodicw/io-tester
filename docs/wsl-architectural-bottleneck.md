# WSL2 is an architectural bottleneck, not a workflow preference

## The problem

Linux development on Windows via WSL2 is slower than other Linux-in-a-VM setups — not because it uses a VM, but because of **what the VM sits on top of**.

Both WSL2 and ChromeOS Crostini run Linux inside a virtual machine. That is not the issue. The issue is that WSL2's VM is hosted by a **Windows kernel running NTFS**, while Crostini's VM is hosted by a **Linux kernel running btrfs**. Every file operation in WSL2 has to cross an OS boundary — Linux talking to Windows talking to NTFS — and that translation is where the time goes.

```
WSL2:     Linux app → Linux kernel → Hyper-V → Windows kernel → NTFS → .vhdx → SSD
Crostini: Linux app → Linux kernel → crosvm  → Linux kernel   → btrfs image  → SSD
```

Same concept. Same number of layers. But one path is Linux talking to Linux, and the other is Linux talking to Windows. The Windows path has to translate filesystem semantics (permissions, case sensitivity, special files), flush writes through both a virtual disk and a real NTFS disk, and pass every file through Windows Defender. None of that exists in the Linux-to-Linux path.

This is not a matter of preference, configuration, or tooling choice. It is a fixed cost baked into the architecture. No amount of hardware upgrades, settings changes, or workflow adjustments removes the translation layer.

## What the data shows

Benchmarks using [io-tester](https://github.com/kodicw/io-tester) — which measures the small, repeated I/O operations that dev tools perform thousands of times per day — show that a lower-spec Linux machine consistently outperforms a higher-spec Windows+WSL2 machine:

| Operation | Impact |
|---|---|
| File metadata checks | **2.5× slower** on WSL2 |
| Log appends | **5.7× slower** on WSL2 |
| Small file writes | **60% slower** on WSL2 |
| Process spawning (git, npm, make, etc.) | **3.9× slower** on WSL2 |
| C compilation | **38% slower** on WSL2 |
| Incremental rebuilds | **85% slower** on WSL2 |

The Windows machine in this comparison has **2× the CPU clock speed**, **2× the cores**, a **dedicated GPU**, and **more RAM** than the Linux machine. It still loses on the operations that define a developer's day.

## Why hardware won't fix it

The bottleneck is not CPU speed, RAM, or disk throughput. It is per-operation overhead: filesystem translation between Linux and Windows, mandatory antivirus scanning, and double-flushing writes through both a virtual and physical disk. A faster CPU executes instructions quicker but cannot eliminate the translation. More cores cannot parallelize a serialized filesystem path. A faster SSD cannot skip the cross-OS conversion sitting above it.

## The bottom line

This is not "I prefer Linux." This is not "VMs are slow." ChromeOS uses a VM too, and it is faster. The problem is specifically that WSL2 forces every I/O operation through a Windows-to-Linux translation layer. That translation adds a measurable, consistent tax to every file read, write, stat, and process spawn a developer performs. It compounds across thousands of operations per hour into minutes of lost time per day and weeks per year. It is a structural limitation of running Linux on top of a Windows host, and it cannot be configured away.
