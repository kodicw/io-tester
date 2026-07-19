# WSL2 vs Crostini: a dev-workload comparison

> A human-friendly explanation of the `io-tester` WSL vs Crostini results.

## The short version

This is a comparison of two Linux-on-VM setups.

**Crostini (ChromeOS)** runs Linux inside a VM called Termina, managed by `crosvm` on top of KVM. Inside the VM, LXD runs a Debian container (`penguin`). The container's storage is a `btrfs` disk image.

```
Your Linux app
  → LXC container (Debian "penguin")
    → Termina VM
      → crosvm (KVM)
        → ChromeOS Linux kernel
          → btrfs disk image
            → SSD
```

**WSL2 (Windows)** runs a full Linux kernel inside a utility VM managed by Hyper-V. Your Linux files live inside a `.vhdx` virtual disk file on Windows NTFS.

```
Your Linux app
  → Linux kernel
    → WSL2 utility VM
      → Hyper-V
        → Windows kernel
          → NTFS
            → .vhdx file
              → SSD
```

Both are virtualized. The hardware is not virtualized equally: the Windows machine in this comparison has a faster CPU, more RAM, and better graphics. The measurements below show what happens when both run the same dev-workload benchmarks.

---

## The hardware side-by-side

| Spec | Windows WSL machine | Chromebook (Crostini) | Paper winner |
|------|---------------------|-----------------------|--------------|
| Processor | AMD Ryzen 7 PRO 250<br>16 cores @ 3.29 GHz | Intel Core 3 N355<br>8 cores @ 1.88 GHz | **Windows** |
| RAM | 7.08 GB | 6.32 GB | **Windows** |
| Graphics | AMD Radeon 780M | Basic virtual GPU | **Windows** |
| Storage | SSD with NTFS | SSD with btrfs | Tie |

The Windows laptop has faster silicon on every axis: CPU, RAM, and GPU. The benchmark numbers below were collected on this hardware. Dev work is not one big task like rendering a video; it is thousands of tiny tasks — open a file, read it, write it, check its size, create a process, compile a file, link it, delete it.

---

## The architecture side-by-side

Both Crostini and WSL2 run Linux inside a virtual machine. The difference is *what kind* of VM and *what it sits on top of*.

### ChromeOS (Crostini)

```
Your Linux app
  → LXC container (Debian "penguin")
    → Termina VM (lightweight Linux guest)
      → crosvm (Rust VMM on KVM)
        → ChromeOS Linux kernel
          → btrfs disk image
            → SSD
```

`crosvm` is a virtual machine monitor written specifically to run Linux guests on Linux hosts. It uses **paravirtualized devices** (virtio), which means the guest knows it's in a VM and can take shortcuts. ChromeOS has optimized this path end-to-end. The Linux container's storage is a `btrfs` disk image, which is a Linux-native filesystem.

### Windows (WSL2)

```
Your Linux app
  → Linux kernel
    → WSL2 utility VM
      → Hyper-V hypervisor
        → Windows kernel
          → NTFS
            → .vhdx virtual disk file
              → SSD
```

WSL2 runs a full Linux kernel, but it lives inside a utility VM managed by Hyper-V. Your Linux files are stored inside a `.vhdx` virtual disk file, which lives on Windows NTFS. Every time Linux wants to touch a file, the request has to cross the VM boundary and then go through a Windows filesystem.

---

## What the numbers actually mean

`io-tester` measures "how many small things can the computer do per second?" These are the same small things your code editor, compiler, and package manager do all day.

| Benchmark | What it simulates | Windows WSL | Chromebook | Real difference |
|---|---|---:|---:|---|
| `concurrent_write` | Many files written at once | 10,409/sec | 72,260/sec | Chromebook **7× faster** |
| `small_write` | Saving lots of tiny source files | 4,649/sec | 56,056/sec | Chromebook **12× faster** |
| `process_spawn` | Running lots of short commands | 282/sec | 1,093/sec | Chromebook **4× faster** |
| `build_c` | Compiling a small C project | 24.7/sec | 31.5/sec | Chromebook **28% faster** |

The Windows machine has a CPU that is roughly **twice as fast** on paper. The benchmark numbers above show it is slower on every measured dev task.

---

## Where the time goes

### 1. The filesystem is double-wrapped

When you use WSL2, your Linux files live inside a `.vhdx` file. That's a virtual hard disk sitting inside your Windows disk. Every file operation has to go through:

```
Linux program
  → Linux kernel
    → virtual disk driver
      → .vhdx file
        → Windows NTFS
          → actual SSD
```

With Crostini, the path is more like:

```
Linux program
  → Linux kernel
    → btrfs disk image
      → SSD
```

Both use a VM. Both use a disk image. But Crostini's stack is Linux talking to Linux, while WSL2 is Linux talking to Windows. The Windows path has more translation steps and more flushing.

### 2. Every process creation pays a tax

A modern dev workflow spawns thousands of short processes: `git`, `npm`, `cargo`, `make`, `tsc`, `eslint`, `prettier`. On WSL2, each one has to cross the Hyper-V boundary. On Crostini, the process starts inside a container on a Linux guest, which is much closer to native speed.

### 3. Fsync hurts twice

When a program says "make sure this is really written to disk" (which compilers and databases do constantly), WSL2 has to flush both the virtual disk and the real Windows disk. Crostini only flushes once through its Linux-native stack.

### 4. Windows doesn't own the filesystem

Windows and Linux have different ideas about file permissions, case sensitivity, and special files. WSL2 has to translate these ideas back and forth. Crostini doesn't — the entire stack from the container up to the disk image is Linux.

---

## Why faster hardware won't fix it

The Windows machine already has better hardware. Adding more of it does not remove the places where time is lost.

### A faster CPU does not remove translation

Most of the slowdown is not from the CPU working slowly. It is from the CPU *waiting*: waiting for the VM boundary, waiting for NTFS, waiting for the `.vhdx` driver, waiting for Windows Defender to scan a file. A faster CPU can execute instructions quicker, but it cannot make a round-trip through three software layers happen in zero time.

### More cores do not help

Many of the operations in the benchmark table are serialized by the filesystem and the VM. A file create on NTFS has to be ordered. A `fsync` has to wait for the disk. A process creation has to go through the Hyper-V path one at a time. Throwing 16 cores or 32 cores at the problem does not parallelize the bottleneck.

### A faster SSD only helps part of the problem

A faster NVMe drive can push more bytes per second. But the table above is not measuring bulk throughput. It is measuring thousands of tiny operations. Each operation has a fixed cost that is independent of SSD speed: a syscall, a context switch, a VM exit, an antivirus scan, a metadata update. A faster drive shrinks the time spent moving bytes, but the per-operation overhead remains.

### More RAM does not help

The benchmark runs fit easily in memory. The issue is not that the machine is running out of RAM. The issue is that every file operation has to be acknowledged, flushed, translated, and checked by multiple layers. More RAM does not remove those layers.

### The cost vs performance analogy

Imagine two cars in a race across town.

One is a modest hatchback. Small engine, basic interior, nothing flashy. But the road ahead is clear, every light is green, and the highway on-ramp is right there.

The other is a sports car with twice the horsepower, twice as many cylinders, a dedicated racing computer, and a bigger fuel tank. On paper it is superior in every way. But it is parked inside a gated compound. Every time the driver wants to move:

1. Radio the gatehouse for permission to leave
2. Wait for a security escort to unlock the inner gate
3. Follow the escort through a one-lane service road
4. Stop at the outer checkpoint for an inspection
5. Finally reach the public road

The sports car has **2× the clock speed**, **2× the cores**, a **dedicated GPU**, and **more RAM**. Those are real advantages — for a drag strip. But this is not a drag strip. This is city driving: thousands of short trips, constant stopping and starting, quick errands one after another. The hatchback finishes every errand faster because nothing stands between the engine and the road.

The benchmark results tell the same story. The Windows machine has roughly twice the CPU, twice the cores, and a real GPU. The Chromebook still writes files **7–12× faster**, spawns processes **4× faster**, and compiles **28% faster**. The specs lost to the architecture.

---

## What the slowdown can cost in time

A 2022 developer survey found that the average build takes about 20 minutes, and developers spend roughly **57 minutes per day** just waiting for builds to finish. Another 2025 survey reported that engineering teams spend a combined average of **32 hours per day** running builds across all their members.

Those are industry averages, not necessarily the numbers in the table above. But they give a sense of scale. Dev work is mostly waiting: waiting for installs, waiting for compiles, waiting for tests, waiting for git. Every second of overhead on those operations multiplies across a day.

### A back-of-the-envelope example

The `io-tester` numbers above show a 28% slowdown in the `build_c` benchmark on WSL2 vs Crostini. If a developer's incremental build takes 2 minutes on Crostini, a 28% slowdown means roughly **34 seconds extra** per build.

| Builds per day | Extra time per day | Over a 250-day year |
|---|---:|---:|
| 10 | ~6 minutes | ~25 hours |
| 20 | ~11 minutes | ~47 hours |
| 40 | ~23 minutes | ~95 hours |

That is only the compile step. If you add in `npm install` (which creates tens of thousands of small files), `git status`, test runners, and other file-heavy tools, the extra overhead compounds. The `small_write` benchmark above is 12× slower on WSL2, and `process_spawn` is 4× slower. Those are the exact patterns package managers and build systems repeat thousands of times.

Plugging the numbers in another way: a survey found that 98% of developers admit they waste time waiting for builds. If a Linux-native environment shaves even 10–15 minutes of wait time per day, that adds up to roughly **one to two weeks of engineering time per year** for one person. Across a team, it becomes months or years of lost time.

These are rough estimates. The actual number depends on the project, the tools, the codebase size, and how often the developer builds, tests, and installs dependencies. The reader can do their own math.

---

The reader can decide which matters more: the paper specs on the box, or the number of roadblocks between the code and the hardware.
