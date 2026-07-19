# Why a low-end Chromebook beats a powerful Windows laptop for dev work

> A human-friendly explanation of the `io-tester` WSL vs Crostini results.

## The short version

A **weaker Chromebook** can feel faster for coding than a **more powerful Windows laptop** because ChromeOS is designed *around* Linux, while Windows *hosts* Linux as an afterthought.

Both systems use a virtual machine to run Linux. But ChromeOS's VM is a lightweight, Linux-on-Linux design (`crosvm` on KVM), and its storage is a Linux-native filesystem (`btrfs`). Windows's WSL2 VM is a heavier Windows-on-Linux design, and your Linux files live inside a `.vhdx` file sitting on top of Windows NTFS.

So the Chromebook takes the direct route, while Windows makes Linux take the scenic route — through a translation layer on every single file operation.

It's not about raw horsepower. It's about how many roadblocks the operating system puts between your code and the hardware.

---

## The hardware side-by-side

| Spec | Windows WSL machine | Chromebook (Crostini) | Paper winner |
|------|---------------------|-----------------------|--------------|
| Processor | AMD Ryzen 7 PRO 250<br>16 cores @ 3.29 GHz | Intel Core 3 N355<br>8 cores @ 1.88 GHz | **Windows** |
| RAM | 7.08 GB | 6.32 GB | **Windows** |
| Graphics | AMD Radeon 780M | Basic virtual GPU | **Windows** |
| Storage | SSD with NTFS | SSD with btrfs | Tie |

On paper, the Windows laptop should crush the Chromebook. It has a faster CPU, more RAM, and better graphics. But dev work isn't a single big task like rendering a video. Dev work is **thousands of tiny tasks** — open a file, read it, write it, check its size, create a process, compile a file, link it, delete it.

For that kind of work, the Chromebook wins because the path is shorter and purpose-built for Linux.

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

## The analogy: two workshops

### Crostini (Chromebook) = a prefab workshop in the driveway

The workshop is a separate building, but it was built by the same company that built your house. The doors are the right size, the power outlets match, and the tools connect directly. You still have to walk across the driveway, but the path is short and smooth.

### WSL2 (Windows) = a workshop inside a shipping container on a flatbed truck

You have a nicer workshop inside a shipping container. But the container is sitting on a flatbed truck that's driving on Windows roads. Every time you want a tool, you have to:

1. Walk to the container door
2. The truck driver radios the warehouse
3. The warehouse finds the tool and loads it onto the truck
4. You finally get the tool

Both setups have a separate workshop. The Crostini one is just *designed* to be a workshop from the start.

---

## What the numbers actually mean

`io-tester` measures "how many small things can the computer do per second?" These are the same small things your code editor, compiler, and package manager do all day.

| Benchmark | What it simulates | Windows WSL | Chromebook | Real difference |
|---|---|---:|---:|---|
| `concurrent_write` | Many files written at once | 10,409/sec | 72,260/sec | Chromebook **7× faster** |
| `small_write` | Saving lots of tiny source files | 4,649/sec | 56,056/sec | Chromebook **12× faster** |
| `process_spawn` | Running lots of short commands | 282/sec | 1,093/sec | Chromebook **4× faster** |
| `build_c` | Compiling a small C project | 24.7/sec | 31.5/sec | Chromebook **28% faster** |

The Windows machine has a CPU that is roughly **twice as fast** on paper, yet it loses on every single dev task. The only thing the Windows machine would definitely win at is something like gaming or video rendering — big, single jobs that don't need to cross the WSL2 boundary thousands of times.

---

## Why Windows loses

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

## Why this is sad for Windows as a dev OS

The problem is not that Windows can't run Linux. WSL2 is genuinely impressive engineering. The problem is that Windows is still a **Windows-first** operating system. Linux is a guest, not a native citizen.

So every time you:
- Save a file in VS Code
- Run `npm install`
- Compile a project
- Run a test
- Stage a file in git

...Windows adds a little tax. One tax is fine. A thousand taxes per second makes the whole machine feel sluggish.

On a Chromebook, Linux is a first-class resident. ChromeOS was built to run a Linux container (Crostini) from the start. The VM, the storage layer, and the filesystem were all designed with Linux in mind. That's why a weaker machine can feel snappier for the kind of work developers actually do.

---

A Chromebook, despite the weaker specs, was built without the toll booths.
