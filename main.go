package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {
	benchmarks := []struct {
		name string
		fn   func(tmp string) BenchResult
	}{
		{"small_write", benchSmallWrite},
		{"small_read", benchSmallRead},
		{"mixed_rw", benchMixedRW},
		{"deep_tree", benchDeepTree},
		{"concurrent_write", benchConcurrentWrite},
		{"append_log", benchAppendLog},
		{"rename_batch", benchRenameBatch},
		{"stat_batch", benchStatBatch},
		{"symlink_batch", benchSymlinkBatch},
		{"delete_batch", benchDeleteBatch},
	}

	args := os.Args[1:]
	filter := ""
	fileCount := 5000
	fileSize := 256
	workers := 4
	depth := 6
	dir := ""
	printHelp := false

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--all" || args[i] == "all":
			filter = ""
		case args[i] == "--help" || args[i] == "-h":
			printHelp = true
		case strings.HasPrefix(args[i], "--"):
			flag, val := parseFlag(args, &i)
			switch flag {
			case "--files":
				fileCount = mustAtoi(val, 5000)
			case "--size":
				fileSize = mustAtoi(val, 256)
			case "--workers":
				workers = mustAtoi(val, 4)
			case "--depth":
				depth = mustAtoi(val, 6)
			case "--dir":
				dir = val
			default:
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", flag)
				os.Exit(1)
			}
		default:
			filter = args[i]
		}
	}

	if printHelp {
		printUsage(benchmarks)
		return
	}

	// Write params as marker files so benchmarks can read them
	writeMarkers := func(base string) {
		writeMarker(base, "files", fileCount)
		writeMarker(base, "size", fileSize)
		writeMarker(base, "workers", workers)
		writeMarker(base, "depth", depth)
	}

	if dir == "" {
		var err error
		dir, err = os.MkdirTemp("", "io-tester-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
			os.Exit(1)
		}
		defer os.RemoveAll(dir)
	} else {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create dir %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	writeMarkers(dir)

	fmt.Printf("╔═══ io-tester — dev I/O benchmarks ═══╗\n")
	fmt.Printf("  files: %d  size: %dB  workers: %d  depth: %d\n", fileCount, fileSize, workers, depth)
	fmt.Printf("  workdir: %s\n\n", dir)

	resultsCh := make(chan benchItem, len(benchmarks))
	var wg sync.WaitGroup

	for _, b := range benchmarks {
		if filter != "" && !strings.Contains(strings.ToLower(b.name), strings.ToLower(filter)) {
			continue
		}
		wg.Add(1)
		b := b
		go func() {
			defer wg.Done()
			tmp := filepath.Join(dir, b.name)
			os.MkdirAll(tmp, 0755)
			writeMarkers(tmp)
			r := b.fn(tmp)
			resultsCh <- benchItem{b.name, r}
		}()
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var results []benchItem
	for r := range resultsCh {
		results = append(results, r)
	}

	if len(results) == 0 {
		fmt.Println("No benchmarks matched.")
		return
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].result.OpsPerSec > results[j].result.OpsPerSec
	})

	fmt.Println("╔═══ Results (sorted by ops/sec) ═══╗")
	fmt.Printf("%-20s %10s %12s %12s %12s\n", "Benchmark", "Ops", "Ops/s", "MB/s", "Avg Lat")
	fmt.Println(strings.Repeat("─", 70))
	for _, r := range results {
		fmt.Printf("%-20s %10d %11.1f  %10.2f  %12s\n",
			r.name, r.result.Ops, r.result.OpsPerSec, r.result.MBPerSec, formatLatency(r.result.Latency))
	}
	fmt.Println(strings.Repeat("─", 70))
	fmt.Printf("Total: %d ops across %d benchmarks\n",
		sumOps(results), len(results))
}

type benchItem struct {
	name   string
	result BenchResult
}

type BenchResult struct {
	Ops       int
	Bytes     int64
	Duration  time.Duration
	OpsPerSec float64
	MBPerSec  float64
	Latency   time.Duration
}

func runBench(fn func() (int, int64, time.Duration)) BenchResult {
	ops, bytes, dur := fn()
	if ops == 0 {
		return BenchResult{}
	}
	r := BenchResult{Ops: ops, Bytes: bytes, Duration: dur}
	secs := dur.Seconds()
	r.OpsPerSec = float64(ops) / secs
	r.MBPerSec = (float64(bytes) / secs) / (1024 * 1024)
	r.Latency = dur / time.Duration(ops)
	return r
}

// ── Benchmarks ────────────────────────────────────────────────

func benchSmallWrite(dir string) BenchResult {
	return runBench(func() (int, int64, time.Duration) {
		fc := readMarkerInt(dir, "files", 5000)
		size := readMarkerInt(dir, "size", 256)
		data := make([]byte, size)
		rand.Read(data)

		start := time.Now()
		var total int64
		for i := 0; i < fc; i++ {
			path := filepath.Join(dir, fmt.Sprintf("f_%d.bin", i))
			if err := os.WriteFile(path, data, 0644); err != nil {
				break
			}
			total += int64(len(data))
		}
		syncDir(dir)
		return fc, total, time.Since(start)
	})
}

func benchSmallRead(dir string) BenchResult {
	return runBench(func() (int, int64, time.Duration) {
		entries, _ := os.ReadDir(dir)
		start := time.Now()
		var total int64
		count := 0
		buf := make([]byte, 64*1024)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			f, err := os.Open(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			n, _ := io.CopyBuffer(io.Discard, f, buf)
			f.Close()
			total += n
			count++
		}
		return count, total, time.Since(start)
	})
}

func benchMixedRW(dir string) BenchResult {
	return runBench(func() (int, int64, time.Duration) {
		fc := readMarkerInt(dir, "files", 5000)
		size := readMarkerInt(dir, "size", 256)
		data := make([]byte, size)
		rand.Read(data)

		start := time.Now()
		var total int64
		for i := 0; i < fc; i++ {
			path := filepath.Join(dir, fmt.Sprintf("rw_%d.bin", i))
			if err := os.WriteFile(path, data, 0644); err != nil {
				break
			}
			buf, err := os.ReadFile(path)
			if err != nil {
				break
			}
			total += int64(len(buf))
		}
		return fc, total, time.Since(start)
	})
}

func benchDeepTree(dir string) BenchResult {
	return runBench(func() (int, int64, time.Duration) {
		fc := readMarkerInt(dir, "files", 5000)
		depth := readMarkerInt(dir, "depth", 6)
		size := readMarkerInt(dir, "size", 256)
		perDir := fc / max(depth, 1)
		if perDir < 1 {
			perDir = 1
		}
		data := make([]byte, size)
		rand.Read(data)

		start := time.Now()
		var total int64
		count := 0

		var walk func(d string, level int)
		walk = func(d string, level int) {
			if level >= depth || count >= fc {
				return
			}
			for i := 0; i < perDir && count < fc; i++ {
				path := filepath.Join(d, fmt.Sprintf("l%d_f%d.bin", level, i))
				os.WriteFile(path, data, 0644)
				total += int64(len(data))
				count++
			}
			for i := 0; i < 2 && count < fc; i++ {
				sub := filepath.Join(d, fmt.Sprintf("d_%d", i))
				os.MkdirAll(sub, 0755)
				walk(sub, level+1)
			}
		}
		walk(dir, 0)
		return count, total, time.Since(start)
	})
}

func benchConcurrentWrite(dir string) BenchResult {
	return runBench(func() (int, int64, time.Duration) {
		fc := readMarkerInt(dir, "files", 5000)
		workers := readMarkerInt(dir, "workers", 4)
		size := readMarkerInt(dir, "size", 256)
		data := make([]byte, size)
		rand.Read(data)

		ch := make(chan int, fc)
		for i := 0; i < fc; i++ {
			ch <- i
		}
		close(ch)

		start := time.Now()
		var mu sync.Mutex
		var total int64
		var count int
		var wg sync.WaitGroup

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range ch {
					path := filepath.Join(dir, fmt.Sprintf("conc_%d.bin", i))
					if err := os.WriteFile(path, data, 0644); err != nil {
						return
					}
					mu.Lock()
					total += int64(len(data))
					count++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		return count, total, time.Since(start)
	})
}

func benchAppendLog(dir string) BenchResult {
	return runBench(func() (int, int64, time.Duration) {
		fc := readMarkerInt(dir, "files", 5000)
		f, _ := os.Create(filepath.Join(dir, "appends.log"))
		if f == nil {
			return 0, 0, 0
		}
		defer f.Close()

		start := time.Now()
		var total int64
		for i := 0; i < fc; i++ {
			line := fmt.Sprintf("[%d] op_%d ok\n", time.Now().UnixNano(), i)
			n, _ := f.WriteString(line)
			total += int64(n)
		}
		f.Sync()
		return fc, total, time.Since(start)
	})
}

func benchRenameBatch(dir string) BenchResult {
	fc := readMarkerInt(dir, "files", 5000)
	size := readMarkerInt(dir, "size", 256)
	srcDir := filepath.Join(dir, "rename_src")
	os.MkdirAll(srcDir, 0755)
	data := make([]byte, size)
	rand.Read(data)
	for i := 0; i < fc; i++ {
		os.WriteFile(filepath.Join(srcDir, fmt.Sprintf("old_%d.bin", i)), data, 0644)
	}

	return runBench(func() (int, int64, time.Duration) {
		dstDir := filepath.Join(dir, "rename_dst")
		os.MkdirAll(dstDir, 0755)
		start := time.Now()
		count := 0
		for i := 0; i < fc; i++ {
			src := filepath.Join(srcDir, fmt.Sprintf("old_%d.bin", i))
			dst := filepath.Join(dstDir, fmt.Sprintf("new_%d.bin", i))
			if os.Rename(src, dst) != nil {
				break
			}
			count++
		}
		syncDir(dstDir)
		os.RemoveAll(srcDir)
		os.RemoveAll(dstDir)
		return count, 0, time.Since(start)
	})
}

func benchStatBatch(dir string) BenchResult {
	fc := readMarkerInt(dir, "files", 5000)
	size := readMarkerInt(dir, "size", 256)
	data := make([]byte, size)
	rand.Read(data)
	for i := 0; i < fc; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("stat_%d.bin", i)), data, 0644)
	}

	return runBench(func() (int, int64, time.Duration) {
		start := time.Now()
		var total int64
		count := 0
		for i := 0; i < fc; i++ {
			fi, err := os.Stat(filepath.Join(dir, fmt.Sprintf("stat_%d.bin", i)))
			if err != nil {
				break
			}
			total += fi.Size()
			count++
		}
		return count, total, time.Since(start)
	})
}

func benchSymlinkBatch(dir string) BenchResult {
	fc := readMarkerInt(dir, "files", 5000)
	size := readMarkerInt(dir, "size", 256)
	tgtDir := filepath.Join(dir, "sym_targets")
	os.MkdirAll(tgtDir, 0755)
	data := make([]byte, size)
	rand.Read(data)
	for i := 0; i < fc; i++ {
		os.WriteFile(filepath.Join(tgtDir, fmt.Sprintf("tgt_%d.bin", i)), data, 0644)
	}

	return runBench(func() (int, int64, time.Duration) {
		lnkDir := filepath.Join(dir, "sym_links")
		os.MkdirAll(lnkDir, 0755)
		start := time.Now()
		count := 0
		for i := 0; i < fc; i++ {
			target := filepath.Join(tgtDir, fmt.Sprintf("tgt_%d.bin", i))
			link := filepath.Join(lnkDir, fmt.Sprintf("l_%d", i))
			if os.Symlink(target, link) != nil {
				break
			}
			count++
		}
		// Verify symlinks
		ok := 0
		for i := 0; i < count; i++ {
			if _, err := os.Readlink(filepath.Join(lnkDir, fmt.Sprintf("l_%d", i))); err == nil {
				ok++
			}
		}
		_ = ok
		return count, 0, time.Since(start)
	})
}

func benchDeleteBatch(dir string) BenchResult {
	fc := readMarkerInt(dir, "files", 5000)
	size := readMarkerInt(dir, "size", 256)
	delDir := filepath.Join(dir, "to_delete")
	os.MkdirAll(delDir, 0755)
	data := make([]byte, size)
	rand.Read(data)
	for i := 0; i < fc; i++ {
		os.WriteFile(filepath.Join(delDir, fmt.Sprintf("del_%d.bin", i)), data, 0644)
	}

	return runBench(func() (int, int64, time.Duration) {
		start := time.Now()
		count := 0
		for i := 0; i < fc; i++ {
			if os.Remove(filepath.Join(delDir, fmt.Sprintf("del_%d.bin", i))) != nil {
				break
			}
			count++
		}
		syncDir(dir)
		return count, 0, time.Since(start)
	})
}

// ── Helpers ───────────────────────────────────────────────────

func writeMarker(dir, name string, val int) {
	os.WriteFile(filepath.Join(dir, ".io-"+name), []byte(strconv.Itoa(val)), 0644)
}

func readMarkerInt(dir, name string, def int) int {
	data, err := os.ReadFile(filepath.Join(dir, ".io-"+name))
	if err != nil {
		return def
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return def
	}
	return v
}

func syncDir(dir string) {
	f, err := os.Open(dir)
	if err == nil {
		f.Sync()
		f.Close()
	}
}

func parseFlag(args []string, i *int) (string, string) {
	flag := args[*i]
	if strings.Contains(flag, "=") {
		parts := strings.SplitN(flag, "=", 2)
		return parts[0], parts[1]
	}
	if *i+1 < len(args) && !strings.HasPrefix(args[*i+1], "--") {
		*i++
		return flag, args[*i]
	}
	return flag, ""
}

func mustAtoi(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func formatLatency(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	case d >= time.Microsecond:
		return fmt.Sprintf("%.1fµs", float64(d.Nanoseconds())/1000)
	default:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
}

func sumOps(results []benchItem) int {
	total := 0
	for _, r := range results {
		total += r.result.Ops
	}
	return total
}

func printUsage(benchmarks []struct {
	name string
	fn   func(string) BenchResult
}) {
	fmt.Printf("io-tester — filesystem I/O benchmark for dev workloads\n\n")
	fmt.Printf("Usage:\n")
	fmt.Printf("  nix run . [benchmark] [flags]\n\n")
	fmt.Printf("Go-based benchmarks (omit to run all):\n")
	for _, b := range benchmarks {
		fmt.Printf("  %s\n", b.name)
	}
	fmt.Printf("\nAlso available via wrapper:\n")
	fmt.Printf("  nix run .#fs_mark    — run fs_mark (external tool)\n")
	fmt.Printf("  nix run .#fio        — run fio    (external tool)\n")
	fmt.Printf("  nix run .#bonnie     — run bonnie++ (external tool)\n")
	fmt.Printf("  nix run .#all-tools  — run all external tools\n\n")
	fmt.Printf("Flags:\n")
	fmt.Printf("  --files=N       Number of file operations (default: 5000)\n")
	fmt.Printf("  --size=N        File size in bytes (default: 256)\n")
	fmt.Printf("  --workers=N     Concurrent workers (default: 4)\n")
	fmt.Printf("  --depth=N       Directory tree depth (default: 6)\n")
	fmt.Printf("  --dir=PATH      Working directory (default: temp dir)\n")
	fmt.Printf("  --all           Run all Go benchmarks\n")
	fmt.Printf("  --help, -h      Show this help\n\n")
	fmt.Printf("Examples:\n")
	fmt.Printf("  nix run .                    # run all Go benchmarks\n")
	fmt.Printf("  nix run .#fs_mark            # run fs_mark\n")
	fmt.Printf("  nix run . -- --files=10000   # Go bench with 10k files\n")
	fmt.Printf("  nix run .#all-tools          # run all external tools\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
